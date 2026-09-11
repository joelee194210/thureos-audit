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

// dedupeMonitorIDs parsea cada hex de monitorIds y descarta repetidos,
// conservando el orden de primera aparición. Separada de CreateConversation
// para poder fijar con un test el caso borde de ids repetidos (ver el
// comentario sobre buildMonitorAliases más abajo) sin levantar Mongo.
func dedupeMonitorIDs(raw []string) ([]primitive.ObjectID, error) {
	seen := make(map[primitive.ObjectID]bool, len(raw))
	ids := make([]primitive.ObjectID, 0, len(raw))
	for _, r := range raw {
		id, err := primitive.ObjectIDFromHex(r)
		if err != nil {
			return nil, err
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids, nil
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
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cuerpo de la petición inválido"})
	}
	if len(body.MonitorIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "hay que elegir al menos un monitor"})
	}

	// Deduplicar preservando el orden de entrada, ANTES de chequear el
	// tope: Max acota el costo del prompt por monitor DISTINTO (cada uno
	// agrega un bloque de schema al system prompt), no por entradas
	// repetidas del mismo id. Sin este dedupe, buildMonitorAliases le
	// asignaría un alias distinto a cada copia del mismo monitor (p.ej.
	// "transacciones" y "transacciones_2"), y el LLM lo leería como dos
	// fuentes independientes que "se corroboran" entre sí — el mismo dato
	// contando doble. El orden de primera aparición se conserva porque
	// buildMonitorAliases lo usa para decidir qué monitor se queda con el
	// alias base cuando dos nombres colisionan; un orden inestable
	// haría inestables los alias.
	monitorIDs, err := dedupeMonitorIDs(body.MonitorIDs)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "monitorId inválido"})
	}

	if len(monitorIDs) > models.MaxMonitorsPerConversation {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("una conversación puede abarcar hasta %d monitores", models.MaxMonitorsPerConversation),
		})
	}

	for _, id := range monitorIDs {
		if _, err := h.monitorRepo.FindByID(c.Context(), id); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "monitor no encontrado"})
		}
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
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cuerpo de la petición inválido"})
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

	// El conjunto persistido se toma de la respuesta del repositorio, no
	// se recompone acá: es la única forma de que el 200 no pueda contradecir
	// lo que quedó en la base (ver AddMonitor en chat_repo.go).
	updated, err := h.chatRepo.AddMonitor(c.Context(), convID, conv.MonitorIDs, monitorID)
	if err != nil {
		// Alguien más cambió el conjunto entre la lectura de arriba y la
		// escritura: no se reintenta a ciegas porque el tope se validó
		// contra el estado viejo. El cliente reintenta con datos frescos.
		if errors.Is(err, repository.ErrConversationModified) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "la conversación cambió mientras se agregaba el monitor; hay que reintentar",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	conv.MonitorIDs = updated
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
		// él: ese caso tampoco filtra nada, el sentinel no lleva detalle.
		// Ojo: el 500 de abajo SÍ puede llevar texto crudo del driver (el
		// "cargando monitor %s: %w" de chat_service.go, y el "cargando
		// historial: %w" de al lado). Es deliberado: un fallo de Mongo que
		// no sea ErrNoDocuments aborta en vez de responder una comparación
		// incompleta, y el detalle viaja igual que en el resto de los 500
		// de este handler.
		if errors.Is(err, services.ErrConversationNotFound) || errors.Is(err, services.ErrMonitorNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(msg)
}
