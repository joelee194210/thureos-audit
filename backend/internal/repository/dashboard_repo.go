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

type DashboardRepository struct {
	col *mongo.Collection
}

func NewDashboardRepository(db *database.MongoDB) *DashboardRepository {
	return &DashboardRepository{col: db.Collection("dashboards")}
}

func (r *DashboardRepository) Create(ctx context.Context, dashboard *models.Dashboard) error {
	dashboard.CreatedAt = time.Now()
	dashboard.UpdatedAt = time.Now()

	result, err := r.col.InsertOne(ctx, dashboard)
	if err != nil {
		return err
	}
	dashboard.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *DashboardRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.Dashboard, error) {
	var dashboard models.Dashboard
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&dashboard)
	if err != nil {
		return nil, err
	}
	return &dashboard, nil
}

// FindAccessible lists dashboards the user can see. isOrgRole mirrors
// handlers.isOrgRole (admin/compliance see the whole platform, per
// CLAUDE.md) — passing it here instead of a role string keeps this
// package unaware of the role model.
func (r *DashboardRepository) FindAccessible(ctx context.Context, userID primitive.ObjectID, isOrgRole bool) ([]models.Dashboard, error) {
	filter := bson.M{}
	if !isOrgRole {
		filter["$or"] = []bson.M{
			{"owner_id": userID},
			{"shared_with": userID},
			{"is_public": true},
		}
	}

	cursor, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	var dashboards []models.Dashboard
	if err := cursor.All(ctx, &dashboards); err != nil {
		return nil, err
	}
	return dashboards, nil
}

func (r *DashboardRepository) Update(ctx context.Context, id primitive.ObjectID, update bson.M) error {
	update["updated_at"] = time.Now()
	_, err := r.col.UpdateByID(ctx, id, bson.M{"$set": update})
	return err
}

func (r *DashboardRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	return err
}
