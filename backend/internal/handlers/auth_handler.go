package handlers

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/joelee/datawatch/internal/models"
	"github.com/joelee/datawatch/internal/repository"
	"github.com/joelee/datawatch/internal/services"
)

type AuthHandler struct {
	authService  *services.AuthService
	activityRepo *repository.ActivityLogRepository
}

func NewAuthHandler(authService *services.AuthService, activityRepo *repository.ActivityLogRepository) *AuthHandler {
	return &AuthHandler{authService: authService, activityRepo: activityRepo}
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req models.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if req.Email == "" || req.Password == "" || req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "email, name, and password are required"})
	}

	resp, err := h.authService.Register(c.Context(), req)
	if err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(resp)
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req models.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	resp, err := h.authService.Login(c.Context(), req)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	// Log login activity
	if h.activityRepo != nil {
		actLog := &models.ActivityLog{
			UserID:    resp.User.ID,
			UserName:  resp.User.Name,
			UserEmail: resp.User.Email,
			Action:    models.ActivityLogin,
			Detail:    "Inicio de sesión",
			Resource:  "session",
			IP:        c.IP(),
		}
		if err := h.activityRepo.Create(c.Context(), actLog); err != nil {
			log.Printf("WARNING: failed to log login activity: %v", err)
		}
	}

	return c.JSON(resp)
}

func (h *AuthHandler) Me(c *fiber.Ctx) error {
	userID, _ := c.Locals("userId").(string)
	email, _ := c.Locals("email").(string)
	role, _ := c.Locals("role").(string)
	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	return c.JSON(fiber.Map{
		"id":    userID,
		"email": email,
		"role":  role,
	})
}
