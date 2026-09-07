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

type ScreeningRepository struct {
	col *mongo.Collection
}

func NewScreeningRepository(db *database.MongoDB) *ScreeningRepository {
	r := &ScreeningRepository{col: db.Collection("screening_results")}
	r.ensureIndexes()
	return r
}

func (r *ScreeningRepository) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _ = r.col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "red_flag_id", Value: 1}},
	})
	_, _ = r.col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "status", Value: 1}, {Key: "whitelist_expires_at", Value: 1}},
	})
}

func (r *ScreeningRepository) Create(ctx context.Context, result *models.ScreeningResult) error {
	result.CreatedAt = time.Now()
	res, err := r.col.InsertOne(ctx, result)
	if err != nil {
		return fmt.Errorf("creando screening result: %w", err)
	}
	result.ID = res.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *ScreeningRepository) FindByRedFlagID(ctx context.Context, redFlagID primitive.ObjectID) ([]models.ScreeningResult, error) {
	cursor, err := r.col.Find(ctx, bson.M{"red_flag_id": redFlagID},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("buscando screening results: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var results []models.ScreeningResult
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("decodificando screening results: %w", err)
	}
	return results, nil
}

func (r *ScreeningRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.ScreeningResult, error) {
	var result models.ScreeningResult
	if err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&result); err != nil {
		return nil, fmt.Errorf("screening result %s no encontrado: %w", id.Hex(), err)
	}
	return &result, nil
}

// Dismiss descarta un match/review con nota obligatoria y fija el
// vencimiento del whitelist a models.ScreeningWhitelistDuration — el
// screening_reeval_job lo vuelve a evaluar cuando venza.
func (r *ScreeningRepository) Dismiss(ctx context.Context, id primitive.ObjectID, status models.ScreeningStatus, reviewedBy primitive.ObjectID, notes string) error {
	now := time.Now()
	expiresAt := now.Add(models.ScreeningWhitelistDuration)
	_, err := r.col.UpdateByID(ctx, id, bson.M{"$set": bson.M{
		"status":               status,
		"reviewed_by":          reviewedBy,
		"reviewed_at":          now,
		"review_notes":         notes,
		"whitelist_expires_at": expiresAt,
	}})
	if err != nil {
		return fmt.Errorf("descartando screening result %s: %w", id.Hex(), err)
	}
	return nil
}

// FindExpiredWhitelist devuelve los descartes vencidos — lo que
// screening_reeval_job necesita reevaluar en cada tick.
func (r *ScreeningRepository) FindExpiredWhitelist(ctx context.Context) ([]models.ScreeningResult, error) {
	filter := bson.M{
		"status":               bson.M{"$in": bson.A{models.ScreeningDismissed, models.ScreeningFalsePositive}},
		"whitelist_expires_at": bson.M{"$lt": time.Now()},
	}
	cursor, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("buscando whitelist vencido: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var results []models.ScreeningResult
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("decodificando whitelist vencido: %w", err)
	}
	return results, nil
}

// UpdateAfterReeval reemplaza status/matches tras reevaluar un descarte
// vencido — limpia whitelist_expires_at (ya no es un descarte activo,
// vuelve a ser lo que Watchman diga hoy).
func (r *ScreeningRepository) UpdateAfterReeval(ctx context.Context, id primitive.ObjectID, status models.ScreeningStatus, strongestMatch string, matches []models.NormalizedMatch) error {
	_, err := r.col.UpdateByID(ctx, id, bson.M{
		"$set": bson.M{
			"status":          status,
			"strongest_match": strongestMatch,
			"matches":         matches,
		},
		"$unset": bson.M{"whitelist_expires_at": ""},
	})
	if err != nil {
		return fmt.Errorf("actualizando screening result %s tras reevaluación: %w", id.Hex(), err)
	}
	return nil
}
