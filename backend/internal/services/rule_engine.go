package services

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type RuleEngine struct {
	ruleRepo    *repository.RuleRepository
	redFlagRepo *repository.RedFlagRepository
	monitorRepo *repository.MonitorRepository
	reportRepo  *repository.RedFlagReportRepository
	execLogRepo *repository.RuleExecutionLogRepository
	notifier    *NotificationService
}

func NewRuleEngine(
	ruleRepo *repository.RuleRepository,
	redFlagRepo *repository.RedFlagRepository,
	monitorRepo *repository.MonitorRepository,
	reportRepo *repository.RedFlagReportRepository,
	execLogRepo ...*repository.RuleExecutionLogRepository,
) *RuleEngine {
	e := &RuleEngine{
		ruleRepo:    ruleRepo,
		redFlagRepo: redFlagRepo,
		monitorRepo: monitorRepo,
		reportRepo:  reportRepo,
	}
	if len(execLogRepo) > 0 {
		e.execLogRepo = execLogRepo[0]
	}
	return e
}

// SetNotifier conecta el servicio de notificaciones (opcional, para no
// romper los constructores existentes ni los tests).
func (e *RuleEngine) SetNotifier(n *NotificationService) {
	e.notifier = n
}

// triggerNotification encola el aviso de una bandera roja NUEVA; el
// upsert de una existente no re-avisa (evita tormentas por un mismo
// patrón). Corre en goroutine propia por la misma razón que el reporte.
func (e *RuleEngine) triggerNotification(rf models.RedFlag, isNew bool) {
	if e.notifier == nil || !isNew {
		return
	}
	go e.notifier.TriggerRedFlag(rf)
}

