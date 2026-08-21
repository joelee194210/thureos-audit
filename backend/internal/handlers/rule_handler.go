package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
	"github.com/thureos/compliance/internal/services"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RuleHandler struct {
	ruleRepo       *repository.RuleRepository
	monitorRepo    *repository.MonitorRepository
	aiRulesService *services.AIRulesService
	ruleEngine     *services.RuleEngine
}

func NewRuleHandler(
	ruleRepo *repository.RuleRepository,
	monitorRepo *repository.MonitorRepository,
	aiRulesService *services.AIRulesService,
	ruleEngine *services.RuleEngine,
) *RuleHandler {
	return &RuleHandler{
		ruleRepo:       ruleRepo,
		monitorRepo:    monitorRepo,
		aiRulesService: aiRulesService,
		ruleEngine:     ruleEngine,
	}
}

func (h *RuleHandler) Create(c *fiber.Ctx) error {
	var req models.CreateRuleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	monitorID, err := primitive.ObjectIDFromHex(req.MonitorID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}

	userID, _ := primitive.ObjectIDFromHex(c.Locals("userId").(string))

	rule := &models.Rule{
		MonitorID:           monitorID,
		Name:                req.Name,
		Description:         req.Description,
		ConditionGroup:      req.ConditionGroup,
		AggregateConditions: req.AggregateConditions,
		Actions:             req.Actions,
		Severity:            req.Severity,
		CreatedBy:           userID,
	}

	// Apply schedule if provided
	if req.Schedule != nil && req.Schedule.Enabled {
		rule.Schedule = *req.Schedule
		if rule.Schedule.Preset != "" {
			rule.Schedule.CronExpr = models.PresetToCron(rule.Schedule.Preset)
		}
		if rule.Schedule.CronExpr != "" {
			nextRun := services.CalculateNextRun(rule.Schedule.CronExpr)
			rule.Schedule.NextRun = &nextRun
		}
	}

	if err := h.ruleRepo.Create(c.Context(), rule); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(rule)
}

func (h *RuleHandler) List(c *fiber.Ctx) error {
	monitorID := c.Query("monitorId")

	var rules []models.Rule
	var err error

	if monitorID != "" {
		oid, parseErr := primitive.ObjectIDFromHex(monitorID)
		if parseErr != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
		}
		rules, err = h.ruleRepo.FindByMonitor(c.Context(), oid)
	} else {
		rules, err = h.ruleRepo.FindAll(c.Context())
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if rules == nil {
		rules = []models.Rule{}
	}
	return c.JSON(rules)
}

func (h *RuleHandler) Get(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid rule ID"})
	}

	rule, err := h.ruleRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "rule not found"})
	}

	return c.JSON(rule)
}

// updateRuleRequest mirrors the JSON body for rule updates with proper BSON mapping.
type updateRuleRequest struct {
	Name                *string                     `json:"name" bson:"name,omitempty"`
	Description         *string                     `json:"description" bson:"description,omitempty"`
	Severity            *models.Severity            `json:"severity" bson:"severity,omitempty"`
	Active              *bool                       `json:"active" bson:"active,omitempty"`
	Actions             []models.ActionType         `json:"actions" bson:"actions,omitempty"`
	ConditionGroup      *models.ConditionGroup      `json:"conditionGroup" bson:"condition_group,omitempty"`
	AggregateConditions []models.AggregateCondition `json:"aggregateConditions" bson:"aggregate_conditions,omitempty"`
	Schedule            *models.RuleSchedule        `json:"schedule" bson:"schedule,omitempty"`
}

func (h *RuleHandler) Update(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid rule ID"})
	}

	var req updateRuleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	// Build update map with BSON keys
	update := bson.M{}
	if req.Name != nil {
		update["name"] = *req.Name
	}
	if req.Description != nil {
		update["description"] = *req.Description
	}
	if req.Severity != nil {
		update["severity"] = *req.Severity
	}
	if req.Active != nil {
		update["active"] = *req.Active
	}
	if req.Actions != nil {
		update["actions"] = req.Actions
	}
	if req.ConditionGroup != nil {
		update["condition_group"] = req.ConditionGroup
	}
	if req.AggregateConditions != nil {
		update["aggregate_conditions"] = req.AggregateConditions
	}
	if req.Schedule != nil {
		sched := *req.Schedule
		if sched.Preset != "" {
			sched.CronExpr = models.PresetToCron(sched.Preset)
		}
		if sched.Enabled && sched.CronExpr != "" {
			nextRun := services.CalculateNextRun(sched.CronExpr)
			sched.NextRun = &nextRun
		}
		update["schedule"] = sched
	}

	if len(update) == 0 {
		return c.JSON(fiber.Map{"message": "nothing to update"})
	}

	if err := h.ruleRepo.Update(c.Context(), id, update); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "rule updated"})
}

func (h *RuleHandler) Delete(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid rule ID"})
	}

	if err := h.ruleRepo.Delete(c.Context(), id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "rule deleted"})
}

// Execute runs a rule against today's data in real-time
func (h *RuleHandler) Execute(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid rule ID"})
	}

	rule, err := h.ruleRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "rule not found"})
	}

	monitor, err := h.monitorRepo.FindByID(c.Context(), rule.MonitorID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
	}

	redFlags, err := h.ruleEngine.EvaluateRuleNow(c.Context(), monitor, *rule)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"redFlagsGenerated": len(redFlags),
		"redFlags":          redFlags,
	})
}

func (h *RuleHandler) GenerateAIRules(c *fiber.Ctx) error {
	var req models.AIRuleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	monitorID, err := primitive.ObjectIDFromHex(req.MonitorID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}

	monitor, err := h.monitorRepo.FindByID(c.Context(), monitorID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
	}

	suggestions, err := h.aiRulesService.GenerateRules(c.Context(), monitor.Schema, req.DataSample, req.Prompt)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"suggestions": suggestions})
}
