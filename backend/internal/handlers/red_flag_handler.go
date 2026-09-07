package handlers

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
	"github.com/thureos/compliance/internal/services"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type RedFlagHandler struct {
	redFlagRepo    *repository.RedFlagRepository
	redFlagLogRepo *repository.RedFlagLogRepository
	ruleRepo       *repository.RuleRepository
	monitorRepo    *repository.MonitorRepository
	userRepo       *repository.UserRepository
	activityRepo   *repository.ActivityLogRepository
	reportRepo     *repository.RedFlagReportRepository
	caseRepo       *repository.RedFlagCaseRepository
}

func NewRedFlagHandler(
	redFlagRepo *repository.RedFlagRepository,
	redFlagLogRepo *repository.RedFlagLogRepository,
	ruleRepo *repository.RuleRepository,
	monitorRepo *repository.MonitorRepository,
	userRepo *repository.UserRepository,
	activityRepo *repository.ActivityLogRepository,
	reportRepo *repository.RedFlagReportRepository,
) *RedFlagHandler {
	return &RedFlagHandler{
		redFlagRepo:    redFlagRepo,
		redFlagLogRepo: redFlagLogRepo,
		ruleRepo:       ruleRepo,
		monitorRepo:    monitorRepo,
		userRepo:       userRepo,
		activityRepo:   activityRepo,
		reportRepo:     reportRepo,
	}
}

// SetCaseRepo conecta el repositorio de casos (opcional para no romper
// los constructores existentes).
func (h *RedFlagHandler) SetCaseRepo(r *repository.RedFlagCaseRepository) {
	h.caseRepo = r
}

// GetReport descarga el PDF guardado automáticamente para esta bandera
// roja. No lo regenera — si no hay uno guardado (por ejemplo, alertas
// creadas antes de que existiera este endpoint), responde 404.
func (h *RedFlagHandler) GetReport(c *fiber.Ctx) error {
	redFlagID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid red flag ID"})
	}

	pdf, err := h.reportRepo.Get(c.Context(), redFlagID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "no hay informe guardado para esta alerta"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="informe-bandera-roja-%s.pdf"`, redFlagID.Hex()))
	return c.Send(pdf)
}

func (h *RedFlagHandler) List(c *fiber.Ctx) error {
	limitStr := c.Query("limit", "50")
	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil || limit < 1 || limit > 500 {
		limit = 50
	}

	monitorID := c.Query("monitorId")

	var redFlags []models.RedFlag

	if monitorID != "" {
		oid, parseErr := primitive.ObjectIDFromHex(monitorID)
		if parseErr != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
		}
		redFlags, err = h.redFlagRepo.FindByMonitor(c.Context(), oid, limit)
	} else {
		redFlags, err = h.redFlagRepo.FindRecent(c.Context(), limit)
	}

	if err != nil {
		log.Printf("ERROR listing red flags: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error listing red flags"})
	}
	if redFlags == nil {
		redFlags = []models.RedFlag{}
	}
	return c.JSON(redFlags)
}

func (h *RedFlagHandler) Get(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid red flag ID"})
	}

	redFlag, err := h.redFlagRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "red flag not found"})
	}

	return c.JSON(redFlag)
}

func (h *RedFlagHandler) UpdateStatus(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid red flag ID"})
	}

	var body struct {
		Status   string `json:"status"`
		Category string `json:"category"`
		Notes    string `json:"notes"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	status := models.RedFlagStatus(body.Status)
	switch status {
	case models.RedFlagNew, models.RedFlagAcknowledged, models.RedFlagResolved, models.RedFlagDismissed:
		// valid
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid status value"})
	}

	// Validate category and notes for non-new statuses
	category := models.ActionCategory(body.Category)
	if status != models.RedFlagNew {
		switch category {
		case models.CategoryInvestigation, models.CategoryFalsePositive, models.CategoryEscalation,
			models.CategoryCorrectiveAction, models.CategoryOther:
			// valid
		default:
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "category is required"})
		}
		if strings.TrimSpace(body.Notes) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "notes are required"})
		}
	}

	userIDStr, ok := c.Locals("userId").(string)
	if !ok || userIDStr == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	userID, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user session"})
	}

	// Fetch current red flag for snapshot and previous status
	redFlag, err := h.redFlagRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "red flag not found"})
	}

	// Fetch user for log entry
	user, err := h.userRepo.FindByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "user not found"})
	}

	// Create audit log entry BEFORE updating status to ensure traceability.
	// If the audit log fails, we refuse to update — every status change must be recorded.
	redFlagLog := &models.RedFlagLog{
		RedFlagID:       id,
		PreviousStatus:  redFlag.Status,
		NewStatus:       status,
		UserID:          userID,
		UserName:        user.Name,
		UserEmail:       user.Email,
		Category:        category,
		Notes:           strings.TrimSpace(body.Notes),
		RedFlagSnapshot: *redFlag,
	}
	if err := h.redFlagLogRepo.Create(c.Context(), redFlagLog); err != nil {
		log.Printf("ERROR: failed to create red flag audit log: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error creating audit log"})
	}

	// Update red flag status (audit log is already persisted)
	if err := h.redFlagRepo.UpdateStatus(c.Context(), id, status, &userID); err != nil {
		log.Printf("ERROR updating red flag status: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error updating red flag status"})
	}

	// Log activity
	if h.activityRepo != nil {
		actLog := &models.ActivityLog{
			UserID:     userID,
			UserName:   user.Name,
			UserEmail:  user.Email,
			Action:     models.ActivityRedFlagAction,
			Detail:     fmt.Sprintf("Bandera roja '%s' cambiada a %s", redFlag.RuleName, status),
			Resource:   "red_flag",
			ResourceID: id.Hex(),
			IP:         c.IP(),
		}
		if err := h.activityRepo.Create(c.Context(), actLog); err != nil {
			log.Printf("WARNING: failed to log red flag activity: %v", err)
		}
	}

	return c.JSON(fiber.Map{"message": "red flag status updated"})
}

