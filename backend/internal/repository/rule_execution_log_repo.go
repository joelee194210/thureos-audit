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

type RuleExecutionLogRepository struct {
	col *mongo.Collection
}

func NewRuleExecutionLogRepository(db *database.MongoDB) *RuleExecutionLogRepository {
	col := db.Collection("rule_execution_logs")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "rule_id", Value: 1}, {Key: "executed_at", Value: -1}}},
		{Keys: bson.D{{Key: "executed_at", Value: -1}}},
	})

	return &RuleExecutionLogRepository{col: col}
}

func (r *RuleExecutionLogRepository) Create(ctx context.Context, log *models.RuleExecutionLog) error {
	log.ExecutedAt = time.Now()
	result, err := r.col.InsertOne(ctx, log)
	if err != nil {
		return err
	}
	log.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *RuleExecutionLogRepository) FindByRule(ctx context.Context, ruleID primitive.ObjectID, limit int64) ([]models.RuleExecutionLog, error) {
	opts := options.Find().SetSort(bson.D{{Key: "executed_at", Value: -1}}).SetLimit(limit)
	cursor, err := r.col.Find(ctx, bson.M{"rule_id": ruleID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var logs []models.RuleExecutionLog
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, err
	}
	if logs == nil {
		logs = []models.RuleExecutionLog{}
	}
	return logs, nil
}

func (r *RuleExecutionLogRepository) FindRecent(ctx context.Context, limit int64) ([]models.RuleExecutionLog, error) {
	opts := options.Find().SetSort(bson.D{{Key: "executed_at", Value: -1}}).SetLimit(limit)
	cursor, err := r.col.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var logs []models.RuleExecutionLog
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, err
	}
	if logs == nil {
		logs = []models.RuleExecutionLog{}
	}
	return logs, nil
}
