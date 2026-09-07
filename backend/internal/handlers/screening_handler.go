package handlers

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
	"github.com/thureos/compliance/internal/services"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ScreeningHandler struct {
	screeningService *services.ScreeningService
	screeningRepo    *repository.ScreeningRepository
	activityRepo     *repository.ActivityLogRepository
}

func NewScreeningHandler(
	screeningService *services.ScreeningService,
	screeningRepo *repository.ScreeningRepository,
	activityRepo *repository.ActivityLogRepository,
) *ScreeningHandler {
	return &ScreeningHandler{
		screeningService: screeningService,
		screeningRepo:    screeningRepo,
		activityRepo:     activityRepo,
	}
}

type searchScreeningRequest struct {
	Name        string `json:"name"`
	DateOfBirth string `json:"dateOfBirth,omitempty"`
}

// Search es una búsqueda manual ad-hoc: no persiste nada, misma lógica
// que el disparo automático (mismo clasificador) pero sin caso asociado.
func (h *ScreeningHandler) Search(c *fiber.Ctx) error {
	var req searchScreeningRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name es obligatorio"})
	}

	result, err := h.screeningService.ScreenAndPersistEphemeral(c.Context(), req.Name, req.DateOfBirth)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

// GetByRedFlag devuelve los screenings linkeados a un caso.
func (h *ScreeningHandler) GetByRedFlag(c *fiber.Ctx) error {
	redFlagID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid red flag ID"})
	}
	results, err := h.screeningRepo.FindByRedFlagID(c.Context(), redFlagID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if results == nil {
		results = []models.ScreeningResult{}
	}
	return c.JSON(results)
}

type dismissScreeningRequest struct {
	Status models.ScreeningStatus `json:"status"` // dismissed | false_positive
	Notes  string                 `json:"notes"`
}

// Dismiss descarta un match/review con nota obligatoria — mismo principio
// que cerrar un caso exige disposition (Tarea 8 del roadmap P0).
func (h *ScreeningHandler) Dismiss(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid screening result ID"})
	}
	var req dismissScreeningRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Status != models.ScreeningDismissed && req.Status != models.ScreeningFalsePositive {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "status debe ser dismissed o false_positive"})
	}
	if req.Notes == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "notes es obligatorio para descartar un screening"})
	}

	userID, err := caseUserID(c)
	if err != nil {
		return err
	}

	if err := h.screeningRepo.Dismiss(c.Context(), id, req.Status, userID, req.Notes); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if h.activityRepo != nil {
		userName, _ := c.Locals("email").(string)
		_ = h.activityRepo.Create(c.Context(), &models.ActivityLog{
			UserID:    userID,
			UserName:  userName,
			UserEmail: userName,
			Action:    models.ActivityScreeningDismiss,
			Detail:    fmt.Sprintf("Screening %s: descartado como %s — %s", id.Hex(), req.Status, req.Notes),
			Resource:  "screening_result",
			IP:        c.IP(),
		})
	}

	return c.JSON(fiber.Map{"message": "screening descartado"})
}