// triggerReportGeneration arma y guarda el PDF automático de una bandera
// roja recién creada. Corre en su propia goroutine con timeout propio: un
// fallo acá nunca debe bloquear ni revertir la creación de la alerta, que
// ya quedó confirmada en Mongo antes de este llamado.
func (e *RuleEngine) triggerReportGeneration(rf models.RedFlag) {
	if e.reportRepo == nil {
		return
	}
	go func() {
		pdf, err := RenderRedFlagReportPDF(&rf)
		if err != nil {
			log.Printf("WARNING: failed to render report for red flag %s: %v", rf.ID.Hex(), err)
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := e.reportRepo.Save(ctx, rf.ID, pdf); err != nil {
			log.Printf("WARNING: failed to save report for red flag %s: %v", rf.ID.Hex(), err)
		}
	}()
}

// EvaluateRules runs all active rules for a monitor against its data
func (e *RuleEngine) EvaluateRules(ctx context.Context, monitor *models.Monitor) ([]models.RedFlag, error) {
	rules, err := e.ruleRepo.FindActiveByMonitor(ctx, monitor.ID)
	if err != nil {
		return nil, fmt.Errorf("fetching rules: %w", err)
	}

	var redFlags []models.RedFlag

	for _, rule := range rules {
		ruleStart := time.Now()
		ruleRedFlagCount := 0

		// Evaluate standard row-level conditions
		if len(rule.ConditionGroup.Conditions) > 0 {
			filter := BuildMongoFilter(rule.ConditionGroup)
			matches, err := e.monitorRepo.QueryData(ctx, monitor.CollectionID, filter, 100)
			if err != nil {
				log.Printf("ERROR rule %s query failed for monitor %s: %v", rule.ID.Hex(), monitor.ID.Hex(), err)
			} else if len(matches) > 0 {
				// Store up to 20 records for display
				limit := len(matches)
				if limit > 20 {
					limit = 20
				}
				records := make([]map[string]interface{}, limit)
				for i := 0; i < limit; i++ {
					records[i] = map[string]interface{}(matches[i])
				}
				today := time.Now()
				redFlag := models.RedFlag{
					Fingerprint:    models.RowRedFlagFingerprint(rule.ID, monitor.ID, today),
					MonitorID:      monitor.ID,
					RuleID:         rule.ID,
					RuleName:       rule.Name,
					MonitorName:    monitor.Name,
					Severity:       rule.Severity,
					RedFlagType:    models.RedFlagTypeRow,
					Message:        fmt.Sprintf("Rule '%s' matched %d records", rule.Name, len(matches)),
					MatchedData:    matches[0],
					MatchedRecords: records,
					MatchCount:     len(matches),
				}
				isNew, err := e.redFlagRepo.Upsert(ctx, &redFlag)
				if err != nil {
					log.Printf("ERROR upserting red flag for rule %s: %v", rule.ID.Hex(), err)
				} else {
					redFlags = append(redFlags, redFlag)
					if isNew {
						if err := e.ruleRepo.IncrementTriggerCount(ctx, rule.ID); err != nil {
							log.Printf("WARNING: failed to increment trigger count for rule %s: %v", rule.ID.Hex(), err)
						}
						e.triggerReportGeneration(redFlag)
						e.triggerNotification(redFlag, isNew)
					}
				}
			}
		}

		// Evaluate aggregate conditions (SUM/COUNT/AVG over time windows)
		for _, aggCond := range rule.AggregateConditions {
			aggRedFlags, err := e.evaluateAggregateCondition(ctx, monitor, rule, aggCond)
			if err != nil {
				log.Printf("ERROR aggregate condition for rule %s field=%s: %v", rule.ID.Hex(), aggCond.Field, err)
				continue
			}
			for i := range aggRedFlags {
				isNew, err := e.redFlagRepo.Upsert(ctx, &aggRedFlags[i])
				if err != nil {
					log.Printf("ERROR upserting aggregate red flag for rule %s: %v", rule.ID.Hex(), err)
				} else {
					redFlags = append(redFlags, aggRedFlags[i])
					ruleRedFlagCount++
					if isNew {
						if err := e.ruleRepo.IncrementTriggerCount(ctx, rule.ID); err != nil {
							log.Printf("WARNING: failed to increment trigger count for rule %s: %v", rule.ID.Hex(), err)
						}
						e.triggerReportGeneration(aggRedFlags[i])
						e.triggerNotification(aggRedFlags[i], isNew)
					}
				}
			}
		}

		// Log rule execution for audit trail
		if e.execLogRepo != nil {
			execLog := &models.RuleExecutionLog{
				RuleID:            rule.ID,
				RuleName:          rule.Name,
				MonitorID:         monitor.ID,
				MonitorName:       monitor.Name,
				RedFlagsGenerated: ruleRedFlagCount,
				DurationMs:        time.Since(ruleStart).Milliseconds(),
				Trigger:           "upload",
				Success:           true,
			}
			if err := e.execLogRepo.Create(ctx, execLog); err != nil {
				log.Printf("WARNING: failed to log rule execution: %v", err)
			}
		}
	}

	return redFlags, nil
}

// EvaluateRecord checks a single record against a rule
func (e *RuleEngine) EvaluateRecord(record bson.M, rule models.Rule) bool {
	return evaluateConditionGroup(record, rule.ConditionGroup)
}

func evaluateConditionGroup(record bson.M, group models.ConditionGroup) bool {
	if len(group.Conditions) == 0 {
		return false
	}

	if group.Logic == models.LogicAND {
		for _, cond := range group.Conditions {
			if !evaluateCondition(record, cond) {
				return false
			}
		}
		return true
	}

	// OR logic
	for _, cond := range group.Conditions {
		if evaluateCondition(record, cond) {
			return true
		}
	}
	return false
}

func evaluateCondition(record bson.M, cond models.Condition) bool {
	fieldVal, exists := record[cond.Field]
	if !exists {
		return cond.Operator == models.OpIsNull
	}

	switch cond.Operator {
	case models.OpEqual:
		return fmt.Sprintf("%v", fieldVal) == fmt.Sprintf("%v", cond.Value)
	case models.OpNotEqual:
		return fmt.Sprintf("%v", fieldVal) != fmt.Sprintf("%v", cond.Value)
	case models.OpGreaterThan:
		return toFloat(fieldVal) > toFloat(cond.Value)
	case models.OpLessThan:
		return toFloat(fieldVal) < toFloat(cond.Value)
	case models.OpGreaterEqual:
		return toFloat(fieldVal) >= toFloat(cond.Value)
	case models.OpLessEqual:
		return toFloat(fieldVal) <= toFloat(cond.Value)
	case models.OpContains:
		return strings.Contains(
			strings.ToLower(fmt.Sprintf("%v", fieldVal)),
			strings.ToLower(fmt.Sprintf("%v", cond.Value)),
		)
	case models.OpRegex:
		pattern := fmt.Sprintf("%v", cond.Value)
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false
		}
		return re.MatchString(fmt.Sprintf("%v", fieldVal))
	case models.OpBetween:
		// Value is expected to be [min, max]
		arr, ok := cond.Value.([]interface{})
		if !ok || len(arr) != 2 {
			return false
		}
		val := toFloat(fieldVal)
		return val >= toFloat(arr[0]) && val <= toFloat(arr[1])
	case models.OpStartsWith:
		return strings.HasPrefix(
			strings.ToLower(fmt.Sprintf("%v", fieldVal)),
			strings.ToLower(fmt.Sprintf("%v", cond.Value)),
		)
	case models.OpEndsWith:
		return strings.HasSuffix(
			strings.ToLower(fmt.Sprintf("%v", fieldVal)),
			strings.ToLower(fmt.Sprintf("%v", cond.Value)),
		)
	case models.OpIsNull:
		return fieldVal == nil
	case models.OpIsNotNull:
		return fieldVal != nil
	}

	return false
}

