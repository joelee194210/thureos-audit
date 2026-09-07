package handlers

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
	"github.com/thureos/compliance/internal/services"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RuleTemplateHandler expone el catálogo de tipologías AML y su
// instanciación como reglas normales de un monitor.
type RuleTemplateHandler struct {
	monitorRepo *repository.MonitorRepository
	ruleRepo    *repository.RuleRepository
	redFlagRepo *repository.RedFlagRepository
}

func NewRuleTemplateHandler(monitorRepo *repository.MonitorRepository, ruleRepo *repository.RuleRepository, redFlagRepo *repository.RedFlagRepository) *RuleTemplateHandler {
	return &RuleTemplateHandler{monitorRepo: monitorRepo, ruleRepo: ruleRepo, redFlagRepo: redFlagRepo}
}

// List devuelve el catálogo completo de tipologías.
func (h *RuleTemplateHandler) List(c *fiber.Ctx) error {
	return c.JSON(services.RuleTemplates)
}

// TemplateEffectiveness es EffectivenessStats agregado a nivel de
// tipología (todas las reglas instanciadas desde el mismo template),
// identificado por id y nombre para que el frontend no tenga que resolver
// el catálogo por su cuenta.
type TemplateEffectiveness struct {
	TemplateID string                      `json:"templateId"`
	Name       string                      `json:"name"`
	Stats      services.EffectivenessStats `json:"stats"`
}

// Effectiveness agrega las red flags de todas las reglas de cada
// tipología — a diferencia de RuleHandler.Effectiveness, que agrega por
// una sola regla.
func (h *RuleTemplateHandler) Effectiveness(c *fiber.Ctx) error {
	rules, err := h.ruleRepo.FindAll(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	groups := services.GroupRuleIDsByTemplate(rules)
	result := make([]TemplateEffectiveness, 0, len(groups))
	for templateID, ruleIDs := range groups {
		flags, err := h.redFlagRepo.FindByRuleIDs(c.Context(), ruleIDs)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		name := templateID
		if tpl, ok := services.FindRuleTemplate(templateID); ok {
			name = tpl.Name
		}
		result = append(result, TemplateEffectiveness{
			TemplateID: templateID,
			Name:       name,
			Stats:      services.ComputeEffectiveness(flags),
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return c.JSON(result)
}

// unknownColumns devuelve las columnas del fieldMap que no existen en el
// esquema del monitor. Las vacías se ignoran: BindTemplate las cobra
// como faltantes con su propio mensaje.
func unknownColumns(fieldMap map[string]string, schema []models.SchemaField) []string {
	known := make(map[string]bool, len(schema))
	for _, sf := range schema {
		known[sf.Name] = true
	}
	var unknown []string
	for _, column := range fieldMap {
		if column == "" {
			continue
		}
		if !known[column] {
			unknown = append(unknown, column)
		}
	}
	sort.Strings(unknown)
	return unknown
}

// InstantiateRequest une cada placeholder ($AMOUNT, $ACCOUNT...) a una
// columna del esquema del monitor; Params ajusta los umbrales (los
// ausentes usan el default de la tipología).
type InstantiateRequest struct {
	TemplateID string             `json:"templateId"`
	FieldMap   map[string]string  `json:"fieldMap"`
	Params     map[string]float64 `json:"params"`
	Name       string             `json:"name"`
}

// Instantiate crea una regla estándar a partir de una tipología del
// catálogo. La validación de columnas es contra el esquema detectado del
// monitor — un placeholder mapeado a una columna inexistente es 400.
func (h *RuleTemplateHandler) Instantiate(c *fiber.Ctx) error {
	var req InstantiateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	tpl, ok := services.FindRuleTemplate(req.TemplateID)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("template %q no existe en el catálogo", req.TemplateID)})
	}

	monitorID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}

	monitor, err := h.monitorRepo.FindByID(c.Context(), monitorID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor no encontrado"})
	}

	// Toda columna del fieldMap tiene que existir en el esquema del
	// monitor: el binding produce una regla que el engine ejecutará
	// contra esos datos; una columna inexistente sería una regla sorda.
	if unknown := unknownColumns(req.FieldMap, monitor.Schema); len(unknown) > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("columnas que no existen en el esquema del monitor: %s", strings.Join(unknown, ", ")),
		})
	}

	rule, err := services.BindTemplate(tpl, req.FieldMap, req.Params)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	userID, _ := primitive.ObjectIDFromHex(c.Locals("userId").(string))
	rule.MonitorID = monitorID
	rule.Name = req.Name
	rule.CreatedBy = userID
	rule.TemplateID = tpl.ID

	if err := h.ruleRepo.Create(c.Context(), &rule); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(rule)
}
