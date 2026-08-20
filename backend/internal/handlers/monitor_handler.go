package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
	"github.com/thureos/compliance/internal/services"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type MonitorHandler struct {
	monitorRepo      *repository.MonitorRepository
	ingestionService *services.IngestionService
	jobQueue         *services.JobQueue
	ruleEngine       *services.RuleEngine
}

func NewMonitorHandler(
	monitorRepo *repository.MonitorRepository,
	ingestionService *services.IngestionService,
	jobQueue *services.JobQueue,
	ruleEngine *services.RuleEngine,
) *MonitorHandler {
	return &MonitorHandler{
		monitorRepo:      monitorRepo,
		ingestionService: ingestionService,
		jobQueue:         jobQueue,
		ruleEngine:       ruleEngine,
	}
}

func (h *MonitorHandler) Create(c *fiber.Ctx) error {
	var req models.CreateMonitorRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	userID, _ := primitive.ObjectIDFromHex(c.Locals("userId").(string))

	monitor := &models.Monitor{
		Name:         req.Name,
		Description:  req.Description,
		SourceType:   req.SourceType,
		CollectionID: primitive.NewObjectID().Hex(),
		APIEndpoint:  req.APIEndpoint,
		Schedule:     req.Schedule,
		OwnerID:      userID,
	}

	if err := h.monitorRepo.Create(c.Context(), monitor); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "creating monitor: " + err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(monitor)
}

func (h *MonitorHandler) List(c *fiber.Ctx) error {
	monitors, err := h.monitorRepo.FindAll(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if monitors == nil {
		monitors = []models.Monitor{}
	}
	return c.JSON(monitors)
}

func (h *MonitorHandler) Get(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}

	monitor, err := h.monitorRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
	}

	return c.JSON(monitor)
}

func (h *MonitorHandler) Update(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}

	var body map[string]interface{}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	// Only allow updating name and description
	update := bson.M{}
	if v, ok := body["name"]; ok {
		update["name"] = v
	}
	if v, ok := body["description"]; ok {
		update["description"] = v
	}

	if len(update) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no valid fields to update"})
	}

	if err := h.monitorRepo.Update(c.Context(), id, update); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "monitor updated"})
}

func (h *MonitorHandler) Delete(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}

	if err := h.monitorRepo.Delete(c.Context(), id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "monitor deleted"})
}

func (h *MonitorHandler) UploadData(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}

	monitor, err := h.monitorRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
	}

	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "file is required"})
	}

	f, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "opening file"})
	}
	defer f.Close()

	var count int
	switch monitor.SourceType {
	case models.SourceCSV:
		count, err = h.ingestionService.IngestCSV(c.Context(), monitor, f)
	case models.SourceJSON:
		count, err = h.ingestionService.IngestJSON(c.Context(), monitor, f)
	case models.SourceExcel:
		count, err = h.ingestionService.IngestExcel(c.Context(), monitor, f)
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unsupported source type"})
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Enqueue rule evaluation (async via worker)
	queued := false
	if h.jobQueue != nil {
		err = h.jobQueue.Enqueue(c.Context(), services.EvalJob{MonitorID: id.Hex()})
		queued = err == nil
	}

	return c.JSON(fiber.Map{
		"recordsIngested":  count,
		"schema":           monitor.Schema,
		"evaluationQueued": queued,
	})
}

func (h *MonitorHandler) IngestionHistory(c *fiber.Ctx) error {
	entries, err := h.monitorRepo.GetIngestionHistory(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(entries)
}

func (h *MonitorHandler) Evaluate(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}

	monitor, err := h.monitorRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
	}

	if monitor.RecordCount == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "monitor has no data to evaluate"})
	}

	alerts, err := h.ruleEngine.EvaluateRules(c.Context(), monitor)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "rule evaluation failed: " + err.Error()})
	}

	return c.JSON(fiber.Map{
		"alertsGenerated": len(alerts),
		"alerts":          alerts,
	})
}

func (h *MonitorHandler) GetData(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}

	monitor, err := h.monitorRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
	}

	data, err := h.monitorRepo.QueryData(c.Context(), monitor.CollectionID, nil, 100)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"data":   data,
		"total":  monitor.RecordCount,
		"schema": monitor.Schema,
	})
}