// BuildMongoFilter converts a ConditionGroup to a MongoDB query filter
func BuildMongoFilter(group models.ConditionGroup) bson.M {
	if len(group.Conditions) == 0 {
		return bson.M{}
	}

	conditions := make([]bson.M, 0, len(group.Conditions))
	for _, cond := range group.Conditions {
		conditions = append(conditions, conditionToMongo(cond))
	}

	if len(conditions) == 1 {
		return conditions[0]
	}

	if group.Logic == models.LogicAND {
		return bson.M{"$and": conditions}
	}
	return bson.M{"$or": conditions}
}

func conditionToMongo(cond models.Condition) bson.M {
	switch cond.Operator {
	case models.OpEqual:
		return bson.M{cond.Field: cond.Value}
	case models.OpNotEqual:
		return bson.M{cond.Field: bson.M{"$ne": cond.Value}}
	case models.OpGreaterThan:
		return bson.M{cond.Field: bson.M{"$gt": toFloat(cond.Value)}}
	case models.OpLessThan:
		return bson.M{cond.Field: bson.M{"$lt": toFloat(cond.Value)}}
	case models.OpGreaterEqual:
		return bson.M{cond.Field: bson.M{"$gte": toFloat(cond.Value)}}
	case models.OpLessEqual:
		return bson.M{cond.Field: bson.M{"$lte": toFloat(cond.Value)}}
	case models.OpContains:
		escaped := regexp.QuoteMeta(fmt.Sprintf("%v", cond.Value))
		return bson.M{cond.Field: bson.M{"$regex": escaped, "$options": "i"}}
	case models.OpRegex:
		return bson.M{cond.Field: bson.M{"$regex": fmt.Sprintf("%v", cond.Value)}}
	case models.OpIn:
		return bson.M{cond.Field: bson.M{"$in": cond.Value}}
	case models.OpNotIn:
		return bson.M{cond.Field: bson.M{"$nin": cond.Value}}
	case models.OpBetween:
		arr, ok := cond.Value.([]interface{})
		if ok && len(arr) == 2 {
			return bson.M{cond.Field: bson.M{"$gte": toFloat(arr[0]), "$lte": toFloat(arr[1])}}
		}
		return bson.M{cond.Field: cond.Value}
	case models.OpStartsWith:
		escaped := regexp.QuoteMeta(fmt.Sprintf("%v", cond.Value))
		return bson.M{cond.Field: bson.M{"$regex": "^" + escaped, "$options": "i"}}
	case models.OpEndsWith:
		escaped := regexp.QuoteMeta(fmt.Sprintf("%v", cond.Value))
		return bson.M{cond.Field: bson.M{"$regex": escaped + "$", "$options": "i"}}
	case models.OpIsNull:
		return bson.M{cond.Field: nil}
	case models.OpIsNotNull:
		return bson.M{cond.Field: bson.M{"$ne": nil}}
	default:
		return bson.M{cond.Field: cond.Value}
	}
}

