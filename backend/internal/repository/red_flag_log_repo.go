package repository

import (
	"context"
	"time"

	"github.com/thureos/compliance/internal/database"
	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type RedFlagLogRepository struct {
	col *mongo.Collection
}

func NewRedFlagLogRepository(db *database.MongoDB) *RedFlagLogRepository {
	col := db.Collection("red_flag_logs")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "red_flag_id", Value: 1},
			{Key: "created_at", Value: 1},
		},
	})

	return &RedFlagLogRepository{col: col}
}

func (r *RedFlagLogRepository) Create(ctx context.Context, log *models.RedFlagLog) error {
	log.CreatedAt = time.Now()
	result, err := r.col.InsertOne(ctx, log)
	if err != nil {
		return err
	}
	log.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *RedFlagLogRepository) FindByRedFlagID(ctx context.Context, redFlagID primitive.ObjectID) ([]models.RedFlagLog, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}})
	cursor, err := r.col.Find(ctx, bson.M{"red_flag_id": redFlagID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var logs []models.RedFlagLog
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, err
	}
	if logs == nil {
		logs = []models.RedFlagLog{}
	}
	return logs, nil
}
