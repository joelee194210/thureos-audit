package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
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
	// $match inicial = guarda de tipo date; $sort final = orden por gap
	// antes del tope.
	assertStages(t, pipelineStageKeys(pipeline), []string{"$match", "$sort", "$setWindowFields", "$addFields", "$match", "$sort", "$limit"})
}

func TestBuildVelocityPipelineDateScoped_Forma(t *testing.T) {
	dayStart := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	pipeline := buildVelocityPipelineDateScoped(baseVelocityCond(), dayStart, dayStart.Add(24*time.Hour))
	// Igual que la variante sin acotar, con el $match de _ingested_at
	// insertado justo después del guarda de tipo.
	assertStages(t, pipelineStageKeys(pipeline), []string{"$match", "$match", "$sort", "$setWindowFields", "$addFields", "$match", "$sort", "$limit"})
}

func TestBuildVelocityPipeline_ConFiltroAgregaMatchAlPrincipio(t *testing.T) {
	cond := baseVelocityCond()
	cond.Filter = []models.Condition{
		{Field: "importe", Operator: models.OpGreaterThan, Value: 5000.0},
		{Field: "mcc", Operator: models.OpEqual, Value: 7995.0},
	}
	pipeline := buildVelocityPipeline(cond)
	assertStages(t, pipelineStageKeys(pipeline), []string{"$match", "$match", "$sort", "$setWindowFields", "$addFields", "$match", "$sort", "$limit"})

	// El filtro del usuario va segundo: el primero es el guarda de tipo.
	wantFilter := BuildMongoFilter(models.ConditionGroup{Logic: models.LogicAND, Conditions: cond.Filter})
	gotFilter := stageOperand(t, pipeline[1], "$match")
	if fmt.Sprintf("%v", gotFilter) != fmt.Sprintf("%v", wantFilter) {
		t.Errorf("el $match posterior al guarda debería ser el filtro previo\ngot:  %v\nwant: %v", gotFilter, wantFilter)
	}
}

// gapMatchStage busca el $match que filtra por gapSeconds, sin depender de
// su posición: indexar por número obligaba a renumerar cada test cada vez
// que se agregaba un stage al pipeline.
func gapMatchStage(t *testing.T, pipeline mongo.Pipeline) bson.M {
	t.Helper()
	for _, stage := range pipeline {
		if stage[0].Key != "$match" {
			continue
		}
		operand := stageOperand(t, stage, "$match")
		if _, ok := operand["gapSeconds"]; ok {
			return operand
		}
	}
	t.Fatalf("el pipeline no tiene un $match sobre gapSeconds: %v", pipelineStageKeys(pipeline))
	return nil
}

// El $match final compara contra un número de segundos, no contra un string
// ni una fecha: es la diferencia ya calculada por $dateDiff.
func TestBuildVelocityPipeline_ComparaGapEnSegundos(t *testing.T) {
	pipeline := buildVelocityPipeline(baseVelocityCond())
	final := gapMatchStage(t, pipeline)

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
	final := gapMatchStage(t, pipeline)
	if _, ok := final["prevTime"]; !ok {
		t.Error("el $match final debería descartar los documentos sin evento anterior (prevTime null)")
	}
}

// assertGuardaDeTipoDate verifica que el PRIMER stage del pipeline descarte
// los documentos cuyo campo de tiempo no es una fecha BSON. Dos motivos:
// (1) $sort pone los documentos sin el campo al principio de cada partición,
// así que el primer documento real tomaría su prevTime de uno vacío y
// quedaría descartado; (2) si el campo llega como string en algunos
// documentos, $dateDiff los mezclaría — la vía de reentrada exacta del bug
// histórico de comparación entre tipos BSON.
func assertGuardaDeTipoDate(t *testing.T, pipeline mongo.Pipeline, timeField string) {
	t.Helper()
	if len(pipeline) == 0 {
		t.Fatal("pipeline vacío")
	}
	primero := stageOperand(t, pipeline[0], "$match")
	cond, ok := primero[timeField].(bson.M)
	if !ok {
		t.Fatalf("el primer stage debería ser el guarda de tipo sobre %q, got %#v", timeField, primero)
	}
	if cond["$type"] != "date" {
		t.Errorf("guarda de tipo = %#v, want {$type: \"date\"}", cond)
	}
}

