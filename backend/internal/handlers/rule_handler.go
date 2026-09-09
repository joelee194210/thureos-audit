package handlers

import (
	"context"
	"log"
	"time"

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
	redFlagRepo    *repository.RedFlagRepository
	aiRulesService *services.AIRulesService
	ruleEngine     *services.RuleEngine
}

func NewRuleHandler(
	ruleRepo *repository.RuleRepository,
	monitorRepo *repository.MonitorRepository,
	redFlagRepo *repository.RedFlagRepository,
	aiRulesService *services.AIRulesService,
	ruleEngine *services.RuleEngine,
) *RuleHandler {
	return &RuleHandler{
		ruleRepo:       ruleRepo,
		monitorRepo:    monitorRepo,
		redFlagRepo:    redFlagRepo,
		aiRulesService: aiRulesService,
		ruleEngine:     ruleEngine,
	}
}

// Effectiveness agrega las red flags que esta regla generó: desglose por
// disposición, tasa de falsos positivos y tiempo medio de cierre.
func (h *RuleHandler) Effectiveness(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid rule ID"})
	}
	if _, err := h.ruleRepo.FindByID(c.Context(), id); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "rule not found"})
	}
	flags, err := h.redFlagRepo.FindByRuleID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(services.ComputeEffectiveness(flags))
}

// maxBacktestSample caps how many candidate matches Backtest returns in
// full — a preview only needs enough to judge signal vs. noise, not the
// whole set.
const maxBacktestSample = 20

// backtestRuleRequest mirrors the parts of CreateRuleRequest a backtest
// needs to run the matcher — no MonitorID (viene del path), nada de lo
// que solo aplica a una regla ya persistida (schedule, actions).
type backtestRuleRequest struct {
	ConditionGroup      models.ConditionGroup       `json:"conditionGroup"`
	AggregateConditions []models.AggregateCondition `json:"aggregateConditions,omitempty"`
	Severity            models.Severity             `json:"severity"`
	From                *time.Time                  `json:"from,omitempty"`
	To                  *time.Time                  `json:"to,omitempty"`
}

// Backtest reporta qué habría generado una regla (sin guardarla) contra
// los datos ya ingeridos de un monitor — para juzgar señal vs. ruido
// antes de activarla. No persiste nada: ni red flag, ni notificación, ni
// trigger_count.
func (h *RuleHandler) Backtest(c *fiber.Ctx) error {
	monitorID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}
	monitor, err := h.monitorRepo.FindByID(c.Context(), monitorID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor no encontrado"})
	}

	var req backtestRuleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if len(req.ConditionGroup.Conditions) == 0 && len(req.AggregateConditions) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "la regla necesita al menos una condición para probar"})
	}

	rule := models.Rule{
		MonitorID:           monitorID,
		ConditionGroup:      req.ConditionGroup,
		AggregateConditions: req.AggregateConditions,
		Severity:            req.Severity,
	}

	from := time.Time{} // todo el histórico ingerido, por defecto
	if req.From != nil {
		from = *req.From
	}
	to := time.Now()
	if req.To != nil {
		to = *req.To
	}

	candidates := h.ruleEngine.BacktestRule(c.Context(), monitor, rule, from, to)

	sample := candidates
	if len(sample) > maxBacktestSample {
		sample = sample[:maxBacktestSample]
	}
	return c.JSON(fiber.Map{
		"matchCount": len(candidates),
		"sample":     sample,
	})
}

// ensureVelocityIndexes crea el índice que el pipeline de velocidad necesita
// para ordenar por agrupación y tiempo. Es una optimización, no un requisito
// de correctitud: si falla se registra y la regla se guarda igual, porque una
// regla sin índice funciona (más lento) y una regla que no se puede guardar
// por un índice no funciona en absoluto.
func (h *RuleHandler) ensureVelocityIndexes(ctx context.Context, monitor *models.Monitor, conds []models.VelocityCondition) {
	for _, vc := range conds {
		keys := bson.D{}
		if vc.GroupBy != "" {
			keys = append(keys, bson.E{Key: vc.GroupBy, Value: 1})
		}
		keys = append(keys, bson.E{Key: vc.TimeField, Value: 1})
		if err := h.monitorRepo.EnsureDataIndex(ctx, monitor.CollectionID, keys); err != nil {
			log.Printf("WARNING: índice de velocidad para el monitor %s: %v", monitor.ID.Hex(), err)
		}
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

	// Las condiciones de velocidad se validan contra el schema del monitor
	// al guardar: una condición que apunta a un campo que no es date jamás
	// dispararía, y aceptarla en silencio repite el modo de falla que dejó
	// la ventana temporal rota en producción.
	if len(req.VelocityConditions) > 0 {
		monitor, err := h.monitorRepo.FindByID(c.Context(), monitorID)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
		}
		req.VelocityConditions = services.NormalizeVelocityConditions(req.VelocityConditions)
		for _, vc := range req.VelocityConditions {
			if err := services.ValidateVelocityCondition(vc, monitor.Schema); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
			}
		}
		h.ensureVelocityIndexes(c.Context(), monitor, req.VelocityConditions)
	}

	rule := &models.Rule{
		MonitorID:           monitorID,
		Name:                req.Name,
		Description:         req.Description,
		ConditionGroup:      req.ConditionGroup,
		AggregateConditions: req.AggregateConditions,
		VelocityConditions:  req.VelocityConditions,
		Actions:             req.Actions,
		Severity:            req.Severity,
		AIGenerated:         req.AIGenerated,
		ScreeningFields:     req.ScreeningFields,
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
	VelocityConditions  []models.VelocityCondition  `json:"velocityConditions" bson:"velocity_conditions,omitempty"`
	Schedule            *models.RuleSchedule        `json:"schedule" bson:"schedule,omitempty"`
	ScreeningFields     []string                    `json:"screeningFields" bson:"screening_fields,omitempty"`
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
	if req.VelocityConditions != nil {
		// Misma validación que al crear: sin esto, editar sería una puerta
		// trasera para guardar una condición que jamás dispararía — el modo
		// de falla silenciosa que toda esta funcionalidad evita.
		rule, err := h.ruleRepo.FindByID(c.Context(), id)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "rule not found"})
		}
		monitor, err := h.monitorRepo.FindByID(c.Context(), rule.MonitorID)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
		}
		req.VelocityConditions = services.NormalizeVelocityConditions(req.VelocityConditions)
		for _, vc := range req.VelocityConditions {
			if err := services.ValidateVelocityCondition(vc, monitor.Schema); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
			}
		}
		h.ensureVelocityIndexes(c.Context(), monitor, req.VelocityConditions)
		update["velocity_conditions"] = req.VelocityConditions
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
	if req.ScreeningFields != nil {
		update["screening_fields"] = req.ScreeningFields
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

	result, err := h.aiRulesService.GenerateRules(c.Context(), monitor.Schema, req.DataSample, req.Prompt, req.Fields)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(result)
}
