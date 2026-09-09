package services

import (
	"testing"

	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func baseChatAggregateSpec() ChatAggregateSpec {
	return ChatAggregateSpec{
		Field:    "amount",
		Function: models.AggFuncSum,
		GroupBy:  "account",
	}
}

func TestBuildDataAggregatePipeline_SinFiltroNiVentana(t *testing.T) {
	pipeline := buildDataAggregatePipeline(baseChatAggregateSpec())
	assertStages(t, pipelineStageKeys(pipeline), []string{"$group", "$sort", "$limit"})
}

// A diferencia de buildAggregatePipeline, nunca hay un $match después del
// $group — no existe el concepto de threshold acá.
func TestBuildDataAggregatePipeline_NuncaFiltraPorThreshold(t *testing.T) {
	pipeline := buildDataAggregatePipeline(baseChatAggregateSpec())
	for i, stage := range pipeline {
		if i == 0 {
			continue // el primer stage puede ser el $match del Filter opcional
		}
		if stage[0].Key == "$match" {
			t.Fatalf("no debería haber un $match después del primer stage, encontrado en la posición %d: %v", i, stage)
		}
	}
}

func TestBuildDataAggregatePipeline_ConFiltroAgregaMatchAntesDelGroup(t *testing.T) {
	spec := baseChatAggregateSpec()
	spec.Filter = []models.Condition{
		{Field: "amount", Operator: models.OpGreaterEqual, Value: 1000.0},
		{Field: "amount", Operator: models.OpLessThan, Value: 10000.0},
	}
	pipeline := buildDataAggregatePipeline(spec)
	assertStages(t, pipelineStageKeys(pipeline), []string{"$match", "$group", "$sort", "$limit"})

	wantFilter := BuildMongoFilter(models.ConditionGroup{Logic: models.LogicAND, Conditions: spec.Filter})
	gotFilter := stageOperand(t, pipeline[0], "$match")
	wantBytes, _ := bson.Marshal(wantFilter)
	gotBytes, _ := bson.Marshal(gotFilter)
	if string(wantBytes) != string(gotBytes) {
		t.Errorf("contenido del $match no coincide con BuildMongoFilter:\n got=%v\nwant=%v", gotFilter, wantFilter)
	}
}

func TestBuildDataAggregatePipeline_ConVentanaDeTiempoAgregaMatchDeFecha(t *testing.T) {
	spec := baseChatAggregateSpec()
	spec.TimeField = "fecha"
	spec.TimeWindow = "30d"
	pipeline := buildDataAggregatePipeline(spec)
	assertStages(t, pipelineStageKeys(pipeline), []string{"$match", "$group", "$sort", "$limit"})

	matchValue := stageOperand(t, pipeline[0], "$match")
	if _, ok := matchValue["fecha"]; !ok {
		t.Errorf("el $match de ventana de tiempo debería filtrar por 'fecha', got %+v", matchValue)
	}
}

func TestBuildDataAggregatePipeline_SinGroupByAgrupaTodoJunto(t *testing.T) {
	spec := baseChatAggregateSpec()
	spec.GroupBy = ""
	pipeline := buildDataAggregatePipeline(spec)
	groupValue := stageOperand(t, pipeline[0], "$group")
	if groupValue["_id"] != nil {
		t.Errorf("_id del $group sin GroupBy debería ser nil, got %v", groupValue["_id"])
	}
}

func TestBuildDataAggregatePipeline_RespetaTopeDeGrupos(t *testing.T) {
	pipeline := buildDataAggregatePipeline(baseChatAggregateSpec())
	last := pipeline[len(pipeline)-1]
	if last[0].Key != "$limit" {
		t.Fatalf("el último stage debería ser $limit, got %q", last[0].Key)
	}
	if last[0].Value != chatAggregateResultLimit {
		t.Errorf("$limit = %v, want %d", last[0].Value, chatAggregateResultLimit)
	}
}

// Mismo bug que en rule_engine: el cutoff viajaba como string formateado y
// Mongo no compara entre tipos BSON distintos, así que el chatbot no filtraba
// por ventana de tiempo — devolvía cero filas en vez de las de la ventana.
func TestBuildDataAggregatePipeline_VentanaDeTiempoUsaFechaNoString(t *testing.T) {
	spec := baseChatAggregateSpec()
	spec.TimeField = "fecha"
	spec.TimeWindow = "35s"

	pipeline := buildDataAggregatePipeline(spec)

	windowMatch := stageOperand(t, pipeline[0], "$match")
	fieldCond, ok := windowMatch["fecha"].(bson.M)
	if !ok {
		t.Fatalf("esperaba un operando bson.M para 'fecha', got %#v", windowMatch["fecha"])
	}
	if _, isString := fieldCond["$gte"].(string); isString {
		t.Fatalf("$gte llegó como string (%#v) — Mongo no compara Date contra string", fieldCond["$gte"])
	}
	if _, isDate := fieldCond["$gte"].(primitive.DateTime); !isDate {
		t.Fatalf("$gte debería serializar como fecha BSON, got %T", fieldCond["$gte"])
	}
}