func TestBuildVelocityPipeline_GuardaDeTipoDateEsElPrimerStage(t *testing.T) {
	assertGuardaDeTipoDate(t, buildVelocityPipeline(baseVelocityCond()), "timestamp")

	// También cuando hay filtro de usuario: el guarda va antes.
	cond := baseVelocityCond()
	cond.Filter = []models.Condition{{Field: "importe", Operator: models.OpGreaterThan, Value: 5000.0}}
	assertGuardaDeTipoDate(t, buildVelocityPipeline(cond), "timestamp")
}

func TestBuildVelocityPipelineDateScoped_GuardaDeTipoDateEsElPrimerStage(t *testing.T) {
	dayStart := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	assertGuardaDeTipoDate(t, buildVelocityPipelineDateScoped(baseVelocityCond(), dayStart, dayStart.Add(24*time.Hour)), "timestamp")
}

// Sin orden explícito, el $limit se quedaba con los 50 pares más viejos /
// alfabéticamente menores, re-alertando siempre los mismos y ocultando el
// resto. Los sobrevivientes tienen que ser los peores: los de gap más chico.
func assertOrdenPorGapAntesDelLimite(t *testing.T, pipeline mongo.Pipeline) {
	t.Helper()
	keys := pipelineStageKeys(pipeline)
	if keys[len(keys)-1] != "$limit" {
		t.Fatalf("el último stage debería ser $limit, got %v", keys)
	}
	sortStage := pipeline[len(pipeline)-2]
	if sortStage[0].Key != "$sort" {
		t.Fatalf("el stage anterior a $limit debería ser $sort, got %v", keys)
	}
	operand := stageOperand(t, sortStage, "$sort")
	if operand["gapSeconds"] == nil {
		t.Fatalf("el $sort previo al $limit debería ordenar por gapSeconds, got %#v", operand)
	}
	if fmt.Sprintf("%v", operand["gapSeconds"]) != "1" {
		t.Errorf("gapSeconds = %v, want 1 (ascendente: primero los gaps más chicos)", operand["gapSeconds"])
	}
}

func TestBuildVelocityPipeline_OrdenaPorGapAscendenteAntesDelLimite(t *testing.T) {
	assertOrdenPorGapAntesDelLimite(t, buildVelocityPipeline(baseVelocityCond()))
}

func TestBuildVelocityPipelineDateScoped_OrdenaPorGapAscendenteAntesDelLimite(t *testing.T) {
	dayStart := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	assertOrdenPorGapAntesDelLimite(t, buildVelocityPipelineDateScoped(baseVelocityCond(), dayStart, dayStart.Add(24*time.Hour)))
}

// El pipeline date-scoped acota por _ingested_at, igual que el de agregados.
func TestBuildVelocityPipelineDateScoped_AcotaPorIngestedAt(t *testing.T) {
	dayStart := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.Add(24 * time.Hour)
	pipeline := buildVelocityPipelineDateScoped(baseVelocityCond(), dayStart, dayEnd)

	bound := ingestedAtBound(t, pipeline)
	if bound == nil {
		t.Fatalf("el pipeline date-scoped debería acotar por _ingested_at: %v", pipelineStageKeys(pipeline))
	}
	gte, ok := bound["$gte"].(primitive.DateTime)
	if !ok {
		t.Fatalf("$gte debería ser una fecha BSON, got %T", bound["$gte"])
	}
	if !gte.Time().UTC().Equal(dayStart) {
		t.Errorf("$gte = %v, want %v", gte.Time().UTC(), dayStart)
	}
	lt, ok := bound["$lt"].(primitive.DateTime)
	if !ok {
		t.Fatalf("$lt debería ser una fecha BSON, got %T", bound["$lt"])
	}
	if !lt.Time().UTC().Equal(dayEnd) {
		t.Errorf("$lt = %v, want %v", lt.Time().UTC(), dayEnd)
	}
}

// ingestedAtBound devuelve la condición sobre _ingested_at de cualquier
// $match del pipeline, o nil si no hay ninguna.
func ingestedAtBound(t *testing.T, pipeline mongo.Pipeline) bson.M {
	t.Helper()
	for _, stage := range pipeline {
		if stage[0].Key != "$match" {
			continue
		}
		operand := stageOperand(t, stage, "$match")
		if cond, ok := operand["_ingested_at"].(bson.M); ok {
			return cond
		}
	}
	return nil
}

// fakeMonitorData reemplaza a MonitorRepository en los tests: registra los
// pipelines que recibe y devuelve resultados preparados.
type fakeMonitorData struct {
	pipelines []mongo.Pipeline
	results   []bson.M
	queries   int
}

func (f *fakeMonitorData) QueryData(_ context.Context, _ string, _ bson.M, _ int64) ([]bson.M, error) {
	f.queries++
	return nil, nil
}

