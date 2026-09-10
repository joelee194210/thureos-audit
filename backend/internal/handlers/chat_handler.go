package handlers

import (
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
	"github.com/thureos/compliance/internal/services"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ChatHandler struct {
	chatRepo    *repository.ChatRepository
	monitorRepo *repository.MonitorRepository
	chatService *services.ChatService
}

func NewChatHandler(chatRepo *repository.ChatRepository, monitorRepo *repository.MonitorRepository, chatService *services.ChatService) *ChatHandler {
	return &ChatHandler{chatRepo: chatRepo, monitorRepo: monitorRepo, chatService: chatService}
}

// CreateConversation arranca una conversación nueva sobre uno o varios
// monitores — privada del usuario autenticado (ver Global Constraints).
// El conjunto de monitores fijado acá es la allowlist que después usa
// executeQuery: por eso se valida que cada uno exista antes de guardar.
func (h *ChatHandler) CreateConversation(c *fiber.Ctx) error {
	var body struct {
		MonitorIDs []string `json:"monitorIds"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if len(body.MonitorIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "hay que elegir al menos un monitor"})
	}
	if len(body.MonitorIDs) > models.MaxMonitorsPerConversation {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("una conversación puede abarcar hasta %d monitores", models.MaxMonitorsPerConversation),
		})
	}

	monitorIDs := make([]primitive.ObjectID, 0, len(body.MonitorIDs))
	for _, raw := range body.MonitorIDs {
		id, err := primitive.ObjectIDFromHex(raw)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "monitorId inválido"})
		}
		if _, err := h.monitorRepo.FindByID(c.Context(), id); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "monitor no encontrado"})
		}
		monitorIDs = append(monitorIDs, id)
	}

	userID, err := primitive.ObjectIDFromHex(c.Locals("userId").(string))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "usuario inválido"})
	}

	conv := models.ChatConversation{MonitorIDs: monitorIDs, UserID: userID, Title: "Nueva conversación"}
	if err := h.chatRepo.CreateConversation(c.Context(), &conv); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(conv)
}

// ListConversations devuelve las conversaciones del usuario autenticado,
// ordenadas por actividad. monitorId es un filtro OPCIONAL: una
// conversación que abarca varios monitores no pertenece a la lista de
// ninguno en particular, así que el scoping principal es por usuario.
func (h *ChatHandler) ListConversations(c *fiber.Ctx) error {
	userID, err := primitive.ObjectIDFromHex(c.Locals("userId").(string))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "usuario inválido"})
	}

	var monitorFilter *primitive.ObjectID
	if raw := c.Query("monitorId"); raw != "" {
		id, err := primitive.ObjectIDFromHex(raw)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "monitorId inválido"})
		}
		monitorFilter = &id
	}

	convs, err := h.chatRepo.ListConversationsByUser(c.Context(), userID, monitorFilter)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(convs)
}

// AddMonitor suma un monitor a una conversación existente. No hay
// operación inversa a propósito: el historial ya referenció ese monitor y
// sus datos, y quitarlo dejaría mensajes previos hablando de algo que el
// asistente ya no puede ver (ver el spec, "Fuera de alcance").
func (h *ChatHandler) AddMonitor(c *fiber.Ctx) error {
	convID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
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

	conv, err := h.chatRepo.GetConversation(c.Context(), convID)
	if err != nil || conv.UserID != userID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "conversación no encontrada"})
	}
	for _, existing := range conv.MonitorIDs {
		if existing == monitorID {
			return c.JSON(conv) // idempotente: ya estaba
		}
	}
	if len(conv.MonitorIDs) >= models.MaxMonitorsPerConversation {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("una conversación puede abarcar hasta %d monitores", models.MaxMonitorsPerConversation),
		})
	}
	if _, err := h.monitorRepo.FindByID(c.Context(), monitorID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "monitor no encontrado"})
	}

	if err := h.chatRepo.AddMonitor(c.Context(), convID, monitorID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	conv.MonitorIDs = append(conv.MonitorIDs, monitorID)
	return c.JSON(conv)
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