func toFloat(val interface{}) float64 {
	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		f := 0.0
		if _, err := fmt.Sscanf(v, "%f", &f); err != nil {
			return 0
		}
		return f
	default:
		return 0
	}
}

// evaluateAggregateCondition runs a MongoDB aggregation pipeline for an aggregate condition.
// It groups by GroupBy field, applies the aggregate function, and finds groups exceeding the threshold.
func (e *RuleEngine) evaluateAggregateCondition(
	ctx context.Context,
	monitor *models.Monitor,
	rule models.Rule,
	cond models.AggregateCondition,
) ([]models.RedFlag, error) {
	pipeline := buildAggregatePipeline(cond)

	results, err := e.monitorRepo.AggregateData(ctx, monitor.CollectionID, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate evaluation: %w", err)
	}

	today := time.Now()
	var redFlags []models.RedFlag
	for _, result := range results {
		aggValue := toFloat(result["aggValue"])
		if !compareThreshold(aggValue, cond.Operator, cond.Threshold) {
			continue
		}

		groupKey := fmt.Sprintf("%v", result["_id"])
		count := 0
		if c, ok := result["count"]; ok {
			count = int(toFloat(c))
		}
		redFlag := models.RedFlag{
			Fingerprint:  models.AggRedFlagFingerprint(rule.ID, monitor.ID, today, groupKey),
			MonitorID:    monitor.ID,
			RuleID:       rule.ID,
			RuleName:     rule.Name,
			MonitorName:  monitor.Name,
			Severity:     rule.Severity,
			RedFlagType:  models.RedFlagTypeAggregate,
			AggField:     cond.Field,
			AggFunction:  string(cond.Function),
			AggValue:     aggValue,
			GroupByField: cond.GroupBy,
			GroupByValue: groupKey,
			Threshold:    cond.Threshold,
			Message: fmt.Sprintf(
				"Aggregate rule '%s': %s(%s) for %s='%s' = %.2f (threshold: %s %.2f)",
				rule.Name, cond.Function, cond.Field, cond.GroupBy, groupKey,
				aggValue, cond.Operator, cond.Threshold,
			),
			MatchedData: result,
			MatchCount:  count,
		}
		redFlags = append(redFlags, redFlag)
	}
	return redFlags, nil
}

// buildAggregatePipeline creates a MongoDB aggregation pipeline for an AggregateCondition.
// Pipeline: $match (time window) → $group (by groupBy, apply agg func) → $match (threshold) → $limit
func buildAggregatePipeline(cond models.AggregateCondition) mongo.Pipeline {
	pipeline := mongo.Pipeline{}

	// Step 1: Filter by time window if configured
	if cond.TimeField != "" && cond.TimeWindow != "" {
		dur := parseTimeWindow(cond.TimeWindow)
		if dur > 0 {
			cutoff := time.Now().Add(-dur)
			pipeline = append(pipeline, bson.D{
				{Key: "$match", Value: bson.D{
					{Key: cond.TimeField, Value: bson.D{
						{Key: "$gte", Value: cutoff.Format("01/02/2006 03:04 PM")},
					}},
				}},
			})
		}
	}

	// Step 2: Group by groupBy field and apply aggregation
	aggExpr := buildAggExpr(cond.Function, cond.Field)
	groupID := interface{}(nil)
	if cond.GroupBy != "" {
		groupID = "$" + cond.GroupBy
	}

	pipeline = append(pipeline, bson.D{
		{Key: "$group", Value: bson.D{
			{Key: "_id", Value: groupID},
			{Key: "aggValue", Value: aggExpr},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}},
	})

	// Step 3: Filter by threshold
	thresholdFilter := bson.D{
		{Key: "$match", Value: bson.D{
			{Key: "aggValue", Value: condOperatorToBson(cond.Operator, cond.Threshold)},
		}},
	}
	pipeline = append(pipeline, thresholdFilter)

	// Step 4: Limit results
	pipeline = append(pipeline, bson.D{
		{Key: "$sort", Value: bson.D{{Key: "aggValue", Value: -1}}},
	})
	pipeline = append(pipeline, bson.D{
		{Key: "$limit", Value: 50},
	})

	return pipeline
}

