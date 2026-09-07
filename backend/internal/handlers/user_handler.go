package handlers

import (
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
	"github.com/thureos/compliance/internal/services"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
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

func (h *UserHandler) Create(c *fiber.Ctx) error {
	var body struct {
		Email    string `json:"email"`
		Name     string `json:"name"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if body.Email == "" || body.Name == "" || body.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "email, name, and password are required"})
	}

	if !isValidRole(body.Role) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid role, must be admin, compliance, or viewer"})
	}

	if err := services.ValidatePasswordComplexity(body.Password); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if _, err := h.userRepo.FindByEmail(c.Context(), body.Email); err == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "ya existe un usuario con ese correo"})
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "hashing password"})
	}

	now := time.Now()
	user := &models.User{
		Email:             body.Email,
		Name:              body.Name,
		Password:          string(hashed),
		Role:              models.Role(body.Role),
		PasswordChangedAt: &now,
	}

	if err := h.userRepo.Create(c.Context(), user); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	h.logActivity(c, models.ActivityUserManage,
		fmt.Sprintf("Usuario %s creado con rol %s", user.Email, user.Role), user.ID.Hex())

	return c.Status(fiber.StatusCreated).JSON(user)
}

func isValidRole(role string) bool {
	switch models.Role(role) {
	case models.RoleAdmin, models.RoleCompliance, models.RoleViewer:
		return true
	}
	return false
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

	if !isValidRole(body.Role) {
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
