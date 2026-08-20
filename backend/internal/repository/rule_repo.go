package repository

import (
	"context"
	"time"

	"github.com/thureos/compliance/internal/database"
	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type RuleRepository struct {
	col *mongo.Collection
}

func NewRuleRepository(db *database.MongoDB) *RuleRepository {
	return &RuleRepository{col: db.Collection("rules")}
}

func (r *RuleRepository) Create(ctx context.Context, rule *models.Rule) error {
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	rule.Active = true

	result, err := r.col.InsertOne(ctx, rule)
	if err != nil {
		return err
	}
	rule.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *RuleRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.Rule, error) {
	var rule models.Rule
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&rule)
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *RuleRepository) FindByMonitor(ctx context.Context, monitorID primitive.ObjectID) ([]models.Rule, error) {
	cursor, err := r.col.Find(ctx, bson.M{"monitor_id": monitorID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rules []models.Rule
	if err := cursor.All(ctx, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

func (r *RuleRepository) FindActiveByMonitor(ctx context.Context, monitorID primitive.ObjectID) ([]models.Rule, error) {
	cursor, err := r.col.Find(ctx, bson.M{
		"monitor_id": monitorID,
		"active":     true,
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rules []models.Rule
	if err := cursor.All(ctx, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

func (r *RuleRepository) FindAll(ctx context.Context) ([]models.Rule, error) {
	// Exclude soft-deleted rules
	cursor, err := r.col.Find(ctx, bson.M{"deleted_at": bson.M{"$exists": false}})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rules []models.Rule
	if err := cursor.All(ctx, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

func (r *RuleRepository) Update(ctx context.Context, id primitive.ObjectID, update bson.M) error {
	update["updated_at"] = time.Now()
	_, err := r.col.UpdateByID(ctx, id, bson.M{"$set": update})
	return err
}

func (r *RuleRepository) IncrementTriggerCount(ctx context.Context, id primitive.ObjectID) error {
	now := time.Now()
	_, err := r.col.UpdateByID(ctx, id, bson.M{
		"$inc": bson.M{"trigger_count": 1},
		"$set": bson.M{"last_triggered": now, "updated_at": now},
	})
	return err
}

// Soft-delete — regulatory retention requires keeping rule history (FATF Rec. 11)
func (r *RuleRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	now := time.Now()
	_, err := r.col.UpdateByID(ctx, id, bson.M{
		"$set": bson.M{
			"deleted_at": now,
			"active":     false,
			"updated_at": now,
		},
	})
	return err
}

// FindScheduled returns all active rules with schedule enabled
func (r *RuleRepository) FindScheduled(ctx context.Context) ([]models.Rule, error) {
	cursor, err := r.col.Find(ctx, bson.M{
		"active":           true,
		"schedule.enabled": true,
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rules []models.Rule
	if err := cursor.All(ctx, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

// UpdateScheduleState updates the schedule's lastEvaluated and nextRun timestamps
func (r *RuleRepository) UpdateScheduleState(ctx context.Context, id primitive.ObjectID, lastEvaluated, nextRun time.Time) error {
	_, err := r.col.UpdateByID(ctx, id, bson.M{
		"$set": bson.M{
			"schedule.last_evaluated": lastEvaluated,
			"schedule.next_run":       nextRun,
			"updated_at":              time.Now(),
		},
	})
	return err
}
