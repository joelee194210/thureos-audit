package services

import (
	"fmt"
	"testing"
	"time"

	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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

func TestParseTimeWindow_Seconds(t *testing.T) {
	got := parseTimeWindow("60s")
	want := 60 * time.Second
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseTimeWindow_Minutes(t *testing.T) {
	got := parseTimeWindow("5min")
	want := 5 * time.Minute
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseTimeWindow_MinutesDoesNotCollideWithMonths(t *testing.T) {
	// "min" siempre debe ganar sobre el sufijo de un solo char "m" (meses) —
	// "5min" no puede interpretarse como "5min" -> unidad "n" inválida, ni
	// como meses.
	got := parseTimeWindow("1min")
	want := 1 * time.Minute
	if got != want {
		t.Errorf("got %v, want %v (no debe confundirse con meses)", got, want)
	}
}

func TestParseTimeWindow_HoursUnchanged(t *testing.T) {
	got := parseTimeWindow("24h")
	want := 24 * time.Hour
	if got != want {
		t.Errorf("got %v, want %v (regresión)", got, want)
	}
}

func TestParseTimeWindow_DaysUnchanged(t *testing.T) {
	got := parseTimeWindow("7d")
	want := 7 * 24 * time.Hour
	if got != want {
		t.Errorf("got %v, want %v (regresión)", got, want)
	}
}

func TestParseTimeWindow_MonthsUnchanged(t *testing.T) {
	// "m" sigue significando meses — ninguna regla existente cambia de
	// significado con este cambio.
	got := parseTimeWindow("30m")
	want := time.Duration(30) * 30 * 24 * time.Hour
	if got != want {
		t.Errorf("got %v, want %v (regresión — 'm' debe seguir siendo meses)", got, want)
	}
}

func TestParseTimeWindow_InvalidUnitReturnsZero(t *testing.T) {
	got := parseTimeWindow("5x")
	if got != 0 {
		t.Errorf("got %v, want 0 para una unidad inválida", got)
	}
}

// La ventana de tiempo debe viajar a Mongo como fecha BSON, no como string.
// Antes se mandaba cutoff.Format("01/02/2006 03:04 PM"): Mongo no compara
// entre tipos BSON distintos, así que un campo Date contra un string devolvía
// CERO documentos y la regla nunca se disparaba. Ninguna de las dos ramas de
// ventana tenía cobertura — baseAggCondition no setea TimeField/TimeWindow.
func TestBuildAggregatePipeline_VentanaDeTiempoUsaFechaNoString(t *testing.T) {
	cond := baseAggCondition()
	cond.TimeField = "fecha"
	cond.TimeWindow = "35s"

	pipeline := buildAggregatePipeline(cond)
	assertStages(t, pipelineStageKeys(pipeline), []string{"$match", "$group", "$match", "$sort", "$limit"})

	windowMatch := stageOperand(t, pipeline[0], "$match")
	fieldCond, ok := windowMatch["fecha"].(bson.M)
	if !ok {
		t.Fatalf("esperaba un operando bson.M para 'fecha', got %#v", windowMatch["fecha"])
	}
	if _, isString := fieldCond["$gte"].(string); isString {
		t.Fatalf("$gte llegó como string (%#v) — Mongo no compara Date contra string y la regla nunca dispara", fieldCond["$gte"])
	}
	if _, isDate := fieldCond["$gte"].(primitive.DateTime); !isDate {
		t.Fatalf("$gte debería serializar como fecha BSON, got %T (%#v)", fieldCond["$gte"], fieldCond["$gte"])
	}
}

// Una ventana de 35s exige precisión de segundos: el formato viejo truncaba
// al minuto, volviendo "35 segundos" indistinguible de "este minuto".
func TestBuildAggregatePipeline_VentanaConservaPrecisionDeSegundos(t *testing.T) {
	cond := baseAggCondition()
	cond.TimeField = "fecha"
	cond.TimeWindow = "35s"

	antes := time.Now()
	pipeline := buildAggregatePipeline(cond)
	despues := time.Now()

	windowMatch := stageOperand(t, pipeline[0], "$match")
	fieldCond := windowMatch["fecha"].(bson.M)
	cutoff := fieldCond["$gte"].(primitive.DateTime).Time()

	minEsperado := antes.Add(-35 * time.Second).Add(-time.Second)
	maxEsperado := despues.Add(-35 * time.Second).Add(time.Second)
	if cutoff.Before(minEsperado) || cutoff.After(maxEsperado) {
		t.Errorf("cutoff %v fuera del rango esperado [%v, %v] para una ventana de 35s", cutoff, minEsperado, maxEsperado)
	}
}

func TestBuildAggregatePipelineDateScoped_VentanaDeTiempoUsaFechaNoString(t *testing.T) {
	dayStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.Add(24 * time.Hour)
	cond := baseAggCondition()
	cond.TimeField = "fecha"
	cond.TimeWindow = "35s"

	pipeline := buildAggregatePipelineDateScoped(cond, dayStart, dayEnd)

	windowMatch := stageOperand(t, pipeline[1], "$match")
	fieldCond, ok := windowMatch["fecha"].(bson.M)
	if !ok {
		t.Fatalf("esperaba un operando bson.M para 'fecha', got %#v", windowMatch["fecha"])
	}
	if _, isString := fieldCond["$gte"].(string); isString {
		t.Fatalf("$gte llegó como string (%#v) — Mongo no compara Date contra string", fieldCond["$gte"])
	}
	cutoff, isDate := fieldCond["$gte"].(primitive.DateTime)
	if !isDate {
		t.Fatalf("$gte debería serializar como fecha BSON, got %T", fieldCond["$gte"])
	}
	if want := dayEnd.Add(-35 * time.Second); !cutoff.Time().UTC().Equal(want.UTC()) {
		t.Errorf("cutoff = %v, want %v (dayEnd - 35s)", cutoff.Time().UTC(), want.UTC())
	}
}

func baseVelocityCond() models.VelocityCondition {
	return models.VelocityCondition{
		TimeField: "timestamp",
		MaxGap:    "35s",
		GroupBy:   "tarjeta",
		MinEvents: 2,
	}
}

// La forma del pipeline es el contrato: ordenar, calcular el evento
// anterior por partición, medir la diferencia y quedarse con los pares
// demasiado próximos.
func TestBuildVelocityPipeline_Forma(t *testing.T) {
	pipeline := buildVelocityPipeline(baseVelocityCond())
	assertStages(t, pipelineStageKeys(pipeline), []string{"$sort", "$setWindowFields", "$addFields", "$match"})
}

func TestBuildVelocityPipeline_ConFiltroAgregaMatchAlPrincipio(t *testing.T) {
	cond := baseVelocityCond()
	cond.Filter = []models.Condition{
		{Field: "importe", Operator: models.OpGreaterThan, Value: 5000.0},
		{Field: "mcc", Operator: models.OpEqual, Value: 7995.0},
	}
	pipeline := buildVelocityPipeline(cond)
	assertStages(t, pipelineStageKeys(pipeline), []string{"$match", "$sort", "$setWindowFields", "$addFields", "$match"})

	wantFilter := BuildMongoFilter(models.ConditionGroup{Logic: models.LogicAND, Conditions: cond.Filter})
	gotFilter := stageOperand(t, pipeline[0], "$match")
	if fmt.Sprintf("%v", gotFilter) != fmt.Sprintf("%v", wantFilter) {
		t.Errorf("el primer $match debería ser el filtro previo\ngot:  %v\nwant: %v", gotFilter, wantFilter)
	}
}

// El $match final compara contra un número de segundos, no contra un string
// ni una fecha: es la diferencia ya calculada por $dateDiff.
func TestBuildVelocityPipeline_ComparaGapEnSegundos(t *testing.T) {
	pipeline := buildVelocityPipeline(baseVelocityCond())
	final := stageOperand(t, pipeline[len(pipeline)-1], "$match")

	gapCond, ok := final["gapSeconds"].(bson.M)
	if !ok {
		t.Fatalf("esperaba una condición sobre gapSeconds, got %#v", final["gapSeconds"])
	}
	lte, ok := gapCond["$lte"]
	if !ok {
		t.Fatalf("esperaba $lte sobre gapSeconds, got %#v", gapCond)
	}
	if fmt.Sprintf("%v", lte) != "35" {
		t.Errorf("$lte = %v, want 35 (segundos de '35s')", lte)
	}
	// La comparación de valor por sí sola no distingue un número de un
	// string ("35" vs 35 imprimen igual) — que es exactamente el bug que
	// dejó una regla sin disparar durante meses. Se verifica el tipo BSON.
	if _, isNumber := lte.(int64); !isNumber {
		t.Errorf("$lte llegó como %T (%#v), quiere un número de segundos (int64)", lte, lte)
	}
}

// Sin evento anterior no hay gap que medir: la primera transacción de cada
// partición no puede disparar por sí sola.
func TestBuildVelocityPipeline_DescartaElPrimeroDeCadaParticion(t *testing.T) {
	pipeline := buildVelocityPipeline(baseVelocityCond())
	final := stageOperand(t, pipeline[len(pipeline)-1], "$match")
	if _, ok := final["prevTime"]; !ok {
		t.Error("el $match final debería descartar los documentos sin evento anterior (prevTime null)")
	}
}
