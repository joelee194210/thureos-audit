package repository

import (
	"context"

	"github.com/joelee/datawatch/internal/database"
	"github.com/joelee/datawatch/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MCCRepository struct {
	collection *mongo.Collection
}

func NewMCCRepository(db *database.MongoDB) *MCCRepository {
	return &MCCRepository{collection: db.Database.Collection("mccs")}
}

func (r *MCCRepository) Seed(ctx context.Context, mccs []models.MCC) error {
	count, err := r.collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return err
	}
	if count > 0 {
		return nil // already seeded
	}

	// Create unique index on code
	r.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "code", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	// Text index for search
	r.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "description", Value: "text"},
			{Key: "category", Value: "text"},
		},
	})

	docs := make([]interface{}, len(mccs))
	for i, m := range mccs {
		docs[i] = m
	}
	opts := options.InsertMany().SetOrdered(false)
	_, err = r.collection.InsertMany(ctx, docs, opts)
	if mongo.IsDuplicateKeyError(err) {
		return nil // ignore duplicate key errors
	}
	return err
}

func (r *MCCRepository) FindAll(ctx context.Context) ([]models.MCC, error) {
	cursor, err := r.collection.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "code", Value: 1}}))
	if err != nil {
		return nil, err
	}
	var results []models.MCC
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *MCCRepository) Search(ctx context.Context, query string) ([]models.MCC, error) {
	filter := bson.M{
		"$or": []bson.M{
			{"code": bson.M{"$regex": query, "$options": "i"}},
			{"description": bson.M{"$regex": query, "$options": "i"}},
			{"category": bson.M{"$regex": query, "$options": "i"}},
		},
	}
	cursor, err := r.collection.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "code", Value: 1}}))
	if err != nil {
		return nil, err
	}
	var results []models.MCC
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *MCCRepository) FindByCategory(ctx context.Context, category string) ([]models.MCC, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"category": category}, options.Find().SetSort(bson.D{{Key: "code", Value: 1}}))
	if err != nil {
		return nil, err
	}
	var results []models.MCC
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *MCCRepository) GetCategories(ctx context.Context) ([]string, error) {
	values, err := r.collection.Distinct(ctx, "category", bson.M{})
	if err != nil {
		return nil, err
	}
	cats := make([]string, 0, len(values))
	for _, v := range values {
		if s, ok := v.(string); ok {
			cats = append(cats, s)
		}
	}
	return cats, nil
}

func (r *MCCRepository) Update(ctx context.Context, id interface{}, update bson.M) error {
	_, err := r.collection.UpdateByID(ctx, id, bson.M{"$set": update})
	return err
}

func (r *MCCRepository) FindByRiskLevel(ctx context.Context, level string) ([]models.MCC, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"risk_level": level}, options.Find().SetSort(bson.D{{Key: "code", Value: 1}}))
	if err != nil {
		return nil, err
	}
	var results []models.MCC
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}