// GetLogs returns the audit trail for a specific red flag
func (h *RedFlagHandler) GetLogs(c *fiber.Ctx) error {
	redFlagID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid red flag ID"})
	}

	logs, err := h.redFlagLogRepo.FindByRedFlagID(c.Context(), redFlagID)
	if err != nil {
		log.Printf("ERROR fetching red flag logs: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error fetching red flag logs"})
	}

	return c.JSON(logs)
}

func (h *RedFlagHandler) Stats(c *fiber.Ctx) error {
	newCount, err := h.redFlagRepo.CountByStatus(c.Context(), models.RedFlagNew)
	if err != nil {
		log.Printf("ERROR counting new red flags: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error fetching red flag stats"})
	}
	ackCount, err := h.redFlagRepo.CountByStatus(c.Context(), models.RedFlagAcknowledged)
	if err != nil {
		log.Printf("ERROR counting acknowledged red flags: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error fetching red flag stats"})
	}

	return c.JSON(fiber.Map{
		"new":          newCount,
		"acknowledged": ackCount,
	})
}

// GetRecords re-queries the monitor's data collection using the rule's conditions.
// Supports pagination: ?page=1&limit=50
// For row red flags: rebuilds the MongoDB filter from the rule's conditionGroup
// For aggregate red flags: filters by groupByField = groupByValue
func (h *RedFlagHandler) GetRecords(c *fiber.Ctx) error {
	redFlagID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid red flag ID"})
	}

	page, err := strconv.ParseInt(c.Query("page", "1"), 10, 64)
	if err != nil || page < 1 {
		page = 1
	}
	limit, err := strconv.ParseInt(c.Query("limit", "50"), 10, 64)
	if err != nil || limit < 1 || limit > 500 {
		limit = 50
	}

	redFlag, err := h.redFlagRepo.FindByID(c.Context(), redFlagID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "red flag not found"})
	}

	monitor, err := h.monitorRepo.FindByID(c.Context(), redFlag.MonitorID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "monitor not found"})
	}

	var filter bson.M

	if redFlag.RedFlagType == models.RedFlagTypeAggregate {
		filter = bson.M{}
		if redFlag.GroupByField != "" && redFlag.GroupByValue != "" {
			if strings.HasPrefix(redFlag.GroupByField, "$") {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid group by field"})
			}
			filter = bson.M{redFlag.GroupByField: redFlag.GroupByValue}
		}
	} else {
		rule, err := h.ruleRepo.FindByID(c.Context(), redFlag.RuleID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "rule not found"})
		}
		filter = services.BuildMongoFilter(rule.ConditionGroup)
	}

	records, total, err := h.monitorRepo.QueryDataPaginated(c.Context(), monitor.CollectionID, filter, page, limit)
	if err != nil {
		log.Printf("ERROR querying red flag records: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error querying records"})
	}

	// Clean _id fields for JSON serialization
	cleaned := make([]map[string]interface{}, len(records))
	for i, rec := range records {
		m := make(map[string]interface{}, len(rec))
		for k, v := range rec {
			if k == "_id" {
				continue
			}
			// Convert numeric strings to numbers for proper display
			if s, ok := v.(string); ok {
				s = strings.TrimSpace(s)
				if f, err := strconv.ParseFloat(s, 64); err == nil && s != "" {
					m[k] = f
					continue
				}
			}
			m[k] = v
		}
		cleaned[i] = m
	}

	return c.JSON(fiber.Map{
		"records":    cleaned,
		"total":      total,
		"page":       page,
		"limit":      limit,
		"totalPages": (total + limit - 1) / limit,
	})
}

// ByDate returns red flags for a specific date (YYYY-MM-DD)
func (h *RedFlagHandler) ByDate(c *fiber.Ctx) error {
	dateStr := c.Params("date")
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid date format, use YYYY-MM-DD"})
	}

	redFlags, err := h.redFlagRepo.FindByDateRange(c.Context(), date, date.AddDate(0, 0, 1))
	if err != nil {
		log.Printf("ERROR fetching red flags by date: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error fetching red flags"})
	}
	if redFlags == nil {
		redFlags = []models.RedFlag{}
	}
	return c.JSON(redFlags)
}

