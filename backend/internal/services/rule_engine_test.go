package services

import (
	"testing"
	"time"

	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson"
)

// pipelineStageKeys returns each pipeline stage's operator ("$match",
// "$group", ...) in order — enough to assert shape without hand-decoding
// every bson.D.
func pipelineStageKeys(pipeline []bson.D) []string {
	keys := make([]string, len(pipeline))
	for i, stage := range pipeline {
		keys[i] = stage[0].Key
	}
	return keys
}

// stageOperand round-trips a stage's operand through BSON into a bson.M —
// bson.D.Map() is deprecated (removed in Go Driver 2.0), and the operand
// can be either a bson.D or a bson.M depending on which stage built it.
func stageOperand(t *testing.T, stage bson.D, key string) bson.M {
	t.Helper()
	raw, err := bson.Marshal(stage)
	if err != nil {
		t.Fatalf("marshaling stage: %v", err)
	}
	var decoded bson.M
	if err := bson.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshaling stage: %v", err)
	}
	operand, ok := decoded[key].(bson.M)
	if !ok {
		t.Fatalf("stage %v has no bson.M operand under %q", stage, key)
	}
	return operand
}

func baseAggCondition() models.AggregateCondition {
	return models.AggregateCondition{
		Field:     "amount",
		Function:  models.AggFuncSum,
		GroupBy:   "account",
		Operator:  models.OpGreaterThan,
		Threshold: 10000,
	}
}

func assertStages(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("stages = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("stage %d = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

// Sin Filter, el pipeline debe quedar exactamente como antes de agregar el
// soporte de correlación filtrada — regresión explícita.
func TestBuildAggregatePipeline_SinFiltroNoAgregaMatchExtra(t *testing.T) {
	pipeline := buildAggregatePipeline(baseAggCondition())
	assertStages(t, pipelineStageKeys(pipeline), []string{"$group", "$match", "$sort", "$limit"})

	// El único $match presente es el del threshold, no un filtro previo.
	matchValue := stageOperand(t, pipeline[1], "$match")
	if _, ok := matchValue["aggValue"]; !ok {
		t.Errorf("el $match sin Filter debería ser el de threshold (aggValue), got %+v", matchValue)
	}
}

// Con Filter, se inserta un $match ANTES del $group con las condiciones
// del filtro — el universo se reduce antes de agrupar.
func TestBuildAggregatePipeline_ConFiltroAgregaMatchAntesDelGroup(t *testing.T) {
	cond := baseAggCondition()
	cond.Filter = []models.Condition{
		{Field: "amount", Operator: models.OpGreaterEqual, Value: 1000.0},
		{Field: "amount", Operator: models.OpLessThan, Value: 10000.0},
	}
	pipeline := buildAggregatePipeline(cond)
	assertStages(t, pipelineStageKeys(pipeline), []string{"$match", "$group", "$match", "$sort", "$limit"})

	// El primer $match debe ser exactamente lo que BuildMongoFilter
	// produciría para las mismas condiciones (no el de threshold, que va
	// después del $group).
	wantFilter := BuildMongoFilter(models.ConditionGroup{Logic: models.LogicAND, Conditions: cond.Filter})
	gotFilter := stageOperand(t, pipeline[0], "$match")
	wantBytes, _ := bson.Marshal(wantFilter)
	gotBytes, _ := bson.Marshal(gotFilter)
	if string(wantBytes) != string(gotBytes) {
		t.Errorf("contenido del $match de filtro no coincide con BuildMongoFilter:\n got=%v\nwant=%v", gotFilter, wantFilter)
	}
}

func TestBuildAggregatePipelineDateScoped_SinFiltroNoAgregaMatchExtra(t *testing.T) {
	dayStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.Add(24 * time.Hour)
	pipeline := buildAggregatePipelineDateScoped(baseAggCondition(), dayStart, dayEnd)
	assertStages(t, pipelineStageKeys(pipeline), []string{"$match", "$group", "$match", "$sort", "$limit"})

	// El único $match antes del $group es el de rango de fecha, no un filtro.
	dateMatch := stageOperand(t, pipeline[0], "$match")
	if _, ok := dateMatch["_ingested_at"]; !ok {
		t.Errorf("primer $match debería ser el rango de fecha, got %+v", dateMatch)
	}
}

func TestBuildAggregatePipelineDateScoped_ConFiltroAgregaMatchEntreFechaYGroup(t *testing.T) {
	dayStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.Add(24 * time.Hour)
	cond := baseAggCondition()
	cond.Filter = []models.Condition{{Field: "amount", Operator: models.OpGreaterEqual, Value: 1000.0}}

	pipeline := buildAggregatePipelineDateScoped(cond, dayStart, dayEnd)
	assertStages(t, pipelineStageKeys(pipeline), []string{"$match", "$match", "$group", "$match", "$sort", "$limit"})

	// El primer $match sigue siendo el rango de fecha; el segundo es el filtro.
	dateMatch := stageOperand(t, pipeline[0], "$match")
	if _, ok := dateMatch["_ingested_at"]; !ok {
		t.Errorf("primer $match debería ser el rango de fecha, got %+v", dateMatch)
	}
	filterMatch := stageOperand(t, pipeline[1], "$match")
	if _, ok := filterMatch["amount"]; !ok {
		t.Errorf("segundo $match debería ser el filtro sobre 'amount', got %+v", filterMatch)
	}
}
