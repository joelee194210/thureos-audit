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
	_, _ = r.conversations.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "monitor_id", Value: 1}, {Key: "user_id", Value: 1}, {Key: "updated_at", Value: -1}},
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
	return &conv, nil
}

func (r *ChatRepository) ListConversationsByMonitorAndUser(ctx context.Context, monitorID, userID primitive.ObjectID) ([]models.ChatConversation, error) {
	cursor, err := r.conversations.Find(ctx,
		bson.M{"monitor_id": monitorID, "user_id": userID},
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
	return results, nil
}

// TouchConversation actualiza updated_at — se llama después de cada
// intercambio para que ListConversationsByMonitorAndUser ordene por
// actividad reciente, no por fecha de creación.
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
