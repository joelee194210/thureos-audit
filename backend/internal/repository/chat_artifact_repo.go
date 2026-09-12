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

type ChatArtifactRepository struct {
	artifacts *mongo.Collection
}

func NewChatArtifactRepository(db *database.MongoDB) *ChatArtifactRepository {
	r := &ChatArtifactRepository{artifacts: db.Collection("chat_artifacts")}
	r.ensureIndexes()
	return r
}

func (r *ChatArtifactRepository) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _ = r.artifacts.Indexes().CreateMany(ctx, []mongo.IndexModel{
		// La biblioteca: los guardados del usuario, más recientes primero.
		{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "saved", Value: 1}, {Key: "created_at", Value: -1}}},
		// El borrado en cascada al eliminar una conversación.
		{Keys: bson.D{{Key: "conversation_id", Value: 1}}},
	})
}

func (r *ChatArtifactRepository) Create(ctx context.Context, art *models.ChatArtifact) error {
	art.CreatedAt = time.Now()
	res, err := r.artifacts.InsertOne(ctx, art)
	if err != nil {
		return fmt.Errorf("creando artefacto: %w", err)
	}
	art.ID = res.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *ChatArtifactRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.ChatArtifact, error) {
	var art models.ChatArtifact
	if err := r.artifacts.FindOne(ctx, bson.M{"_id": id}).Decode(&art); err != nil {
		return nil, fmt.Errorf("artefacto %s no encontrado: %w", id.Hex(), err)
	}
	return &art, nil
}

// ListSavedByUser devuelve la biblioteca del usuario. Solo los guardados:
// los efímeros viven en su mensaje y no tienen por qué ensuciar la lista.
func (r *ChatArtifactRepository) ListSavedByUser(ctx context.Context, userID primitive.ObjectID) ([]models.ChatArtifact, error) {
	cursor, err := r.artifacts.Find(ctx,
		bson.M{"user_id": userID, "saved": true},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("listando artefactos: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	results := []models.ChatArtifact{}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("decodificando artefactos: %w", err)
	}
	return results, nil
}

func (r *ChatArtifactRepository) SetSaved(ctx context.Context, id primitive.ObjectID, saved bool, name string) error {
	_, err := r.artifacts.UpdateByID(ctx, id, bson.M{"$set": bson.M{"saved": saved, "saved_name": name}})
	if err != nil {
		return fmt.Errorf("guardando artefacto %s: %w", id.Hex(), err)
	}
	return nil
}

// SetCache escribe el resultado de la última ejecución. Lo llama SOLO el
// endpoint /run: exportar re-ejecuta pero no muta el artefacto, para que
// bajar un Excel no le cambie la fecha a quien lo esté mirando.
func (r *ChatArtifactRepository) SetCache(ctx context.Context, id primitive.ObjectID, data []map[string]interface{}, ranAt time.Time) error {
	_, err := r.artifacts.UpdateByID(ctx, id, bson.M{"$set": bson.M{"cached_data": data, "ran_at": ranAt}})
	if err != nil {
		return fmt.Errorf("cacheando artefacto %s: %w", id.Hex(), err)
	}
	return nil
}

func (r *ChatArtifactRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	if _, err := r.artifacts.DeleteOne(ctx, bson.M{"_id": id}); err != nil {
		return fmt.Errorf("borrando artefacto %s: %w", id.Hex(), err)
	}
	return nil
}

// DeleteUnsavedByConversation limpia los efímeros al borrar una
// conversación. Los guardados sobreviven: la biblioteca es del usuario,
// no de la conversación, y el valor de un artefacto guardado está en sus
// sources, no en el hilo que lo originó.
func (r *ChatArtifactRepository) DeleteUnsavedByConversation(ctx context.Context, conversationID primitive.ObjectID) error {
	_, err := r.artifacts.DeleteMany(ctx, bson.M{"conversation_id": conversationID, "saved": false})
	if err != nil {
		return fmt.Errorf("borrando artefactos de la conversación %s: %w", conversationID.Hex(), err)
	}
	return nil
}
