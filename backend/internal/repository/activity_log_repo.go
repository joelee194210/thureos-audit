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

type ActivityLogRepository struct {
	col *mongo.Collection
}

func NewActivityLogRepository(db *database.MongoDB) *ActivityLogRepository {
	col := db.Collection("activity_logs")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "created_at", Value: -1}}},
		{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}}},
	})

	return &ActivityLogRepository{col: col}
}

func (r *ActivityLogRepository) Create(ctx context.Context, log *models.ActivityLog) error {
	log.CreatedAt = time.Now()
	result, err := r.col.InsertOne(ctx, log)
	if err != nil {
		return err
	}
	log.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *ActivityLogRepository) FindRecent(ctx context.Context, limit int64) ([]models.ActivityLog, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(limit)
	cursor, err := r.col.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var logs []models.ActivityLog
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, err
	}
	if logs == nil {
		logs = []models.ActivityLog{}
	}
	return logs, nil
}

func (r *ActivityLogRepository) FindFiltered(ctx context.Context, filter bson.M, limit int64) ([]models.ActivityLog, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(limit)
	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var logs []models.ActivityLog
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, err
	}
	if logs == nil {
		logs = []models.ActivityLog{}
	}
	return logs, nil
}
