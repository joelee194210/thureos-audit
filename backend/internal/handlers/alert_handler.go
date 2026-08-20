package handlers

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
	"github.com/thureos/compliance/internal/services"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AlertHandler struct {
	alertRepo    *repository.AlertRepository
	alertLogRepo *repository.AlertLogRepository
	ruleRepo     *repository.RuleRepository
	monitorRepo  *repository.MonitorRepository
	userRepo     *repository.UserRepository
	activityRepo *repository.ActivityLogRepository
}

func NewAlertHandler(
	alertRepo *repository.AlertRepository,
	alertLogRepo *repository.AlertLogRepository,
	ruleRepo *repository.RuleRepository,
	monitorRepo *repository.MonitorRepository,
	userRepo *repository.UserRepository,
	activityRepo *repository.ActivityLogRepository,
) *AlertHandler {
	return &AlertHandler{
		alertRepo:    alertRepo,
		alertLogRepo: alertLogRepo,
		ruleRepo:     ruleRepo,
		monitorRepo:  monitorRepo,
		userRepo:     userRepo,
		activityRepo: activityRepo,
	}
}

func (h *AlertHandler) List(c *fiber.Ctx) error {
	limitStr := c.Query("limit", "50")
	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil || limit < 1 || limit > 500 {
		limit = 50
	}

	monitorID := c.Query("monitorId")

	var alerts []models.Alert

	if monitorID != "" {
		oid, parseErr := primitive.ObjectIDFromHex(monitorID)
		if parseErr != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
		}
		alerts, err = h.alertRepo.FindByMonitor(c.Context(), oid, limit)
	} else {
		alerts, err = h.alertRepo.FindRecent(c.Context(), limit)
	}

	if err != nil {
		log.Printf("ERROR listing alerts: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error listing alerts"})
	}
	if alerts == nil {
		alerts = []models.Alert{}
	}
	return c.JSON(alerts)
}

func (h *AlertHandler) Get(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid alert ID"})
	}

	alert, err := h.alertRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "alert not found"})
	}

	return c.JSON(alert)
}

