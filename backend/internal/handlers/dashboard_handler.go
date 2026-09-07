package handlers

import (
	"fmt"
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

// userContext reads the authenticated caller's ID and role from the JWT
// claims middleware.AuthRequired sets in c.Locals.
func userContext(c *fiber.Ctx) (primitive.ObjectID, string) {
	// Both reads use the comma-ok form: a bare .(string) assertion would
	// panic instead of denying access if the claim were ever missing.
	hexID, _ := c.Locals("userId").(string)
	userID, _ := primitive.ObjectIDFromHex(hexID)
	role, _ := c.Locals("role").(string)
	return userID, role
}

// isOrgRole marks the roles que ven la plataforma completa. CLAUDE.md define
// compliance como un rol de organización ("Create/edit monitors, rules,
// dashboards"), igual que en monitores y reglas: sin esto, un usuario
// compliance recibiría 404 en cualquier dashboard que no haya creado él mismo.
// El rol viewer queda fuera y depende de IsPublic/OwnerID/SharedWith.
func isOrgRole(role string) bool {
	return role == string(models.RoleAdmin) || role == string(models.RoleCompliance)
}

// canReadDashboard mirrors DashboardRepository.FindAccessible's rule —
// Get/GetData/DrillDown fetch by ID directly and previously skipped this
// check entirely: any authenticated user who knew or guessed a dashboard's
// ID could read a private dashboard's full widget data even though it
// never appeared in their list.
func canReadDashboard(d *models.Dashboard, userID primitive.ObjectID, role string) bool {
	if isOrgRole(role) || d.IsPublic || d.OwnerID == userID {
		return true
	}
	for _, id := range d.SharedWith {
		if id == userID {
			return true
		}
	}
	return false
}

// canWriteDashboard gates mutation (AddWidget/DeleteWidget/Delete) to the
// owner, admins, and users the dashboard was explicitly shared with — a
// public dashboard is readable by anyone but not editable by anyone. Unlike
// monitors/rules (org-wide resources with no per-item ownership), dashboards
// have an explicit sharing model, so RequireComplianceOrAbove alone isn't
// enough: it only proves the caller's role, not their relationship to this
// specific dashboard.
func canWriteDashboard(d *models.Dashboard, userID primitive.ObjectID, role string) bool {
	if isOrgRole(role) || d.OwnerID == userID {
		return true
	}
	for _, id := range d.SharedWith {
		if id == userID {
			return true
		}
	}
	return false
}

// validateWidgetAggregation rejects what dashboard_service.buildAggExpression
// would otherwise accept silently: an unrecognized Aggregation falls through
// to "count" (AggDistinct is defined in the model but never implemented in
// the pipeline), and sum/avg/min/max with an empty Field become an empty
// Mongo field path — both previously surfaced only as a broken widget after
// creation, not as a rejected request.
func validateWidgetAggregation(w models.Widget) error {
	switch w.Aggregation {
	case models.AggCount, models.AggSum, models.AggAvg, models.AggMin, models.AggMax:
	default:
		return fmt.Errorf("agregación no soportada: %q", w.Aggregation)
	}
	if w.Aggregation != models.AggCount && w.Field == "" {
		return fmt.Errorf("el campo es obligatorio para la agregación %q", w.Aggregation)
	}
	return nil
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
	userID, role := userContext(c)

	dashboards, err := h.dashboardRepo.FindAccessible(c.Context(), userID, isOrgRole(role))
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

	userID, role := userContext(c)
	if !canReadDashboard(dashboard, userID, role) {
		// Same 404 a caller gets for a nonexistent ID — a 403 would confirm
		// the dashboard exists, leaking its presence to someone with no
		// access to it.
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

	if err := validateWidgetAggregation(widget); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	dashboard, err := h.dashboardRepo.FindByID(c.Context(), dashboardID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "dashboard not found"})
	}

	userID, role := userContext(c)
	if !canWriteDashboard(dashboard, userID, role) {
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

	userID, role := userContext(c)
	if !canWriteDashboard(dashboard, userID, role) {
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
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid dashboard ID"})
	}

	dashboard, err := h.dashboardRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "dashboard not found"})
	}

	userID, role := userContext(c)
	if !canReadDashboard(dashboard, userID, role) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "dashboard not found"})
	}

	data, err := h.dashboardService.GetDashboardData(c.Context(), c.Params("id"))
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

	dashboard, err := h.dashboardRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "dashboard not found"})
	}

	userID, role := userContext(c)
	if !canWriteDashboard(dashboard, userID, role) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "dashboard not found"})
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

	dashOID, err := primitive.ObjectIDFromHex(dashboardID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid dashboard ID"})
	}
	dashboard, err := h.dashboardRepo.FindByID(c.Context(), dashOID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "dashboard not found"})
	}
	userID, role := userContext(c)
	if !canReadDashboard(dashboard, userID, role) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "dashboard not found"})
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