func buildAggExpr(fn models.AggFunction, field string) bson.D {
	switch fn {
	case models.AggFuncSum:
		return bson.D{{Key: "$sum", Value: "$" + field}}
	case models.AggFuncCount:
		return bson.D{{Key: "$sum", Value: 1}}
	case models.AggFuncAvg:
		return bson.D{{Key: "$avg", Value: "$" + field}}
	case models.AggFuncMin:
		return bson.D{{Key: "$min", Value: "$" + field}}
	case models.AggFuncMax:
		return bson.D{{Key: "$max", Value: "$" + field}}
	default:
		return bson.D{{Key: "$sum", Value: "$" + field}}
	}
}

func condOperatorToBson(op models.Operator, threshold float64) bson.D {
	switch op {
	case models.OpGreaterThan:
		return bson.D{{Key: "$gt", Value: threshold}}
	case models.OpGreaterEqual:
		return bson.D{{Key: "$gte", Value: threshold}}
	case models.OpLessThan:
		return bson.D{{Key: "$lt", Value: threshold}}
	case models.OpLessEqual:
		return bson.D{{Key: "$lte", Value: threshold}}
	case models.OpEqual:
		return bson.D{{Key: "$eq", Value: threshold}}
	case models.OpNotEqual:
		return bson.D{{Key: "$ne", Value: threshold}}
	default:
		return bson.D{{Key: "$gt", Value: threshold}}
	}
}

func compareThreshold(value float64, op models.Operator, threshold float64) bool {
	switch op {
	case models.OpGreaterThan:
		return value > threshold
	case models.OpGreaterEqual:
		return value >= threshold
	case models.OpLessThan:
		return value < threshold
	case models.OpLessEqual:
		return value <= threshold
	case models.OpEqual:
		return value == threshold
	case models.OpNotEqual:
		return value != threshold
	default:
		return value > threshold
	}
}

// EvaluateRuleForDateRange evaluates a single rule against data ingested in [dayStart, dayEnd).
// Used by the scheduler for day-by-day backlog processing.
func (e *RuleEngine) EvaluateRuleForDateRange(
	ctx context.Context,
	monitor *models.Monitor,
	rule models.Rule,
	dayStart, dayEnd time.Time,
) ([]models.RedFlag, error) {
	candidates := e.matchRuleForDateRange(ctx, monitor, rule, dayStart, dayEnd)

	var redFlags []models.RedFlag
	for i := range candidates {
		candidate := candidates[i]
		isNew, err := e.redFlagRepo.Upsert(ctx, &candidate)
		if err != nil {
			log.Printf("ERROR upserting %s red flag for rule %s: %v", candidate.RedFlagType, rule.ID.Hex(), err)
			continue
		}
		redFlags = append(redFlags, candidate)
		if isNew {
			if err := e.ruleRepo.IncrementTriggerCount(ctx, rule.ID); err != nil {
				log.Printf("WARNING: failed to increment trigger count for rule %s: %v", rule.ID.Hex(), err)
			}
			e.triggerReportGeneration(candidate)
			e.triggerNotification(candidate, isNew)
		}
	}
	return redFlags, nil
}

// BacktestRule reporta qué generaría una regla en [dayStart, dayEnd) sin
// persistir nada: ni red flag, ni reporte, ni notificación, ni incremento
// de trigger_count. Para que un usuario pueda ver el impacto de una regla
// antes de activarla — mismo matching que EvaluateRuleForDateRange, sin
// los efectos secundarios de ese camino.
func (e *RuleEngine) BacktestRule(
	ctx context.Context,
	monitor *models.Monitor,
	rule models.Rule,
	dayStart, dayEnd time.Time,
) []models.RedFlag {
	return e.matchRuleForDateRange(ctx, monitor, rule, dayStart, dayEnd)
}

