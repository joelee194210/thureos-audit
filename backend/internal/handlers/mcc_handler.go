package handlers

import (
	"fmt"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/joelee/datawatch/internal/models"
	"github.com/joelee/datawatch/internal/repository"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type MCCHandler struct {
	mccRepo      *repository.MCCRepository
	activityRepo *repository.ActivityLogRepository
}

func NewMCCHandler(mccRepo *repository.MCCRepository, activityRepo *repository.ActivityLogRepository) *MCCHandler {
	return &MCCHandler{mccRepo: mccRepo, activityRepo: activityRepo}
}

func (h *MCCHandler) List(c *fiber.Ctx) error {
	mccs, err := h.mccRepo.FindAll(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(mccs)
}

func (h *MCCHandler) Search(c *fiber.Ctx) error {
	q := c.Query("q")
	if q == "" {
		return h.List(c)
	}
	mccs, err := h.mccRepo.Search(c.Context(), q)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(mccs)
}

func (h *MCCHandler) Categories(c *fiber.Ctx) error {
	cats, err := h.mccRepo.GetCategories(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(cats)
}

func (h *MCCHandler) ByCategory(c *fiber.Ctx) error {
	cat := c.Params("category")
	mccs, err := h.mccRepo.FindByCategory(c.Context(), cat)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(mccs)
}

func (h *MCCHandler) Update(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid MCC ID"})
	}

	var body struct {
		RiskLevel   string   `json:"riskLevel"`
		Networks    []string `json:"networks"`
		Description string   `json:"description"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	update := bson.M{}
	if body.RiskLevel != "" {
		validLevels := map[string]bool{"high": true, "medium": true, "low": true}
		if !validLevels[body.RiskLevel] {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid risk level"})
		}
		update["risk_level"] = body.RiskLevel
	}
	if body.Networks != nil {
		update["networks"] = body.Networks
	}
	if body.Description != "" {
		update["description"] = body.Description
	}

	if len(update) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no fields to update"})
	}

	if err := h.mccRepo.Update(c.Context(), id, update); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Log MCC change in activity log for audit trail
	if h.activityRepo != nil {
		userIDStr, _ := c.Locals("userId").(string)
		userEmail, _ := c.Locals("email").(string)
		userOID, _ := primitive.ObjectIDFromHex(userIDStr)
		detail := fmt.Sprintf("MCC %s actualizado:", c.Params("id"))
		if body.RiskLevel != "" {
			detail += fmt.Sprintf(" riskLevel=%s", body.RiskLevel)
		}
		if body.Networks != nil {
			detail += fmt.Sprintf(" networks=%v", body.Networks)
		}
		actLog := &models.ActivityLog{
			UserID:     userOID,
			UserName:   userEmail,
			UserEmail:  userEmail,
			Action:     models.ActivityMCCUpdate,
			Detail:     detail,
			Resource:   "mcc",
			ResourceID: c.Params("id"),
			IP:         c.IP(),
		}
		if err := h.activityRepo.Create(c.Context(), actLog); err != nil {
			log.Printf("WARNING: failed to log MCC update activity: %v", err)
		}
	}

	return c.JSON(fiber.Map{"message": "MCC updated"})
}

func (h *MCCHandler) ByRiskLevel(c *fiber.Ctx) error {
	level := c.Params("level")
	mccs, err := h.mccRepo.FindByRiskLevel(c.Context(), level)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(mccs)
}
