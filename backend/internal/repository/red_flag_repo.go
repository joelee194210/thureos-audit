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

type RedFlagRepository struct {
	col *mongo.Collection
}

func NewRedFlagRepository(db *database.MongoDB) *RedFlagRepository {
	repo := &RedFlagRepository{col: db.Collection("red_flags")}
	// Ensure unique index on fingerprint for deduplication
	repo.ensureIndexes(context.Background())
	return repo
}

func (r *RedFlagRepository) ensureIndexes(ctx context.Context) {
	// Unique sparse index: only applies to documents with a non-empty fingerprint.
	// Existing red flags without fingerprint are ignored by the sparse index.
	_, _ = r.col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "fingerprint", Value: 1}},
		Options: options.Index().
			SetUnique(true).
			SetSparse(true).
			SetName("idx_fingerprint_unique"),
	})
}

func (r *RedFlagRepository) Create(ctx context.Context, redFlag *models.RedFlag) error {
	redFlag.CreatedAt = time.Now()
	redFlag.UpdatedAt = time.Now()
	redFlag.Status = models.RedFlagNew
	slaDueAt := redFlag.CreatedAt.Add(models.SLADefaultForSeverity(redFlag.Severity))
	redFlag.SLADueAt = &slaDueAt

	result, err := r.col.InsertOne(ctx, redFlag)
	if err != nil {
		return err
	}
	redFlag.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

// Upsert creates a new red flag or updates an existing one with the same fingerprint.
// If a red flag with this fingerprint already exists:
//   - Data fields (matchedData, matchedRecords, matchCount, aggValue, message) are refreshed
//   - Status is preserved (user may have acknowledged/resolved it)
//   - CreatedAt is preserved (original detection time)
//   - UpdatedAt is set to now
//
// Returns the red flag (existing or new) and whether it was a new creation.
func (r *RedFlagRepository) Upsert(ctx context.Context, redFlag *models.RedFlag) (bool, error) {
	if redFlag.Fingerprint == "" {
		// No fingerprint — fall back to simple create
		return true, r.Create(ctx, redFlag)
	}

	now := time.Now()
	redFlag.UpdatedAt = now

	// Fields to set on insert only (not overwritten on update). sla_due_at
	// va acá, no en setAlways: es el plazo objetivo desde la primera
	// detección — re-triggerear un red flag existente no debe moverlo.
	setOnInsert := bson.M{
		"_id":        primitive.NewObjectID(),
		"created_at": now,
		"status":     models.RedFlagNew,
		"sla_due_at": now.Add(models.SLADefaultForSeverity(redFlag.Severity)),
	}

	// Fields to always update (refresh data)
	setAlways := bson.M{
		"fingerprint":     redFlag.Fingerprint,
		"monitor_id":      redFlag.MonitorID,
		"rule_id":         redFlag.RuleID,
		"rule_name":       redFlag.RuleName,
		"monitor_name":    redFlag.MonitorName,
		"severity":        redFlag.Severity,
		"message":         redFlag.Message,
		"red_flag_type":   redFlag.RedFlagType,
		"matched_data":    redFlag.MatchedData,
		"matched_records": redFlag.MatchedRecords,
		"match_count":     redFlag.MatchCount,
		"agg_field":       redFlag.AggField,
		"agg_function":    redFlag.AggFunction,
		"agg_value":       redFlag.AggValue,
		"group_by_field":  redFlag.GroupByField,
		"group_by_value":  redFlag.GroupByValue,
		"threshold":       redFlag.Threshold,
		"updated_at":      now,
	}

	filter := bson.M{"fingerprint": redFlag.Fingerprint}
	update := bson.M{
		"$set":         setAlways,
		"$setOnInsert": setOnInsert,
	}

	opts := options.Update().SetUpsert(true)
	result, err := r.col.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return false, err
	}

	isNew := result.UpsertedCount > 0
	if isNew && result.UpsertedID != nil {
		redFlag.ID = result.UpsertedID.(primitive.ObjectID)
		// setOnInsert los fija en Mongo pero no en este struct — sin esto,
		// cualquier caller que use el valor devuelto (la respuesta de
		// /rules/:id/execute, el informe PDF automático) ve CreatedAt en
		// cero y Status vacío pese a que ya quedaron bien guardados.
		redFlag.CreatedAt = now
		redFlag.Status = models.RedFlagNew
		slaDueAt := now.Add(models.SLADefaultForSeverity(redFlag.Severity))
		redFlag.SLADueAt = &slaDueAt
	}
	return isNew, nil
}