// matchRuleForDateRange encuentra qué matchearía una regla en
// [dayStart, dayEnd) sin tocar Mongo en escritura — ni upsert, ni
// trigger count, ni notificación. Los red flags devueltos no tienen ID
// (nunca se persistieron); el caller decide qué hacer con ellos.
func (e *RuleEngine) matchRuleForDateRange(
	ctx context.Context,
	monitor *models.Monitor,
	rule models.Rule,
	dayStart, dayEnd time.Time,
) []models.RedFlag {
	var candidates []models.RedFlag

	// Date filter scoped to _ingested_at
	dateFilter := bson.M{
		"_ingested_at": bson.M{
			"$gte": dayStart,
			"$lt":  dayEnd,
		},
	}

	// Evaluate row-level conditions within the date range
	if len(rule.ConditionGroup.Conditions) > 0 {
		condFilter := BuildMongoFilter(rule.ConditionGroup)
		// Merge date filter with condition filter
		mergedFilter := bson.M{"$and": []bson.M{dateFilter, condFilter}}

		matches, err := e.monitorRepo.QueryData(ctx, monitor.CollectionID, mergedFilter, 100)
		if err != nil {
			log.Printf("ERROR rule %s date-range query failed: %v", rule.ID.Hex(), err)
		} else if len(matches) > 0 {
			limit := len(matches)
			if limit > 20 {
				limit = 20
			}
			records := make([]map[string]interface{}, limit)
			for i := 0; i < limit; i++ {
				records[i] = map[string]interface{}(matches[i])
			}
			candidates = append(candidates, models.RedFlag{
				Fingerprint:    models.RowRedFlagFingerprint(rule.ID, monitor.ID, dayStart),
				MonitorID:      monitor.ID,
				RuleID:         rule.ID,
				RuleName:       rule.Name,
				MonitorName:    monitor.Name,
				Severity:       rule.Severity,
				RedFlagType:    models.RedFlagTypeRow,
				Message:        fmt.Sprintf("Scheduled rule '%s' matched %d records (%s)", rule.Name, len(matches), dayStart.Format("2006-01-02")),
				MatchedData:    matches[0],
				MatchedRecords: records,
				MatchCount:     len(matches),
			})
		}
	}

	// Evaluate aggregate conditions with date-scoped pipeline
	for _, aggCond := range rule.AggregateConditions {
		aggRedFlags, err := e.evaluateAggregateConditionDateScoped(ctx, monitor, rule, aggCond, dayStart, dayEnd)
		if err != nil {
			log.Printf("ERROR aggregate condition (date-scoped) for rule %s: %v", rule.ID.Hex(), err)
			continue
		}
		candidates = append(candidates, aggRedFlags...)
	}

	return candidates
}

// EvaluateRuleNow evaluates a single rule against today's data (real-time execution).
func (e *RuleEngine) EvaluateRuleNow(
	ctx context.Context,
	monitor *models.Monitor,
	rule models.Rule,
) ([]models.RedFlag, error) {
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dayEnd := dayStart.Add(24 * time.Hour)
	return e.EvaluateRuleForDateRange(ctx, monitor, rule, dayStart, dayEnd)
}

