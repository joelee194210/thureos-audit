package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/thureos/compliance/internal/database"
	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ChatRepository struct {
	conversations *mongo.Collection
	messages      *mongo.Collection
}

func NewChatRepository(db *database.MongoDB) *ChatRepository {
	r := &ChatRepository{
		conversations: db.Collection("chat_conversations"),
		messages:      db.Collection("chat_messages"),
	}
	r.ensureIndexes()
	return r
}

func (r *ChatRepository) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// {user_id, updated_at}: la consulta principal del sidebar, que ya no
	// se scopea por monitor. {user_id, monitor_ids}: el filtro opcional
	// "solo las que incluyen X" — Mongo indexa arrays elemento a elemento,
	// así que un multikey sobre monitor_ids alcanza.
	_, _ = r.conversations.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "updated_at", Value: -1}}},
		{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "monitor_ids", Value: 1}}},
	})
	_, _ = r.messages.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "conversation_id", Value: 1}, {Key: "created_at", Value: 1}},
	})
}

func (r *ChatRepository) CreateConversation(ctx context.Context, conv *models.ChatConversation) error {
	conv.CreatedAt = time.Now()
	conv.UpdatedAt = conv.CreatedAt
	res, err := r.conversations.InsertOne(ctx, conv)
	if err != nil {
		return fmt.Errorf("creando conversación: %w", err)
	}
	conv.ID = res.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *ChatRepository) GetConversation(ctx context.Context, id primitive.ObjectID) (*models.ChatConversation, error) {
	var conv models.ChatConversation
	if err := r.conversations.FindOne(ctx, bson.M{"_id": id}).Decode(&conv); err != nil {
		return nil, fmt.Errorf("conversación %s no encontrada: %w", id.Hex(), err)
	}
	conv.Normalize()
	return &conv, nil
}

// ListConversationsByUser devuelve las conversaciones del usuario
// ordenadas por actividad. monitorFilter es opcional: si viene, acota a
// las conversaciones que incluyen ese monitor.
//
// A diferencia de la versión anterior, el scoping por monitor dejó de ser
// obligatorio: una conversación que abarca varios monitores no pertenece
// a la lista de ninguno en particular.
func (r *ChatRepository) ListConversationsByUser(
	ctx context.Context,
	userID primitive.ObjectID,
	monitorFilter *primitive.ObjectID,
) ([]models.ChatConversation, error) {
	filter := bson.M{"user_id": userID}
	if monitorFilter != nil {
		// Matchea tanto monitor_ids (array) como el monitor_id escalar de
		// documentos legacy: en Mongo, igualdad contra un campo array
		// matchea si algún elemento coincide.
		filter["$or"] = []bson.M{
			{"monitor_ids": *monitorFilter},
			{"monitor_id": *monitorFilter},
		}
	}
	cursor, err := r.conversations.Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("listando conversaciones: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	results := []models.ChatConversation{}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("decodificando conversaciones: %w", err)
	}
	for i := range results {
		results[i].Normalize()
	}
	return results, nil
}

// AddMonitor agrega un monitor a la conversación sin duplicarlo.
// $addToSet sobre monitor_ids es idempotente: repetir la llamada con el
// mismo monitor no cambia nada.
func (r *ChatRepository) AddMonitor(ctx context.Context, id, monitorID primitive.ObjectID) error {
	_, err := r.conversations.UpdateByID(ctx, id, bson.M{"$addToSet": bson.M{"monitor_ids": monitorID}})
	if err != nil {
		return fmt.Errorf("agregando monitor a la conversación %s: %w", id.Hex(), err)
	}
	return nil
}

// TouchConversation actualiza updated_at — se llama después de cada
// intercambio para que ListConversationsByUser ordene por actividad
// reciente, no por fecha de creación.
func (r *ChatRepository) TouchConversation(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.conversations.UpdateByID(ctx, id, bson.M{"$set": bson.M{"updated_at": time.Now()}})
	if err != nil {
		return fmt.Errorf("actualizando conversación %s: %w", id.Hex(), err)
	}
	return nil
}

// SetTitle reemplaza el título — chat_service.go la llama una sola vez,
// después de la primera pregunta de la conversación (ver Ask en la Tarea 5).
func (r *ChatRepository) SetTitle(ctx context.Context, id primitive.ObjectID, title string) error {
	_, err := r.conversations.UpdateByID(ctx, id, bson.M{"$set": bson.M{"title": title}})
	if err != nil {
		return fmt.Errorf("renombrando conversación %s: %w", id.Hex(), err)
	}
	return nil
}

func (r *ChatRepository) CreateMessage(ctx context.Context, msg *models.ChatMessage) error {
	msg.CreatedAt = time.Now()
	res, err := r.messages.InsertOne(ctx, msg)
	if err != nil {
		return fmt.Errorf("creando mensaje: %w", err)
	}
	msg.ID = res.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *ChatRepository) ListMessagesByConversation(ctx context.Context, conversationID primitive.ObjectID) ([]models.ChatMessage, error) {
	cursor, err := r.messages.Find(ctx,
		bson.M{"conversation_id": conversationID},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("listando mensajes: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	results := []models.ChatMessage{}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("decodificando mensajes: %w", err)
	}
	return results, nil
}

// DeleteConversation borra la conversación y todos sus mensajes. El
// llamador es responsable de verificar ownership antes de invocar esto
// (mismo criterio que el resto del repositorio — ver chat_handler.go).
func (r *ChatRepository) DeleteConversation(ctx context.Context, id primitive.ObjectID) error {
	if _, err := r.messages.DeleteMany(ctx, bson.M{"conversation_id": id}); err != nil {
		return fmt.Errorf("borrando mensajes de la conversación %s: %w", id.Hex(), err)
	}
	if _, err := r.conversations.DeleteOne(ctx, bson.M{"_id": id}); err != nil {
		return fmt.Errorf("borrando conversación %s: %w", id.Hex(), err)
	}
	return nil
}
