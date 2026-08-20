package repository

import (
	"context"
	"time"

	"github.com/thureos/compliance/internal/database"
	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type SystemConfigRepository struct {
	col *mongo.Collection
}

func NewSystemConfigRepository(db *database.MongoDB) *SystemConfigRepository {
	return &SystemConfigRepository{col: db.Collection("system_config")}
}

// Get returns the singleton system config, or a default if none exists.
func (r *SystemConfigRepository) Get(ctx context.Context) (*models.SystemConfig, error) {
	var cfg models.SystemConfig
	err := r.col.FindOne(ctx, bson.M{}).Decode(&cfg)
	if err == mongo.ErrNoDocuments {
		return r.defaultConfig(), nil
	}
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Upsert creates or updates the singleton config document.
func (r *SystemConfigRepository) Upsert(ctx context.Context, cfg *models.SystemConfig) error {
	cfg.UpdatedAt = time.Now()

	opts := options.Update().SetUpsert(true)
	update := bson.M{
		"$set": bson.M{
			"ai":         cfg.AI,
			"updated_at": cfg.UpdatedAt,
			"updated_by": cfg.UpdatedBy,
		},
	}

	_, err := r.col.UpdateOne(ctx, bson.M{}, update, opts)
	return err
}

// SeedDefault inserts default config only if no document exists.
func (r *SystemConfigRepository) SeedDefault(ctx context.Context, anthropicKey string) error {
	count, err := r.col.CountDocuments(ctx, bson.M{})
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	cfg := r.defaultConfig()
	if anthropicKey != "" {
		cfg.AI.APIKey = anthropicKey
	}

	_, err = r.col.InsertOne(ctx, cfg)
	return err
}

func (r *SystemConfigRepository) defaultConfig() *models.SystemConfig {
	return &models.SystemConfig{
		AI: models.AIConfig{
			Provider: models.AIProviderAnthropic,
			Model:    "claude-sonnet-4-20250514",
		},
		UpdatedAt: time.Now(),
		UpdatedBy: "system",
	}
}