// DeleteDuplicates removes duplicate red flags that share the same rule_id + monitor_id + date.
// Keeps the oldest red flag per group. Returns the number of deleted duplicates.
func (r *RedFlagRepository) DeleteDuplicates(ctx context.Context) (int64, error) {
	// Find groups with duplicates: same rule_id + red_flag_type + day
	pipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "rule_id", Value: "$rule_id"},
				{Key: "monitor_id", Value: "$monitor_id"},
				{Key: "red_flag_type", Value: "$red_flag_type"},
				{Key: "group_by_value", Value: "$group_by_value"},
				{Key: "day", Value: bson.D{{Key: "$dateToString", Value: bson.D{
					{Key: "format", Value: "%Y-%m-%d"},
					{Key: "date", Value: "$created_at"},
				}}}},
			}},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "keepId", Value: bson.D{{Key: "$first", Value: "$_id"}}},
			{Key: "allIds", Value: bson.D{{Key: "$push", Value: "$_id"}}},
		}}},
		{{Key: "$match", Value: bson.D{
			{Key: "count", Value: bson.D{{Key: "$gt", Value: 1}}},
		}}},
	}

	cursor, err := r.col.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	var totalDeleted int64
	for cursor.Next(ctx) {
		var result bson.M
		if err := cursor.Decode(&result); err != nil {
			continue
		}
		allIds, ok := result["allIds"].(primitive.A)
		if !ok || len(allIds) <= 1 {
			continue
		}
		keepId := result["keepId"]

		// Delete all except the first (oldest)
		var toDelete []interface{}
		for _, id := range allIds {
			if id != keepId {
				toDelete = append(toDelete, id)
			}
		}
		if len(toDelete) > 0 {
			res, err := r.col.DeleteMany(ctx, bson.M{"_id": bson.M{"$in": toDelete}})
			if err == nil {
				totalDeleted += res.DeletedCount
			}
		}
	}
	return totalDeleted, nil
}

func (r *RedFlagRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.RedFlag, error) {
	var redFlag models.RedFlag
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&redFlag)
	if err != nil {
		return nil, err
	}
	return &redFlag, nil
}

func (r *RedFlagRepository) FindRecent(ctx context.Context, limit int64) ([]models.RedFlag, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(limit)

	cursor, err := r.col.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	var redFlags []models.RedFlag
	if err := cursor.All(ctx, &redFlags); err != nil {
		return nil, err
	}
	return redFlags, nil
}

func (r *RedFlagRepository) FindByMonitor(ctx context.Context, monitorID primitive.ObjectID, limit int64) ([]models.RedFlag, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(limit)

	cursor, err := r.col.Find(ctx, bson.M{"monitor_id": monitorID}, opts)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	var redFlags []models.RedFlag
	if err := cursor.All(ctx, &redFlags); err != nil {
		return nil, err
	}
	return redFlags, nil
}

// FindByRuleID returns every red flag a single rule has ever generated —
// backs the effectiveness view for one rule.
func (r *RedFlagRepository) FindByRuleID(ctx context.Context, ruleID primitive.ObjectID) ([]models.RedFlag, error) {
	cursor, err := r.col.Find(ctx, bson.M{"rule_id": ruleID})
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	var redFlags []models.RedFlag
	if err := cursor.All(ctx, &redFlags); err != nil {
		return nil, err
	}
	return redFlags, nil
}

