package repository

import (
	"context"
	"time"

	"github.com/joelee/datawatch/internal/database"
	"github.com/joelee/datawatch/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type AlertLogRepository struct {
	col *mongo.Collection
}

func NewAlertLogRepository(db *database.MongoDB) *AlertLogRepository {
	col := db.Collection("alert_logs")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "alert_id", Value: 1},
			{Key: "created_at", Value: 1},
		},
	})

	return &AlertLogRepository{col: col}
}

func (r *AlertLogRepository) Create(ctx context.Context, log *models.AlertLog) error {
	log.CreatedAt = time.Now()
	result, err := r.col.InsertOne(ctx, log)
	if err != nil {
		return err
	}
	log.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *AlertLogRepository) FindByAlertID(ctx context.Context, alertID primitive.ObjectID) ([]models.AlertLog, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}})
	cursor, err := r.col.Find(ctx, bson.M{"alert_id": alertID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var logs []models.AlertLog
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, err
	}
	if logs == nil {
		logs = []models.AlertLog{}
	}
	return logs, nil
}
