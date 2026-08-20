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

type CountryRepository struct {
	collection *mongo.Collection
}

func NewCountryRepository(db *database.MongoDB) *CountryRepository {
	return &CountryRepository{collection: db.Database.Collection("countries")}
}

func (r *CountryRepository) Seed(ctx context.Context, countries []models.Country) error {
	count, err := r.collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	// Unique index on code
	r.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "code", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	// Index for queries
	r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "risk_level", Value: 1}}},
		{Keys: bson.D{{Key: "active", Value: 1}}},
		{Keys: bson.D{{Key: "region", Value: 1}}},
	})

	docs := make([]interface{}, len(countries))
	for i, c := range countries {
		docs[i] = c
	}
	opts := options.InsertMany().SetOrdered(false)
	_, err = r.collection.InsertMany(ctx, docs, opts)
	if mongo.IsDuplicateKeyError(err) {
		return nil
	}
	return err
}

func (r *CountryRepository) FindAll(ctx context.Context) ([]models.Country, error) {
	cursor, err := r.collection.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return nil, err
	}
	var results []models.Country
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *CountryRepository) FindActive(ctx context.Context) ([]models.Country, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"active": true}, options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return nil, err
	}
	var results []models.Country
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *CountryRepository) FindByRiskLevel(ctx context.Context, level string) ([]models.Country, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"risk_level": level, "active": true}, options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return nil, err
	}
	var results []models.Country
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *CountryRepository) FindByRegion(ctx context.Context, region string) ([]models.Country, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"region": region, "active": true}, options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return nil, err
	}
	var results []models.Country
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *CountryRepository) GetRegions(ctx context.Context) ([]string, error) {
	values, err := r.collection.Distinct(ctx, "region", bson.M{"active": true})
	if err != nil {
		return nil, err
	}
	regions := make([]string, len(values))
	for i, v := range values {
		regions[i] = v.(string)
	}
	return regions, nil
}

func (r *CountryRepository) Search(ctx context.Context, query string) ([]models.Country, error) {
	filter := bson.M{
		"$or": []bson.M{
			{"code": bson.M{"$regex": query, "$options": "i"}},
			{"code3": bson.M{"$regex": query, "$options": "i"}},
			{"name": bson.M{"$regex": query, "$options": "i"}},
			{"name_en": bson.M{"$regex": query, "$options": "i"}},
		},
	}
	cursor, err := r.collection.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return nil, err
	}
	var results []models.Country
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *CountryRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.Country, error) {
	var country models.Country
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&country)
	if err != nil {
		return nil, err
	}
	return &country, nil
}

func (r *CountryRepository) Create(ctx context.Context, country *models.Country) error {
	country.CreatedAt = time.Now()
	country.UpdatedAt = time.Now()
	result, err := r.collection.InsertOne(ctx, country)
	if err != nil {
		return err
	}
	country.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *CountryRepository) Update(ctx context.Context, id primitive.ObjectID, update bson.M) error {
	update["updated_at"] = time.Now()
	_, err := r.collection.UpdateByID(ctx, id, bson.M{"$set": update})
	return err
}

func (r *CountryRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}