// FindByRuleIDs returns every red flag generated by any rule in the given
// set — backs the effectiveness view grouped by typology (all the rules
// instantiated from the same template).
func (r *RedFlagRepository) FindByRuleIDs(ctx context.Context, ruleIDs []primitive.ObjectID) ([]models.RedFlag, error) {
	if len(ruleIDs) == 0 {
		return []models.RedFlag{}, nil
	}
	cursor, err := r.col.Find(ctx, bson.M{"rule_id": bson.M{"$in": ruleIDs}})
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	var redFlags []models.RedFlag
	if err := cursor.All(ctx, &redFlags); err != nil {
		return nil, err
	}
	return redFlags, nil
}

// FindOverdueUnnotified returns red flags whose SLA already breached,
// that aren't in a terminal state, and that the escalation job hasn't
// notified yet — the exact set the escalation cron needs each tick.
func (r *RedFlagRepository) FindOverdueUnnotified(ctx context.Context) ([]models.RedFlag, error) {
	filter := bson.M{
		"sla_due_at":             bson.M{"$lt": time.Now()},
		"status":                 bson.M{"$nin": bson.A{models.RedFlagResolved, models.RedFlagDismissed}},
		"escalation_notified_at": bson.M{"$exists": false},
	}
	cursor, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	var redFlags []models.RedFlag
	if err := cursor.All(ctx, &redFlags); err != nil {
		return nil, err
	}
	return redFlags, nil
}

// MarkEscalationNotified stamps escalation_notified_at so the next tick of
// the escalation job doesn't re-notify the same breach.
func (r *RedFlagRepository) MarkEscalationNotified(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.col.UpdateByID(ctx, id, bson.M{"$set": bson.M{"escalation_notified_at": time.Now()}})
	return err
}

func (r *RedFlagRepository) UpdateStatus(ctx context.Context, id primitive.ObjectID, status models.RedFlagStatus, userID *primitive.ObjectID) error {
	update := bson.M{
		"status":     status,
		"updated_at": time.Now(),
	}
	if userID != nil {
		update["acknowledged_by"] = userID
	}
	_, err := r.col.UpdateByID(ctx, id, bson.M{"$set": update})
	return err
}

func (r *RedFlagRepository) CountByStatus(ctx context.Context, status models.RedFlagStatus) (int64, error) {
	return r.col.CountDocuments(ctx, bson.M{"status": status})
}

// FindByDateRange returns red flags created within the given date range
func (r *RedFlagRepository) FindByDateRange(ctx context.Context, from, to time.Time) ([]models.RedFlag, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(500)

	filter := bson.M{
		"created_at": bson.M{
			"$gte": from,
			"$lt":  to,
		},
	}

	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	var redFlags []models.RedFlag
	if err := cursor.All(ctx, &redFlags); err != nil {
		return nil, err
	}
	return redFlags, nil
}

// CountByDay returns red flag counts per day for a month (year/month)
func (r *RedFlagRepository) CountByDay(ctx context.Context, year, month int) ([]bson.M, error) {
	from := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"created_at": bson.M{"$gte": from, "$lt": to},
		}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.M{
				"$dateToString": bson.M{"format": "%Y-%m-%d", "date": "$created_at"},
			}},
			{Key: "total", Value: bson.M{"$sum": 1}},
			{Key: "critical", Value: bson.M{"$sum": bson.M{
				"$cond": bson.A{bson.M{"$eq": bson.A{"$severity", "critical"}}, 1, 0},
			}}},
			{Key: "high", Value: bson.M{"$sum": bson.M{
				"$cond": bson.A{bson.M{"$eq": bson.A{"$severity", "high"}}, 1, 0},
			}}},
			{Key: "new", Value: bson.M{"$sum": bson.M{
				"$cond": bson.A{bson.M{"$eq": bson.A{"$status", "new"}}, 1, 0},
			}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
	}

	cursor, err := r.col.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	if results == nil {
		results = []bson.M{}
	}
	return results, nil
}
