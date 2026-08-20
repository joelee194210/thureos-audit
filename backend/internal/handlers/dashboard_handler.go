package handlers

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
	"github.com/thureos/compliance/internal/services"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type DashboardHandler struct {
	dashboardRepo    *repository.DashboardRepository
	dashboardService *services.DashboardService
}

func NewDashboardHandler(
	dashboardRepo *repository.DashboardRepository,
	dashboardService *services.DashboardService,
) *DashboardHandler {
	return &DashboardHandler{
		dashboardRepo:    dashboardRepo,
		dashboardService: dashboardService,
	}
}

func (h *DashboardHandler) Create(c *fiber.Ctx) error {
	var req models.CreateDashboardRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	userID, _ := primitive.ObjectIDFromHex(c.Locals("userId").(string))

	monitorIDs := make([]primitive.ObjectID, 0, len(req.MonitorIDs))
	for _, id := range req.MonitorIDs {
		oid, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			continue
		}
		monitorIDs = append(monitorIDs, oid)
	}

	dashboard := &models.Dashboard{
		Name:        req.Name,
		Description: req.Description,
		MonitorIDs:  monitorIDs,
		OwnerID:     userID,
		Widgets:     []models.Widget{},
	}

	if err := h.dashboardRepo.Create(c.Context(), dashboard); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(dashboard)
}

func (h *DashboardHandler) List(c *fiber.Ctx) error {
	userID, _ := primitive.ObjectIDFromHex(c.Locals("userId").(string))

	dashboards, err := h.dashboardRepo.FindAccessible(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if dashboards == nil {
		dashboards = []models.Dashboard{}
	}
	return c.JSON(dashboards)
}

func (h *DashboardHandler) Get(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid dashboard ID"})
	}

	dashboard, err := h.dashboardRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "dashboard not found"})
	}

	return c.JSON(dashboard)
}

func (h *DashboardHandler) AddWidget(c *fiber.Ctx) error {
	dashboardID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid dashboard ID"})
	}

	var req models.AddWidgetRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	widget := models.Widget{
		ID:          primitive.NewObjectID().Hex(),
		Title:       req.Title,
		Type:        req.Type,
		MonitorID:   req.MonitorID,
		Field:       req.Field,
		Aggregation: req.Aggregation,
		GroupBy:     req.GroupBy,
		RuleID:      req.RuleID,
		Position:    req.Position,
	}

	// If ruleId provided, auto-configure widget from the rule
	if req.RuleID != "" {
		if err := h.dashboardService.ConfigureWidgetFromRule(c.Context(), req.RuleID, &widget); err != nil {
			log.Printf("Error configuring widget from rule: %v", err)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "could not configure widget from rule"})
		}
	}

	dashboard, err := h.dashboardRepo.FindByID(c.Context(), dashboardID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "dashboard not found"})
	}

	dashboard.Widgets = append(dashboard.Widgets, widget)

	if err := h.dashboardRepo.Update(c.Context(), dashboardID, bson.M{
		"widgets": dashboard.Widgets,
	}); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(widget)
}

func (h *DashboardHandler) DeleteWidget(c *fiber.Ctx) error {
	dashboardID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid dashboard ID"})
	}

	widgetID := c.Params("widgetId")

	dashboard, err := h.dashboardRepo.FindByID(c.Context(), dashboardID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "dashboard not found"})
	}

	filtered := make([]models.Widget, 0, len(dashboard.Widgets))
	for _, w := range dashboard.Widgets {
		if w.ID != widgetID {
			filtered = append(filtered, w)
		}
	}

	if len(filtered) == len(dashboard.Widgets) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "widget not found"})
	}

	if err := h.dashboardRepo.Update(c.Context(), dashboardID, bson.M{
		"widgets": filtered,
	}); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "widget deleted"})
}

func (h *DashboardHandler) GetData(c *fiber.Ctx) error {
	id := c.Params("id")

	data, err := h.dashboardService.GetDashboardData(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"widgets": data})
}

func (h *DashboardHandler) Delete(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid dashboard ID"})
	}

	if err := h.dashboardRepo.Delete(c.Context(), id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "dashboard deleted"})
}

func (h *DashboardHandler) DrillDown(c *fiber.Ctx) error {
	dashboardID := c.Params("id")
	widgetID := c.Params("widgetId")

	var req models.DrilldownRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	result, err := h.dashboardService.DrillDown(
		c.Context(), dashboardID, widgetID,
		req.GroupValue, req.Page, req.PageSize, req.Search, req.Export,
	)
	if err != nil {
		log.Printf("Drill-down error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not fetch drill-down data"})
	}

	return c.JSON(result)
}