// Calendar returns red flag counts per day for a given month
// Query params: year, month (both required)
func (h *RedFlagHandler) Calendar(c *fiber.Ctx) error {
	yearStr := c.Query("year")
	monthStr := c.Query("month")

	if yearStr == "" || monthStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "year and month are required"})
	}

	year, err := strconv.Atoi(yearStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid year"})
	}
	month, err := strconv.Atoi(monthStr)
	if err != nil || month < 1 || month > 12 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid month"})
	}

	counts, err := h.redFlagRepo.CountByDay(c.Context(), year, month)
	if err != nil {
		log.Printf("ERROR fetching red flag calendar: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error fetching calendar data"})
	}

	return c.JSON(counts)
}

// caseUserID resuelve el usuario autenticado o aborta con 401.
func caseUserID(c *fiber.Ctx) (primitive.ObjectID, error) {
	idStr, ok := c.Locals("userId").(string)
	if !ok || idStr == "" {
		return primitive.NilObjectID, fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return primitive.NilObjectID, fiber.NewError(fiber.StatusBadRequest, "invalid user session")
	}
	return id, nil
}

// AssignCase asigna el caso a un usuario (o al propio analista con
// "me"). Setea prioridad y SLA derivados de la severidad de la alerta.
func (h *RedFlagHandler) AssignCase(c *fiber.Ctx) error {
	if h.caseRepo == nil {
		return fiber.NewError(fiber.StatusNotImplemented, "case management no disponible")
	}
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid red flag ID"})
	}

	var body struct {
		AssigneeID string `json:"assigneeId"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	var assignee primitive.ObjectID
	if body.AssigneeID == "me" {
		assignee, err = caseUserID(c)
		if err != nil {
			return err
		}
	} else {
		assignee, err = primitive.ObjectIDFromHex(body.AssigneeID)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid assigneeId"})
		}
	}

	rf, err := h.redFlagRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "red flag not found"})
	}

	if err := h.caseRepo.Assign(c.Context(), id, assignee,
		services.PriorityForSeverity(rf.Severity),
		services.SLADueAt(rf)); err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	}

	h.logCaseActivity(c, assignee, rf, "asignó el caso")
	return c.JSON(fiber.Map{"message": "caso asignado"})
}

// AddCaseNote agrega una nota al timeline de investigación.
func (h *RedFlagHandler) AddCaseNote(c *fiber.Ctx) error {
	if h.caseRepo == nil {
		return fiber.NewError(fiber.StatusNotImplemented, "case management no disponible")
	}
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid red flag ID"})
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	authorID, err := caseUserID(c)
	if err != nil {
		return err
	}
	authorName, _ := c.Locals("email").(string)

	note := &models.RedFlagNote{
		RedFlagID:  id,
		AuthorID:   authorID,
		AuthorName: authorName,
		Text:       body.Text,
	}
	if err := h.caseRepo.AddNote(c.Context(), note); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(note)
}

// ListCaseNotes devuelve el timeline del caso, más reciente primero.
func (h *RedFlagHandler) ListCaseNotes(c *fiber.Ctx) error {
	if h.caseRepo == nil {
		return fiber.NewError(fiber.StatusNotImplemented, "case management no disponible")
	}
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid red flag ID"})
	}
	notes, err := h.caseRepo.ListNotes(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(notes)
}

// TransitionCase mueve el caso por la máquina de estados validada. El
// cierre exige disposition (false_positive, confirmed_ros, no_action).
func (h *RedFlagHandler) TransitionCase(c *fiber.Ctx) error {
	if h.caseRepo == nil {
		return fiber.NewError(fiber.StatusNotImplemented, "case management no disponible")
	}
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid red flag ID"})
	}
	var body struct {
		Status      string `json:"status"`
		Disposition string `json:"disposition"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	rf, err := h.redFlagRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "red flag not found"})
	}

	to := models.RedFlagStatus(body.Status)
	disposition := models.RedFlagDisposition(body.Disposition)
	if err := services.ValidTransition(rf.Status, to, disposition); err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	}

	userID, err := caseUserID(c)
	if err != nil {
		return err
	}
	if err := h.caseRepo.Transition(c.Context(), id, to, disposition, userID); err != nil {
		if errors.Is(err, repository.ErrCaseClosed) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	h.logCaseActivity(c, userID, rf, "movió el caso a "+string(to))
	return c.JSON(fiber.Map{"message": "caso en " + string(to)})
}

// logCaseActivity deja rastro en la bitácora existente — el timeline de
// auditoría del caso es la bitácora de actividad del sistema.
func (h *RedFlagHandler) logCaseActivity(c *fiber.Ctx, userID primitive.ObjectID, rf *models.RedFlag, action string) {
	if h.activityRepo == nil {
		return
	}
	userName, _ := c.Locals("email").(string)
	_ = h.activityRepo.Create(c.Context(), &models.ActivityLog{
		UserID:    userID,
		UserName:  userName,
		UserEmail: userName,
		Action:    models.ActivityRedFlagAction,
		Detail:    fmt.Sprintf("Caso %s (%s): %s", rf.ID.Hex(), rf.RuleName, action),
		Resource:  "red_flag",
		IP:        c.IP(),
	})
}