// evaluateAggregateConditionDateScoped runs an aggregate pipeline scoped to a date range.
func (e *RuleEngine) evaluateAggregateConditionDateScoped(
	ctx context.Context,
	monitor *models.Monitor,
	rule models.Rule,
	cond models.AggregateCondition,
	dayStart, dayEnd time.Time,
) ([]models.RedFlag, error) {
	pipeline := buildAggregatePipelineDateScoped(cond, dayStart, dayEnd)

	results, err := e.monitorRepo.AggregateData(ctx, monitor.CollectionID, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate evaluation (date-scoped): %w", err)
	}

	var redFlags []models.RedFlag
	for _, result := range results {
		aggValue := toFloat(result["aggValue"])
		if !compareThreshold(aggValue, cond.Operator, cond.Threshold) {
			continue
		}

		groupKey := fmt.Sprintf("%v", result["_id"])
		count := 0
		if c, ok := result["count"]; ok {
			count = int(toFloat(c))
		}
		redFlag := models.RedFlag{
			Fingerprint:  models.AggRedFlagFingerprint(rule.ID, monitor.ID, dayStart, groupKey),
			MonitorID:    monitor.ID,
			RuleID:       rule.ID,
			RuleName:     rule.Name,
			MonitorName:  monitor.Name,
			Severity:     rule.Severity,
			RedFlagType:  models.RedFlagTypeAggregate,
			AggField:     cond.Field,
			AggFunction:  string(cond.Function),
			AggValue:     aggValue,
			GroupByField: cond.GroupBy,
			GroupByValue: groupKey,
			Threshold:    cond.Threshold,
			Message: fmt.Sprintf(
				"Scheduled rule '%s': %s(%s) for %s='%s' = %.2f (threshold: %s %.2f) [%s]",
				rule.Name, cond.Function, cond.Field, cond.GroupBy, groupKey,
				aggValue, cond.Operator, cond.Threshold, dayStart.Format("2006-01-02"),
			),
			MatchedData: result,
			MatchCount:  count,
		}
		redFlags = append(redFlags, redFlag)
	}
	return redFlags, nil
}

// buildAggregatePipelineDateScoped creates a pipeline filtered by _ingested_at date range
func buildAggregatePipelineDateScoped(cond models.AggregateCondition, dayStart, dayEnd time.Time) mongo.Pipeline {
	pipeline := mongo.Pipeline{}

	// Date range filter on _ingested_at
	pipeline = append(pipeline, bson.D{
		{Key: "$match", Value: bson.D{
			{Key: "_ingested_at", Value: bson.D{
				{Key: "$gte", Value: dayStart},
				{Key: "$lt", Value: dayEnd},
			}},
		}},
	})

	// Also apply time window filter if configured (relative to dayEnd)
	if cond.TimeField != "" && cond.TimeWindow != "" {
		dur := parseTimeWindow(cond.TimeWindow)
		if dur > 0 {
			cutoff := dayEnd.Add(-dur)
			pipeline = append(pipeline, bson.D{
				{Key: "$match", Value: bson.D{
					{Key: cond.TimeField, Value: bson.D{
						{Key: "$gte", Value: cutoff.Format("01/02/2006 03:04 PM")},
					}},
				}},
			})
		}
	}

	// Group + aggregate
	aggExpr := buildAggExpr(cond.Function, cond.Field)
	groupID := interface{}(nil)
	if cond.GroupBy != "" {
		groupID = "$" + cond.GroupBy
	}
	pipeline = append(pipeline, bson.D{
		{Key: "$group", Value: bson.D{
			{Key: "_id", Value: groupID},
			{Key: "aggValue", Value: aggExpr},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}},
	})

	// Threshold filter
	pipeline = append(pipeline, bson.D{
		{Key: "$match", Value: bson.D{
			{Key: "aggValue", Value: condOperatorToBson(cond.Operator, cond.Threshold)},
		}},
	})

	// Sort + limit
	pipeline = append(pipeline, bson.D{
		{Key: "$sort", Value: bson.D{{Key: "aggValue", Value: -1}}},
	})
	pipeline = append(pipeline, bson.D{
		{Key: "$limit", Value: 50},
	})

	return pipeline
}

// parseTimeWindow converts a time window string like "24h", "7d", "30d" to a time.Duration.
func parseTimeWindow(window string) time.Duration {
	window = strings.TrimSpace(strings.ToLower(window))
	if len(window) < 2 {
		return 0
	}

	unit := window[len(window)-1:]
	numStr := window[:len(window)-1]
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return 0
	}

	switch unit {
	case "h":
		return time.Duration(num) * time.Hour
	case "d":
		return time.Duration(num) * 24 * time.Hour
	case "m":
		return time.Duration(num) * 30 * 24 * time.Hour // approx months
	default:
		return 0
	}
}