func (f *fakeMonitorData) AggregateData(_ context.Context, _ string, pipeline mongo.Pipeline) ([]bson.M, error) {
	f.pipelines = append(f.pipelines, pipeline)
	return f.results, nil
}

func reglaDeVelocidad() (*models.Monitor, models.Rule) {
	monitor := &models.Monitor{
		ID:           primitive.NewObjectID(),
		Name:         "polizas",
		CollectionID: "data_polizas",
	}
	rule := models.Rule{
		ID:                 primitive.NewObjectID(),
		Name:               "dos compras en menos de 35s",
		Severity:           models.SeverityHigh,
		VelocityConditions: []models.VelocityCondition{baseVelocityCond()},
	}
	return monitor, rule
}

// C2: las condiciones de velocidad solo se evaluaban en EvaluateRules (la
// ingesta). El scheduler, "ejecutar ahora" y el backtest pasan todos por
// matchRuleForDateRange, que las ignoraba en silencio: la regla se guardaba,
// corría, devolvía cero y nunca fallaba. Este test falla si alguien vuelve a
// sacar la velocidad del camino date-scoped.
func TestMatchRuleForDateRange_EvaluaCondicionesDeVelocidad(t *testing.T) {
	fake := &fakeMonitorData{results: []bson.M{
		{"_id": "doc-1", "tarjeta": "T4111", "gapSeconds": int64(12)},
	}}
	engine := &RuleEngine{monitorRepo: fake}
	monitor, rule := reglaDeVelocidad()
	dayStart := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)

	// BacktestRule es el consumidor puro de matchRuleForDateRange (sin
	// escrituras): EvaluateRuleForDateRange y EvaluateRuleNow lo comparten.
	flags := engine.BacktestRule(context.Background(), monitor, rule, dayStart, dayStart.Add(24*time.Hour))

	if len(flags) != 1 {
		t.Fatalf("got %d banderas rojas, want 1 — la velocidad no se está evaluando en el camino date-scoped", len(flags))
	}
	if flags[0].GroupByValue != "T4111" {
		t.Errorf("GroupByValue = %q, want \"T4111\"", flags[0].GroupByValue)
	}
	if flags[0].RuleID != rule.ID || flags[0].MonitorID != monitor.ID {
		t.Error("la bandera roja no quedó atada a la regla y al monitor")
	}
	// El fingerprint tiene que anclarse al día evaluado, no a time.Now():
	// si no, un backlog de varios días del scheduler colapsa en uno solo.
	if want := models.AggRedFlagFingerprint(rule.ID, monitor.ID, dayStart, "T4111|doc-1"); flags[0].Fingerprint != want {
		t.Errorf("Fingerprint = %q, want %q (anclado a dayStart)", flags[0].Fingerprint, want)
	}

	// Y el pipeline que se ejecutó tiene que estar acotado al rango: sin
	// esto, cablear la velocidad sin scoping también pasaría el test.
	if len(fake.pipelines) != 1 {
		t.Fatalf("got %d pipelines ejecutados, want 1", len(fake.pipelines))
	}
	if ingestedAtBound(t, fake.pipelines[0]) == nil {
		t.Error("el pipeline ejecutado no está acotado por _ingested_at")
	}
}

// Aditividad: una regla sin condiciones de velocidad se comporta igual que
// antes de esta rama — no ejecuta ningún pipeline extra.
func TestMatchRuleForDateRange_SinVelocidadNoEjecutaPipelines(t *testing.T) {
	fake := &fakeMonitorData{}
	engine := &RuleEngine{monitorRepo: fake}
	monitor := &models.Monitor{ID: primitive.NewObjectID(), CollectionID: "data_polizas"}
	rule := models.Rule{
		ID: primitive.NewObjectID(),
		ConditionGroup: models.ConditionGroup{
			Logic:      models.LogicAND,
			Conditions: []models.Condition{{Field: "importe", Operator: models.OpGreaterThan, Value: 10000.0}},
		},
	}
	dayStart := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)

	flags := engine.BacktestRule(context.Background(), monitor, rule, dayStart, dayStart.Add(24*time.Hour))

	if len(flags) != 0 {
		t.Errorf("got %d banderas rojas, want 0", len(flags))
	}
	if len(fake.pipelines) != 0 {
		t.Errorf("una regla sin agregados ni velocidad no debería ejecutar pipelines, got %d", len(fake.pipelines))
	}
	if fake.queries != 1 {
		t.Errorf("la consulta fila a fila debería seguir ejecutándose una vez, got %d", fake.queries)
	}
}
