package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
	"github.com/thureos/compliance/internal/services"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ChatHandler struct {
	chatRepo    *repository.ChatRepository
	chatService *services.ChatService
}

func NewChatHandler(chatRepo *repository.ChatRepository, chatService *services.ChatService) *ChatHandler {
	return &ChatHandler{chatRepo: chatRepo, chatService: chatService}
}

// CreateConversation arranca una conversación nueva sobre un monitor —
// privada del usuario autenticado (ver Global Constraints del plan).
func (h *ChatHandler) CreateConversation(c *fiber.Ctx) error {
	var body struct {
		MonitorID string `json:"monitorId"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	monitorID, err := primitive.ObjectIDFromHex(body.MonitorID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "monitorId inválido"})
	}
	userID, err := primitive.ObjectIDFromHex(c.Locals("userId").(string))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "usuario inválido"})
	}

	conv := models.ChatConversation{MonitorID: monitorID, UserID: userID, Title: "Nueva conversación"}
	if err := h.chatRepo.CreateConversation(c.Context(), &conv); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(conv)
}

// ListConversations devuelve solo las conversaciones del usuario
// autenticado para el monitor pedido — nunca las de otro usuario.
func (h *ChatHandler) ListConversations(c *fiber.Ctx) error {
	monitorID, err := primitive.ObjectIDFromHex(c.Query("monitorId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "monitorId inválido"})
	}
	userID, err := primitive.ObjectIDFromHex(c.Locals("userId").(string))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "usuario inválido"})
	}

	convs, err := h.chatRepo.ListConversationsByMonitorAndUser(c.Context(), monitorID, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(convs)
}

func (h *ChatHandler) ListMessages(c *fiber.Ctx) error {
	convID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	userID, err := primitive.ObjectIDFromHex(c.Locals("userId").(string))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "usuario inválido"})
	}

	conv, err := h.chatRepo.GetConversation(c.Context(), convID)
	if err != nil || conv.UserID != userID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "conversación no encontrada"})
	}

	msgs, err := h.chatRepo.ListMessagesByConversation(c.Context(), convID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(msgs)
}

func (h *ChatHandler) DeleteConversation(c *fiber.Ctx) error {
	convID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	userID, err := primitive.ObjectIDFromHex(c.Locals("userId").(string))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "usuario inválido"})
	}

	conv, err := h.chatRepo.GetConversation(c.Context(), convID)
	if err != nil || conv.UserID != userID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "conversación no encontrada"})
	}

	if err := h.chatRepo.DeleteConversation(c.Context(), convID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "conversación borrada"})
}

func (h *ChatHandler) Ask(c *fiber.Ctx) error {
	convID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := c.BodyParser(&body); err != nil || body.Content == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "content es obligatorio"})
	}
	userID, err := primitive.ObjectIDFromHex(c.Locals("userId").(string))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "usuario inválido"})
	}

	msg, err := h.chatService.Ask(c.Context(), convID, userID, body.Content)
	if err != nil {
		// ChatService.Ask usa el mismo sentinel para "no es tuya" y "no
		// existe" — nunca distingue cuál de las dos, ni filtra detalle
		// interno en el body. ErrMonitorNotFound cubre el mismo caso para
		// un monitor borrado mientras la conversación seguía apuntando a
		// él — nunca se filtra el error crudo de Mongo en la respuesta.
		if errors.Is(err, services.ErrConversationNotFound) || errors.Is(err, services.ErrMonitorNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(msg)
}
