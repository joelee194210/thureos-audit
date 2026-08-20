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

type UserHandler struct {
	userRepo     *repository.UserRepository
	activityRepo *repository.ActivityLogRepository
}

func NewUserHandler(userRepo *repository.UserRepository, activityRepo *repository.ActivityLogRepository) *UserHandler {
	return &UserHandler{userRepo: userRepo, activityRepo: activityRepo}
}

func (h *UserHandler) List(c *fiber.Ctx) error {
	users, err := h.userRepo.FindAll(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(users)
}

func (h *UserHandler) Get(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user ID"})
	}

	user, err := h.userRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	return c.JSON(user)
}

func (h *UserHandler) logActivity(c *fiber.Ctx, action models.ActivityType, detail, resourceID string) {
	if h.activityRepo == nil {
		return
	}
	userIDStr, _ := c.Locals("userId").(string)
	userEmail, _ := c.Locals("email").(string)
	userOID, _ := primitive.ObjectIDFromHex(userIDStr)
	actLog := &models.ActivityLog{
		UserID:     userOID,
		UserName:   userEmail,
		UserEmail:  userEmail,
		Action:     action,
		Detail:     detail,
		Resource:   "user",
		ResourceID: resourceID,
		IP:         c.IP(),
	}
	if err := h.activityRepo.Create(c.Context(), actLog); err != nil {
		log.Printf("WARNING: failed to log user activity: %v", err)
	}
}

func (h *UserHandler) UpdateRole(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user ID"})
	}

	var body struct {
		Role string `json:"role"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	validRoles := map[string]bool{"admin": true, "compliance": true, "viewer": true}
	if !validRoles[body.Role] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid role, must be admin, compliance, or viewer"})
	}

	target, err := h.userRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	if err := h.userRepo.Update(c.Context(), id, bson.M{"role": body.Role}); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	h.logActivity(c, models.ActivityUserManage,
		fmt.Sprintf("Rol de %s cambiado de %s a %s", target.Email, target.Role, body.Role),
		id.Hex())

	return c.JSON(fiber.Map{"message": "user role updated"})
}

func (h *UserHandler) ToggleActive(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user ID"})
	}

	user, err := h.userRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	if err := h.userRepo.Update(c.Context(), id, bson.M{"active": !user.Active}); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	action := "activado"
	if user.Active {
		action = "desactivado"
	}
	h.logActivity(c, models.ActivityUserManage,
		fmt.Sprintf("Usuario %s %s", user.Email, action),
		id.Hex())

	return c.JSON(fiber.Map{"message": "user status toggled", "active": !user.Active})
}

func (h *UserHandler) Delete(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user ID"})
	}

	// Prevent self-deletion — an admin cannot delete their own account
	currentUserID, _ := c.Locals("userId").(string)
	if currentUserID == id.Hex() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cannot delete your own account"})
	}

	target, err := h.userRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	// Soft-delete preserves the record for regulatory retention
	if err := h.userRepo.Delete(c.Context(), id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	h.logActivity(c, models.ActivityUserManage,
		fmt.Sprintf("Usuario %s eliminado (soft-delete)", target.Email),
		id.Hex())

	return c.JSON(fiber.Map{"message": "user deleted"})
}
