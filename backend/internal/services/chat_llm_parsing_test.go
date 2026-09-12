package services

import (
	"encoding/json"
	"testing"

	"github.com/thureos/compliance/internal/models"
)

func TestParseQueryToolInput_ConConditionGroupValido(t *testing.T) {
	raw := json.RawMessage(`{"monitor":"m1","conditionGroup":{"logic":"AND","conditions":[{"field":"amount","operator":"gt","value":1000}]}}`)
	input, err := ParseQueryToolInput(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if input.ConditionGroup == nil || len(input.ConditionGroup.Conditions) != 1 {
		t.Fatalf("conditionGroup no se parseó correctamente: %+v", input)
	}
	if input.Limit != chatQueryMaxLimit {
		t.Errorf("Limit sin especificar debería quedar en el tope %d, got %d", chatQueryMaxLimit, input.Limit)
	}
}

func TestParseQueryToolInput_ConAggregateValido(t *testing.T) {
	raw := json.RawMessage(`{"monitor":"m1","aggregate":{"field":"amount","function":"sum","groupBy":"account"}}`)
	input, err := ParseQueryToolInput(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if input.Aggregate == nil || input.Aggregate.Field != "amount" {
		t.Fatalf("aggregate no se parseó correctamente: %+v", input)
	}
}

func TestParseQueryToolInput_SinConditionGroupNiAggregateFalla(t *testing.T) {
	raw := json.RawMessage(`{"monitor":"m1"}`)
	if _, err := ParseQueryToolInput(raw); err == nil {
		t.Fatal("esperaba error cuando no hay conditionGroup ni aggregate")
	}
}

func TestParseQueryToolInput_AggregateSinFieldFalla(t *testing.T) {
	raw := json.RawMessage(`{"monitor":"m1","aggregate":{"function":"sum"}}`)
	if _, err := ParseQueryToolInput(raw); err == nil {
		t.Fatal("esperaba error cuando aggregate.field está vacío")
	}
}

func TestParseQueryToolInput_LimitFueraDeRangoSeAjustaAlTope(t *testing.T) {
	raw := json.RawMessage(`{"monitor":"m1","conditionGroup":{"logic":"AND","conditions":[]},"limit":999999}`)
	input, err := ParseQueryToolInput(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if input.Limit != chatQueryMaxLimit {
		t.Errorf("Limit fuera de rango debería ajustarse a %d, got %d", chatQueryMaxLimit, input.Limit)
	}
}

func TestParseQueryToolInput_JSONInvalidoFalla(t *testing.T) {
	if _, err := ParseQueryToolInput(json.RawMessage(`{not json`)); err == nil {
		t.Fatal("esperaba error con JSON inválido")
	}
}

func TestParseArtifact_VacioDevuelveNilSinError(t *testing.T) {
	artifact, err := ParseArtifact(nil)
	if err != nil || artifact != nil {
		t.Fatalf("esperaba (nil, nil) para input vacío, got (%v, %v)", artifact, err)
	}
}

func TestParseArtifact_ChartValido(t *testing.T) {
	raw := json.RawMessage(`{"type":"chart","title":"Ventas por mes","chartSpec":{"chartType":"bar","data":[{"mes":"enero","total":100}],"xKey":"mes","yKeys":["total"]}}`)
	artifact, err := ParseArtifact(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if artifact.Type != models.ChatArtifactChart || artifact.ChartSpec.ChartType != models.ChatChartBar {
		t.Fatalf("artifact no se parseó correctamente: %+v", artifact)
	}
}

func TestParseArtifact_ChartSinDataFalla(t *testing.T) {
	raw := json.RawMessage(`{"type":"chart","chartSpec":{"chartType":"bar","data":[]}}`)
	if _, err := ParseArtifact(raw); err == nil {
		t.Fatal("esperaba error cuando chartSpec.data está vacío")
	}
}

func TestParseArtifact_ChartTypeInvalidoFalla(t *testing.T) {
	raw := json.RawMessage(`{"type":"chart","chartSpec":{"chartType":"scatter3d","data":[{"a":1}]}}`)
	if _, err := ParseArtifact(raw); err == nil {
		t.Fatal("esperaba error con chartType desconocido")
	}
}

func TestParseArtifact_TableValida(t *testing.T) {
	raw := json.RawMessage(`{"type":"table","title":"Detalle","chartSpec":{"data":[{"cliente":"Juan","total":500}]}}`)
	artifact, err := ParseArtifact(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if artifact.Type != models.ChatArtifactTable {
		t.Fatalf("Type = %q, want table", artifact.Type)
	}
}

func TestParseArtifact_CustomValido(t *testing.T) {
	raw := json.RawMessage(`{"type":"custom","title":"Mapa de calor","code":"<html><body>hola</body></html>"}`)
	artifact, err := ParseArtifact(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if artifact.Code == "" {
		t.Fatal("Code no debería quedar vacío")
	}
}

func TestParseArtifact_CustomSinCodeFalla(t *testing.T) {
	raw := json.RawMessage(`{"type":"custom","title":"Mapa de calor"}`)
	if _, err := ParseArtifact(raw); err == nil {
		t.Fatal("esperaba error cuando custom no trae code")
	}
}

func TestParseArtifact_TipoDesconocidoFalla(t *testing.T) {
	raw := json.RawMessage(`{"type":"video"}`)
	if _, err := ParseArtifact(raw); err == nil {
		t.Fatal("esperaba error con type desconocido")
	}
}

func TestParseArtifact_JSONInvalidoFalla(t *testing.T) {
	if _, err := ParseArtifact(json.RawMessage(`{not json`)); err == nil {
		t.Fatal("esperaba error con JSON inválido")
	}
}

func TestParseQueryToolInput_ExigeMonitor(t *testing.T) {
	raw := []byte(`{"aggregate": {"field": "monto", "function": "sum"}}`)
	if _, err := ParseQueryToolInput(raw); err == nil {
		t.Fatal("un input sin 'monitor' debe ser rechazado")
	}
}

func TestParseQueryToolInput_RechazaMonitorVacio(t *testing.T) {
	raw := []byte(`{"monitor": "   ", "aggregate": {"field": "monto", "function": "sum"}}`)
	if _, err := ParseQueryToolInput(raw); err == nil {
		t.Fatal("un 'monitor' en blanco debe ser rechazado")
	}
}

func TestParseQueryToolInput_ConservaYNormalizaElMonitor(t *testing.T) {
	raw := []byte(`{"monitor": "  transacciones  ", "aggregate": {"field": "monto", "function": "sum"}}`)
	input, err := ParseQueryToolInput(raw)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if input.Monitor != "transacciones" {
		t.Errorf("Monitor = %q, quiero %q", input.Monitor, "transacciones")
	}
}

// REGRESIÓN: bajo el contrato nuevo los datos vienen de sources y Data
// llega vacío. La validación vieja (len(Data) > 0) rechazaría todos los
// artefactos nuevos, en silencio.
func TestParseArtifact_ChartConSourcesYSinData(t *testing.T) {
	raw := []byte(`{"type":"chart","title":"Ventas",
	  "sources":[{"monitor":"transacciones","query":{"monitor":"transacciones","aggregate":{"field":"monto","function":"sum","groupBy":"region"}}}],
	  "chartSpec":{"chartType":"bar","xKey":"_id","yKeys":["aggValue"]}}`)

	art, err := ParseArtifact(raw)
	if err != nil {
		t.Fatalf("un chart con sources y sin data debe ser válido: %v", err)
	}
	if len(art.Sources) != 1 {
		t.Fatalf("quiero 1 source, tengo %d", len(art.Sources))
	}
}

func TestParseArtifact_TableConDataYSinSourcesSigueSiendoValido(t *testing.T) {
	raw := []byte(`{"type":"table","title":"T","chartSpec":{"data":[{"a":1}]}}`)
	art, err := ParseArtifact(raw)
	if err != nil {
		t.Fatalf("una instantánea debe seguir siendo válida: %v", err)
	}
	if art.IsRerunnable() {
		t.Error("sin sources no es re-ejecutable")
	}
}

func TestParseArtifact_SinSourcesNiDataEsInvalido(t *testing.T) {
	raw := []byte(`{"type":"table","title":"T","chartSpec":{}}`)
	if _, err := ParseArtifact(raw); err == nil {
		t.Fatal("un artefacto sin datos ni sources debe ser rechazado")
	}
}

func TestParseArtifact_SourceSinMonitorEsInvalido(t *testing.T) {
	raw := []byte(`{"type":"table","title":"T",
	  "sources":[{"query":{"aggregate":{"field":"monto","function":"sum"}}}]}`)
	if _, err := ParseArtifact(raw); err == nil {
		t.Fatal("una source sin monitor debe ser rechazada")
	}
}

// Un chart con varias fuentes se pivotea sobre xKey; sin xKey no hay
// sobre qué pivotear.
func TestParseArtifact_ChartMultiFuenteSinXKeyEsInvalido(t *testing.T) {
	raw := []byte(`{"type":"chart","title":"T",
	  "sources":[
	    {"monitor":"a","query":{"monitor":"a","aggregate":{"field":"m","function":"sum"}}},
	    {"monitor":"b","query":{"monitor":"b","aggregate":{"field":"m","function":"sum"}}}],
	  "chartSpec":{"chartType":"bar","yKeys":["aggValue"]}}`)
	if _, err := ParseArtifact(raw); err == nil {
		t.Fatal("un chart multi-fuente sin xKey debe ser rechazado")
	}
}

// RunArtifact (Task 6) reconstruye el pipeline directo desde src.Query,
// sin volver a llamar a ParseQueryToolInput. Si acá se guardara la query
// cruda del LLM en vez de la normalizada, un alias con espacios no
// resolvería al re-ejecutar y un limit fuera de rango no se recortaría.
func TestParseArtifact_NormalizaLaQueryDeCadaSourceAlPersistir(t *testing.T) {
	raw := []byte(`{"type":"table","title":"T",
	  "sources":[{"monitor":"  transacciones  ","query":{"conditionGroup":{"logic":"AND","conditions":[]},"limit":999999}}]}`)

	art, err := ParseArtifact(raw)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	src := art.Sources[0]
	if src.Monitor != "transacciones" {
		t.Errorf("Monitor debe quedar normalizado (sin espacios), tengo %q", src.Monitor)
	}
	if src.Query.Monitor != "transacciones" {
		t.Errorf("Query.Monitor debe quedar normalizado, tengo %q", src.Query.Monitor)
	}
	if src.Query.Limit != chatQueryMaxLimit {
		t.Errorf("Limit fuera de rango debe recortarse a %d, tengo %d", chatQueryMaxLimit, src.Query.Limit)
	}
}

// ACCEPTANCE: el bloque <artifact> lo escribe el LLM, y su salida está
// moldeada por celdas de CSV/Excel que subió el usuario — una inyección
// ahí podría intentar colar "saved":true (planta el artefacto en la
// biblioteca sin pasar por el endpoint de guardado) o "monitorIds"/
// "userId" (la allowlist de autorización). Estos campos deben quedar en
// su cero pase lo que pase en el JSON de entrada.
func TestParseArtifact_CamposDeSoloServidorSeIgnoranDelLLM(t *testing.T) {
	raw := []byte(`{"type":"table","title":"T","chartSpec":{"data":[{"a":1}]},
	  "saved":true,"savedName":"robado","userId":"605c5f0f5f0f5f0f5f0f5f0f",
	  "conversationId":"605c5f0f5f0f5f0f5f0f5f0f","messageId":"605c5f0f5f0f5f0f5f0f5f0f",
	  "monitorIds":["605c5f0f5f0f5f0f5f0f5f0f"],
	  "cachedData":[{"x":1}],"ranAt":"2026-01-01T00:00:00Z",
	  "createdAt":"2026-01-01T00:00:00Z","id":"605c5f0f5f0f5f0f5f0f5f0f"}`)

	art, err := ParseArtifact(raw)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if art.Saved {
		t.Error("Saved debe quedar en su cero: el LLM no puede fijarlo")
	}
	if art.SavedName != "" {
		t.Error("SavedName debe quedar vacío")
	}
	if !art.UserID.IsZero() {
		t.Error("UserID debe quedar en su cero")
	}
	if !art.ConversationID.IsZero() {
		t.Error("ConversationID debe quedar en su cero")
	}
	if !art.MessageID.IsZero() {
		t.Error("MessageID debe quedar en su cero")
	}
	if len(art.MonitorIDs) != 0 {
		t.Error("MonitorIDs debe quedar vacío: el LLM no controla la allowlist")
	}
	if art.CachedData != nil {
		t.Error("CachedData debe quedar vacío")
	}
	if !art.RanAt.IsZero() {
		t.Error("RanAt debe quedar en su cero")
	}
	if !art.CreatedAt.IsZero() {
		t.Error("CreatedAt debe quedar en su cero")
	}
	if !art.ID.IsZero() {
		t.Error("ID debe quedar en su cero")
	}
}