func (h *AlertHandler) UpdateStatus(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid alert ID"})
	}

	var body struct {
		Status   string `json:"status"`
		Category string `json:"category"`
		Notes    string `json:"notes"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	status := models.AlertStatus(body.Status)
	switch status {
	case models.AlertNew, models.AlertAcknowledged, models.AlertResolved, models.AlertDismissed:
		// valid
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid status value"})
	}

	// Validate category and notes for non-new statuses
	category := models.ActionCategory(body.Category)
	if status != models.AlertNew {
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

	// Fetch current alert for snapshot and previous status
	alert, err := h.alertRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "alert not found"})
	}

	// Fetch user for log entry
	user, err := h.userRepo.FindByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "user not found"})
	}

	// Create audit log entry BEFORE updating status to ensure traceability.
	// If the audit log fails, we refuse to update — every status change must be recorded.
	alertLog := &models.AlertLog{
		AlertID:        id,
		PreviousStatus: alert.Status,
		NewStatus:      status,
		UserID:         userID,
		UserName:       user.Name,
		UserEmail:      user.Email,
		Category:       category,
		Notes:          strings.TrimSpace(body.Notes),
		AlertSnapshot:  *alert,
	}
	if err := h.alertLogRepo.Create(c.Context(), alertLog); err != nil {
		log.Printf("ERROR: failed to create alert audit log: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error creating audit log"})
	}

	// Update alert status (audit log is already persisted)
	if err := h.alertRepo.UpdateStatus(c.Context(), id, status, &userID); err != nil {
		log.Printf("ERROR updating alert status: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error updating alert status"})
	}

	// Log activity
	if h.activityRepo != nil {
		actLog := &models.ActivityLog{
			UserID:     userID,
			UserName:   user.Name,
			UserEmail:  user.Email,
			Action:     models.ActivityAlertAction,
			Detail:     fmt.Sprintf("Alerta '%s' cambiada a %s", alert.RuleName, status),
			Resource:   "alert",
			ResourceID: id.Hex(),
			IP:         c.IP(),
		}
		if err := h.activityRepo.Create(c.Context(), actLog); err != nil {
			log.Printf("WARNING: failed to log alert activity: %v", err)
		}
	}

	return c.JSON(fiber.Map{"message": "alert status updated"})
}

// GetLogs returns the audit trail for a specific alert
func (h *AlertHandler) GetLogs(c *fiber.Ctx) error {
	alertID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid alert ID"})
	}

	logs, err := h.alertLogRepo.FindByAlertID(c.Context(), alertID)
	if err != nil {
		log.Printf("ERROR fetching alert logs: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error fetching alert logs"})
	}

	return c.JSON(logs)
}

func (h *AlertHandler) Stats(c *fiber.Ctx) error {
	newCount, err := h.alertRepo.CountByStatus(c.Context(), models.AlertNew)
	if err != nil {
		log.Printf("ERROR counting new alerts: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error fetching alert stats"})
	}
	ackCount, err := h.alertRepo.CountByStatus(c.Context(), models.AlertAcknowledged)
	if err != nil {
		log.Printf("ERROR counting acknowledged alerts: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error fetching alert stats"})
	}

	return c.JSON(fiber.Map{
		"new":          newCount,
		"acknowledged": ackCount,
	})
}

// GetRecords re-queries the monitor's data collection using the rule's conditions.
// Supports pagination: ?page=1&limit=50
// For row alerts: rebuilds the MongoDB filter from the rule's conditionGroup
// For aggregate alerts: filters by groupByField = groupByValue
func (h *AlertHandler) GetRecords(c *fiber.Ctx) error {
	alertID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid alert ID"})
	}

	page, err := strconv.ParseInt(c.Query("page", "1"), 10, 64)
	if err != nil || page < 1 {
		page = 1
	}
	limit, err := strconv.ParseInt(c.Query("limit", "50"), 10, 64)
	if err != nil || limit < 1 || limit > 500 {
		limit = 50
	}

	alert, err := h.alertRepo.FindByID(c.Context(), alertID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "alert not found"})
	}

	monitor, err := h.monitorRepo.FindByID(c.Context(), alert.MonitorID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "monitor not found"})
	}

	var filter bson.M

	if alert.AlertType == models.AlertTypeAggregate {
		filter = bson.M{}
		if alert.GroupByField != "" && alert.GroupByValue != "" {
			if strings.HasPrefix(alert.GroupByField, "$") {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid group by field"})
			}
			filter = bson.M{alert.GroupByField: alert.GroupByValue}
		}
	} else {
		rule, err := h.ruleRepo.FindByID(c.Context(), alert.RuleID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "rule not found"})
		}
		filter = services.BuildMongoFilter(rule.ConditionGroup)
	}

	records, total, err := h.monitorRepo.QueryDataPaginated(c.Context(), monitor.CollectionID, filter, page, limit)
	if err != nil {
		log.Printf("ERROR querying alert records: %v", err)
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

// ByDate returns alerts for a specific date (YYYY-MM-DD)
func (h *AlertHandler) ByDate(c *fiber.Ctx) error {
	dateStr := c.Params("date")
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid date format, use YYYY-MM-DD"})
	}

	alerts, err := h.alertRepo.FindByDateRange(c.Context(), date, date.AddDate(0, 0, 1))
	if err != nil {
		log.Printf("ERROR fetching alerts by date: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error fetching alerts"})
	}
	if alerts == nil {
		alerts = []models.Alert{}
	}
	return c.JSON(alerts)
}

// Calendar returns alert counts per day for a given month
// Query params: year, month (both required)
func (h *AlertHandler) Calendar(c *fiber.Ctx) error {
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

	counts, err := h.alertRepo.CountByDay(c.Context(), year, month)
	if err != nil {
		log.Printf("ERROR fetching alert calendar: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error fetching calendar data"})
	}

	return c.JSON(counts)
}
