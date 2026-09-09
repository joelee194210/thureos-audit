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

// monitorDataSource es el subconjunto de MonitorRepository que consume el
// motor de reglas. Existe como interfaz para poder ejercitar los caminos de
// evaluación (incluido el date-scoped, del que dependen el scheduler,
// "ejecutar ahora" y el backtest) sin una base real. En producción el
// constructor sigue recibiendo *repository.MonitorRepository.
type monitorDataSource interface {
	QueryData(ctx context.Context, collectionID string, filter bson.M, limit int64) ([]bson.M, error)
	AggregateData(ctx context.Context, collectionID string, pipeline mongo.Pipeline) ([]bson.M, error)
}

type RuleEngine struct {
	ruleRepo    *repository.RuleRepository
	redFlagRepo *repository.RedFlagRepository
	monitorRepo monitorDataSource
	reportRepo  *repository.RedFlagReportRepository
	execLogRepo *repository.RuleExecutionLogRepository
	notifier    *NotificationService
	screening   *ScreeningService
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

// SetScreeningService conecta el servicio de screening (opcional, para no
// romper los constructores existentes ni los tests).
func (e *RuleEngine) SetScreeningService(s *ScreeningService) {
	e.screening = s
}

// triggerScreening dispara el screening de los campos configurados en la
// regla, solo para red flags NUEVOS — mismo criterio que triggerNotification
// (evita re-screenear en cada re-trigger del mismo patrón). Corre en su
// propia goroutine, igual que el reporte y la notificación.
func (e *RuleEngine) triggerScreening(rf models.RedFlag, rule models.Rule, isNew bool) {
	if e.screening == nil || !isNew || len(rule.ScreeningFields) == 0 {
		return
	}
	go e.screening.ScreenRuleFields(context.Background(), rf.ID, rule.ScreeningFields, rf.MatchedData)
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
						e.triggerScreening(redFlag, rule, isNew)
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
						e.triggerScreening(aggRedFlags[i], rule, isNew)
					}
				}
			}
		}

		// Evaluate velocity conditions (gap entre eventos consecutivos)
		for _, velCond := range rule.VelocityConditions {
			velRedFlags, err := e.evaluateVelocityCondition(ctx, monitor, rule, velCond)
			if err != nil {
				log.Printf("ERROR velocity condition for rule %s field=%s: %v", rule.ID.Hex(), velCond.TimeField, err)
				continue
			}
			for i := range velRedFlags {
				isNew, err := e.redFlagRepo.Upsert(ctx, &velRedFlags[i])
				if err != nil {
					log.Printf("ERROR upserting velocity red flag for rule %s: %v", rule.ID.Hex(), err)
					continue
				}
				redFlags = append(redFlags, velRedFlags[i])
				ruleRedFlagCount++
				if isNew {
					if err := e.ruleRepo.IncrementTriggerCount(ctx, rule.ID); err != nil {
						log.Printf("WARNING: failed to increment trigger count for rule %s: %v", rule.ID.Hex(), err)
					}
					e.triggerReportGeneration(velRedFlags[i])
					e.triggerNotification(velRedFlags[i], isNew)
					e.triggerScreening(velRedFlags[i], rule, isNew)
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
		if cands := equalityCandidates(cond.Value); len(cands) > 1 {
			return bson.M{cond.Field: bson.M{"$in": cands}}
		}
		return bson.M{cond.Field: cond.Value}
	case models.OpNotEqual:
		if cands := equalityCandidates(cond.Value); len(cands) > 1 {
			return bson.M{cond.Field: bson.M{"$nin": cands}}
		}
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
		return bson.M{cond.Field: bson.M{"$in": listCandidates(cond.Value)}}
	case models.OpNotIn:
		return bson.M{cond.Field: bson.M{"$nin": listCandidates(cond.Value)}}
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

// equalityCandidates devuelve los valores con los que el campo puede estar
// almacenado para el mismo dato. MongoDB no compara entre tipos BSON: un mcc
// guardado como número nunca iguala al "7995" que manda el formulario, y el
// filtro no descarta nada — descarta TODO, en silencio. Los operadores
// numéricos ya salvan la diferencia con toFloat; los de igualdad la sufrían.
//
// El candidato numérico solo se agrega si el texto es su forma canónica, para
// no convertir "0012" (un identificador con ceros a la izquierda) en el número
// 12 y traer documentos que nadie pidió.
func equalityCandidates(val interface{}) []interface{} {
	switch v := val.(type) {
	case string:
		s := strings.TrimSpace(v)
		f, err := strconv.ParseFloat(s, 64)
		if err != nil || strconv.FormatFloat(f, 'f', -1, 64) != s {
			return []interface{}{val}
		}
		return []interface{}{val, f}
	case float64, float32, int, int32, int64:
		return []interface{}{val, strconv.FormatFloat(toFloat(v), 'f', -1, 64)}
	default:
		return []interface{}{val}
	}
}

// listCandidates arma la lista para $in/$nin. El formulario manda los códigos
// separados por coma en un solo string (el selector de MCC, por ejemplo), y
// $in sobre un string no es un filtro que no matchea: es un error de MongoDB
// que aborta la evaluación entera de la regla.
func listCandidates(val interface{}) []interface{} {
	var items []interface{}
	switch v := val.(type) {
	case string:
		for _, part := range strings.Split(v, ",") {
			if part = strings.TrimSpace(part); part != "" {
				items = append(items, part)
			}
		}
	case []interface{}:
		items = v
	default:
		items = []interface{}{val}
	}

	out := make([]interface{}, 0, len(items))
	for _, item := range items {
		out = append(out, equalityCandidates(item)...)
	}
	return out
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

// evaluateVelocityCondition emite una bandera roja por cada par de eventos
// consecutivos separados por menos del gap configurado. El documento que
// devuelve el pipeline ES el segundo evento del par, y lleva prevTime y
// gapSeconds adjuntos: eso es lo que se muestra en la alerta.
func (e *RuleEngine) evaluateVelocityCondition(
	ctx context.Context,
	monitor *models.Monitor,
	rule models.Rule,
	cond models.VelocityCondition,
) ([]models.RedFlag, error) {
	pipeline := buildVelocityPipeline(cond)

	results, err := e.monitorRepo.AggregateData(ctx, monitor.CollectionID, pipeline)
	if err != nil {
		return nil, fmt.Errorf("velocity evaluation: %w", err)
	}

	return velocityRedFlagsFromResults(monitor, rule, cond, results, time.Now(), ""), nil
}

// evaluateVelocityConditionDateScoped es la variante acotada a
// [dayStart, dayEnd) de evaluateVelocityCondition — la que usan el
// scheduler, "ejecutar ahora" y el backtest a través de
// matchRuleForDateRange. Misma relación que
// evaluateAggregateConditionDateScoped con su hermana sin acotar: el
// fingerprint se ancla a dayStart (no a time.Now()) para que un backlog de
// varios días no colapse en una sola bandera roja.
func (e *RuleEngine) evaluateVelocityConditionDateScoped(
	ctx context.Context,
	monitor *models.Monitor,
	rule models.Rule,
	cond models.VelocityCondition,
	dayStart, dayEnd time.Time,
) ([]models.RedFlag, error) {
	pipeline := buildVelocityPipelineDateScoped(cond, dayStart, dayEnd)

	results, err := e.monitorRepo.AggregateData(ctx, monitor.CollectionID, pipeline)
	if err != nil {
		return nil, fmt.Errorf("velocity evaluation (date-scoped): %w", err)
	}

	return velocityRedFlagsFromResults(monitor, rule, cond, results, dayStart, dayStart.Format("2006-01-02")), nil
}

// velocityRedFlagsFromResults arma una bandera roja por cada par que
// devolvió el pipeline. `day` ancla el fingerprint (dedupe por día
// evaluado); `dateLabel`, si no está vacío, se agrega al mensaje para
// distinguir el día en el camino date-scoped.
func velocityRedFlagsFromResults(
	monitor *models.Monitor,
	rule models.Rule,
	cond models.VelocityCondition,
	results []bson.M,
	day time.Time,
	dateLabel string,
) []models.RedFlag {
	var redFlags []models.RedFlag
	for _, result := range results {
		groupKey := ""
		if cond.GroupBy != "" {
			groupKey = fmt.Sprintf("%v", result[cond.GroupBy])
		}
		gapSeconds := toFloat(result["gapSeconds"])

		message := fmt.Sprintf(
			"Regla de velocidad '%s': dos transacciones de %s='%s' separadas por %.0f segundos (máximo configurado: %s)",
			rule.Name, cond.GroupBy, groupKey, gapSeconds, cond.MaxGap,
		)
		if dateLabel != "" {
			message += " [" + dateLabel + "]"
		}

		redFlags = append(redFlags, models.RedFlag{
			Fingerprint:  models.AggRedFlagFingerprint(rule.ID, monitor.ID, day, groupKey+"|"+fmt.Sprintf("%v", result["_id"])),
			MonitorID:    monitor.ID,
			RuleID:       rule.ID,
			RuleName:     rule.Name,
			MonitorName:  monitor.Name,
			Severity:     rule.Severity,
			RedFlagType:  models.RedFlagTypeVelocity,
			GroupByField: cond.GroupBy,
			GroupByValue: groupKey,
			Message:      message,
			MatchedData:  result,
			MatchCount:   cond.MinEvents,
		})
	}
	return redFlags
}

// buildAggregatePipeline creates a MongoDB aggregation pipeline for an AggregateCondition.
// Pipeline: $match (time window) → $group (by groupBy, apply agg func) → $match (threshold) → $limit
func buildAggregatePipeline(cond models.AggregateCondition) mongo.Pipeline {
	pipeline := mongo.Pipeline{}

	// Step 1: Filter by time window if configured
	if cond.TimeField != "" && cond.TimeWindow != "" {
		dur := parseTimeWindow(cond.TimeWindow)
		if dur > 0 {
			// El cutoff va como time.Time (fecha BSON), no como string
			// formateado: Mongo no compara entre tipos BSON distintos, así
			// que un campo Date contra un string no matchea NADA y la regla
			// nunca se dispara. Además el formato viejo truncaba al minuto,
			// haciendo inexpresable una ventana de 35s.
			cutoff := time.Now().Add(-dur)
			pipeline = append(pipeline, bson.D{
				{Key: "$match", Value: bson.D{
					{Key: cond.TimeField, Value: bson.D{
						{Key: "$gte", Value: cutoff},
					}},
				}},
			})
		}
	}

	// Step 1.5: Filter records before grouping (correlación filtrada —
	// ej. estructuración: solo montos en la franja, agregados por cuenta).
	// Aditivo: sin Filter, el pipeline queda exactamente igual que antes.
	if len(cond.Filter) > 0 {
		pipeline = append(pipeline, bson.D{
			{Key: "$match", Value: BuildMongoFilter(models.ConditionGroup{
				Logic: models.LogicAND, Conditions: cond.Filter,
			})},
		})
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
			e.triggerScreening(candidate, rule, isNew)
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

	// Evaluate velocity conditions with date-scoped pipeline. Sin este
	// bloque los tres consumidores de matchRuleForDateRange (scheduler,
	// "ejecutar ahora" y backtest) ignoraban las condiciones de velocidad en
	// silencio: la regla se guardaba, corría, devolvía cero y no fallaba.
	for _, velCond := range rule.VelocityConditions {
		velRedFlags, err := e.evaluateVelocityConditionDateScoped(ctx, monitor, rule, velCond, dayStart, dayEnd)
		if err != nil {
			log.Printf("ERROR velocity condition (date-scoped) for rule %s field=%s: %v", rule.ID.Hex(), velCond.TimeField, err)
			continue
		}
		candidates = append(candidates, velRedFlags...)
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
			// Mismo criterio que buildAggregatePipeline: fecha BSON, no string.
			cutoff := dayEnd.Add(-dur)
			pipeline = append(pipeline, bson.D{
				{Key: "$match", Value: bson.D{
					{Key: cond.TimeField, Value: bson.D{
						{Key: "$gte", Value: cutoff},
					}},
				}},
			})
		}
	}

	// Filter records before grouping (correlación filtrada) — mismo
	// tratamiento que buildAggregatePipeline, aditivo.
	if len(cond.Filter) > 0 {
		pipeline = append(pipeline, bson.D{
			{Key: "$match", Value: BuildMongoFilter(models.ConditionGroup{
				Logic: models.LogicAND, Conditions: cond.Filter,
			})},
		})
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

// buildVelocityPipeline arma el pipeline que detecta eventos consecutivos
// demasiado próximos dentro de una misma partición, sobre toda la historia
// de la colección.
// Pipeline: $match (guarda de tipo) → [$match filtro] → $sort →
// $setWindowFields (evento anterior) → $addFields (diferencia en segundos) →
// $match (pares por debajo del gap) → $sort (gap ascendente) → $limit.
// Validado contra MongoDB 7.0.40: $setWindowFields, $shift y $dateDiff
// ejecutan correctamente sobre la colección real del monitor.
func buildVelocityPipeline(cond models.VelocityCondition) mongo.Pipeline {
	return velocityPipelineStages(cond, nil)
}

// buildVelocityPipelineDateScoped es la variante acotada a los documentos
// ingeridos en [dayStart, dayEnd), hermana de
// buildAggregatePipelineDateScoped. La usan el scheduler, "ejecutar ahora" y
// el backtest.
//
// OJO: acotar por _ingested_at acota la ventana en la que se FORMAN los
// pares, no solo el conjunto de resultados. El evento anterior de cada par
// tiene que haber sido ingerido dentro del mismo rango: en un monitor de
// archivo diario, la última transacción del archivo de ayer y la primera del
// de hoy nunca se emparejan por este camino. Es el mismo criterio que el de
// agregados (que también agrupa solo lo ingerido en el rango); el camino de
// ingesta —EvaluateRules→buildVelocityPipeline— sí ve la historia completa.
func buildVelocityPipelineDateScoped(cond models.VelocityCondition, dayStart, dayEnd time.Time) mongo.Pipeline {
	return velocityPipelineStages(cond, mongo.Pipeline{
		{{Key: "$match", Value: bson.D{
			{Key: "_ingested_at", Value: bson.D{
				{Key: "$gte", Value: dayStart},
				{Key: "$lt", Value: dayEnd},
			}},
		}}},
	})
}

// velocityPipelineStages arma el pipeline de velocidad, insertando los
// stages de acotamiento que le pase el llamador DESPUÉS de que los pares se
// formaron. Las dos variantes comparten todo lo demás.
//
// El acotamiento va después y no antes a propósito: $setWindowFields solo ve
// los documentos que le llegan, así que filtrar primero por _ingested_at deja
// fuera el evento anterior de todo par que cruce dos lotes de ingesta — en un
// monitor de archivo diario, la última transacción de ayer y la primera de hoy
// nunca se emparejarían y esa alerta se perdería en silencio. Formando los
// pares sobre la historia completa y recién después descartando los que caen
// fuera de la ventana, se reporta lo que corresponde sin re-alertar el pasado.
func velocityPipelineStages(cond models.VelocityCondition, scope mongo.Pipeline) mongo.Pipeline {
	// Guarda de tipo, SIEMPRE el primer stage. Dos razones:
	//  1. $sort ubica los documentos sin el campo de tiempo al principio de
	//     cada partición, así que la primera transacción que sí lo tiene
	//     tomaría su prevTime de una que no y quedaría descartada — cada
	//     tarjeta perdía su primer evento real.
	//  2. Si el campo llega como string en algunos documentos, $dateDiff los
	//     mezclaría con las fechas: la vía de reentrada exacta del bug que
	//     dejó la ventana temporal comparando Date contra string.
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.D{
			{Key: cond.TimeField, Value: bson.D{{Key: "$type", Value: "date"}}},
		}}},
	}

	if len(cond.Filter) > 0 {
		pipeline = append(pipeline, bson.D{
			{Key: "$match", Value: BuildMongoFilter(models.ConditionGroup{
				Logic: models.LogicAND, Conditions: cond.Filter,
			})},
		})
	}

	sortKeys := bson.D{}
	if cond.GroupBy != "" {
		sortKeys = append(sortKeys, bson.E{Key: cond.GroupBy, Value: 1})
	}
	sortKeys = append(sortKeys, bson.E{Key: cond.TimeField, Value: 1})
	pipeline = append(pipeline, bson.D{{Key: "$sort", Value: sortKeys}})

	var partitionBy interface{}
	if cond.GroupBy != "" {
		partitionBy = "$" + cond.GroupBy
	}
	pipeline = append(pipeline, bson.D{
		{Key: "$setWindowFields", Value: bson.D{
			{Key: "partitionBy", Value: partitionBy},
			{Key: "sortBy", Value: bson.D{{Key: cond.TimeField, Value: 1}}},
			{Key: "output", Value: bson.D{
				{Key: "prevTime", Value: bson.D{
					{Key: "$shift", Value: bson.D{
						{Key: "output", Value: "$" + cond.TimeField},
						{Key: "by", Value: -1},
					}},
				}},
			}},
		}},
	})

	pipeline = append(pipeline, bson.D{
		{Key: "$addFields", Value: bson.D{
			{Key: "gapSeconds", Value: bson.D{
				{Key: "$dateDiff", Value: bson.D{
					{Key: "startDate", Value: "$prevTime"},
					{Key: "endDate", Value: "$" + cond.TimeField},
					{Key: "unit", Value: "second"},
				}},
			}},
		}},
	})

	// Recién acá se acota: los pares ya están formados, así que descartar por
	// ventana ahora reduce qué se reporta sin impedir que un par cruce lotes.
	pipeline = append(pipeline, scope...)

	maxGapSeconds := int64(parseTimeWindow(cond.MaxGap) / time.Second)
	pipeline = append(pipeline, bson.D{
		{Key: "$match", Value: bson.D{
			{Key: "prevTime", Value: bson.D{{Key: "$ne", Value: nil}}},
			{Key: "gapSeconds", Value: bson.D{{Key: "$lte", Value: maxGapSeconds}}},
		}},
	})

	// Orden explícito antes del tope: sin él, el $limit se quedaba con los
	// 50 pares que quedaran del $sort inicial (grupo alfabéticamente menor,
	// evento más viejo), re-alertando siempre los mismos y ocultando el
	// resto para siempre. Ascendente por gap = primero los peores casos.
	pipeline = append(pipeline, bson.D{
		{Key: "$sort", Value: bson.D{{Key: "gapSeconds", Value: 1}}},
	})

	// Mismo tope que buildAggregatePipeline: sin límite, un monitor con
	// tráfico alto podría emitir miles de banderas rojas de una sola corrida.
	pipeline = append(pipeline, bson.D{
		{Key: "$limit", Value: 50},
	})

	return pipeline
}

// parseTimeWindow converts a time window string like "60s", "5min", "24h",
// "7d", "30m" to a time.Duration. "min" is checked as a full suffix before
// falling back to the last-character unit so it never collides with "m"
// (months) — an existing rule's "30m" keeps meaning 30 months.
func parseTimeWindow(window string) time.Duration {
	window = strings.TrimSpace(strings.ToLower(window))
	if len(window) < 2 {
		return 0
	}

	if strings.HasSuffix(window, "min") {
		num, err := strconv.Atoi(window[:len(window)-3])
		if err != nil {
			return 0
		}
		return time.Duration(num) * time.Minute
	}

	unit := window[len(window)-1:]
	numStr := window[:len(window)-1]
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return 0
	}

	switch unit {
	case "s":
		return time.Duration(num) * time.Second
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
