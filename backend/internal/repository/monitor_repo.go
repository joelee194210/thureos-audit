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

type MonitorRepository struct {
	col *mongo.Collection
	db  *database.MongoDB
}

func NewMonitorRepository(db *database.MongoDB) *MonitorRepository {
	return &MonitorRepository{
		col: db.Collection("monitors"),
		db:  db,
	}
}

func (r *MonitorRepository) Create(ctx context.Context, monitor *models.Monitor) error {
	monitor.CreatedAt = time.Now()
	monitor.UpdatedAt = time.Now()

	result, err := r.col.InsertOne(ctx, monitor)
	if err != nil {
		return err
	}
	monitor.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *MonitorRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.Monitor, error) {
	var monitor models.Monitor
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&monitor)
	if err != nil {
		return nil, err
	}
	return &monitor, nil
}

func (r *MonitorRepository) FindAll(ctx context.Context) ([]models.Monitor, error) {
	cursor, err := r.col.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var monitors []models.Monitor
	if err := cursor.All(ctx, &monitors); err != nil {
		return nil, err
	}
	return monitors, nil
}

func (r *MonitorRepository) FindByOwner(ctx context.Context, ownerID primitive.ObjectID) ([]models.Monitor, error) {
	cursor, err := r.col.Find(ctx, bson.M{"owner_id": ownerID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var monitors []models.Monitor
	if err := cursor.All(ctx, &monitors); err != nil {
		return nil, err
	}
	return monitors, nil
}

func (r *MonitorRepository) Update(ctx context.Context, id primitive.ObjectID, update bson.M) error {
	update["updated_at"] = time.Now()
	_, err := r.col.UpdateByID(ctx, id, bson.M{"$set": update})
	return err
}

func (r *MonitorRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

// FindPullMonitors returns all API-source monitors configured for pull mode.
// The caller filters by NextPullAt to decide which are actually due —
// same split as RuleRepository.FindScheduled + Scheduler.checkAndFireRules.
func (r *MonitorRepository) FindPullMonitors(ctx context.Context) ([]models.Monitor, error) {
	cursor, err := r.col.Find(ctx, bson.M{
		"source_type":        models.SourceAPI,
		"source_config.mode": models.APIModePull,
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var monitors []models.Monitor
	if err := cursor.All(ctx, &monitors); err != nil {
		return nil, err
	}
	return monitors, nil
}

// GetDataCollection returns the dynamic collection for a monitor's data
func (r *MonitorRepository) GetDataCollection(collectionID string) *mongo.Collection {
	return r.db.Collection("data_" + collectionID)
}

func (r *MonitorRepository) InsertData(ctx context.Context, collectionID string, documents []interface{}) (int, error) {
	col := r.GetDataCollection(collectionID)
	result, err := col.InsertMany(ctx, documents)
	if err != nil {
		return 0, err
	}
	return len(result.InsertedIDs), nil
}

func (r *MonitorRepository) QueryData(ctx context.Context, collectionID string, filter bson.M, limit int64) ([]bson.M, error) {
	col := r.GetDataCollection(collectionID)

	findOpts := options.Find()
	if limit > 0 {
		findOpts.SetLimit(limit)
	}

	if filter == nil {
		filter = bson.M{}
	}

	cursor, err := col.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// QueryDataPaginated queries data with pagination and returns results + total count
func (r *MonitorRepository) QueryDataPaginated(ctx context.Context, collectionID string, filter bson.M, page, pageSize int64) ([]bson.M, int64, error) {
	col := r.GetDataCollection(collectionID)

	if filter == nil {
		filter = bson.M{}
	}

	total, err := col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	skip := (page - 1) * pageSize
	findOpts := options.Find().SetSkip(skip).SetLimit(pageSize)

	cursor, err := col.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, 0, err
	}
	return results, total, nil
}

// IngestionEntry represents a single day's upload for a monitor
type IngestionEntry struct {
	MonitorID   string `json:"monitorId"`
	MonitorName string `json:"monitorName"`
	SourceType  string `json:"sourceType"`
	Date        string `json:"date"`
	RecordCount int64  `json:"recordCount"`
}

// GetIngestionHistory returns upload records grouped by date across all monitors
func (r *MonitorRepository) GetIngestionHistory(ctx context.Context) ([]IngestionEntry, error) {
	monitors, err := r.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	var entries []IngestionEntry
	for _, m := range monitors {
		col := r.GetDataCollection(m.CollectionID)

		pipeline := mongo.Pipeline{
			{{Key: "$group", Value: bson.D{
				{Key: "_id", Value: bson.D{{Key: "$dateToString", Value: bson.D{
					{Key: "format", Value: "%Y-%m-%d"},
					{Key: "date", Value: "$_ingested_at"},
				}}}},
				{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
			}}},
			{{Key: "$sort", Value: bson.D{{Key: "_id", Value: -1}}}},
		}

		cursor, err := col.Aggregate(ctx, pipeline)
		if err != nil {
			continue // skip monitors with no data
		}

		var results []bson.M
		if err := cursor.All(ctx, &results); err != nil {
			cursor.Close(ctx)
			continue
		}
		cursor.Close(ctx)

		for _, r := range results {
			date, _ := r["_id"].(string)
			count, _ := r["count"].(int32)
			if date == "" {
				continue
			}
			entries = append(entries, IngestionEntry{
				MonitorID:   m.ID.Hex(),
				MonitorName: m.Name,
				SourceType:  string(m.SourceType),
				Date:        date,
				RecordCount: int64(count),
			})
		}
	}

	if entries == nil {
		entries = []IngestionEntry{}
	}
	return entries, nil
}

func (r *MonitorRepository) AggregateData(ctx context.Context, collectionID string, pipeline mongo.Pipeline) ([]bson.M, error) {
	col := r.GetDataCollection(collectionID)
	cursor, err := col.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}
