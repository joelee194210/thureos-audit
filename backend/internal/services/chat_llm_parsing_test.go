package services

import (
	"encoding/json"
	"testing"

	"github.com/thureos/compliance/internal/models"
)

func TestParseQueryToolInput_ConConditionGroupValido(t *testing.T) {
	raw := json.RawMessage(`{"conditionGroup":{"logic":"AND","conditions":[{"field":"amount","operator":"gt","value":1000}]}}`)
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
	raw := json.RawMessage(`{"aggregate":{"field":"amount","function":"sum","groupBy":"account"}}`)
	input, err := ParseQueryToolInput(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if input.Aggregate == nil || input.Aggregate.Field != "amount" {
		t.Fatalf("aggregate no se parseó correctamente: %+v", input)
	}
}

func TestParseQueryToolInput_SinConditionGroupNiAggregateFalla(t *testing.T) {
	raw := json.RawMessage(`{}`)
	if _, err := ParseQueryToolInput(raw); err == nil {
		t.Fatal("esperaba error cuando no hay conditionGroup ni aggregate")
	}
}

func TestParseQueryToolInput_AggregateSinFieldFalla(t *testing.T) {
	raw := json.RawMessage(`{"aggregate":{"function":"sum"}}`)
	if _, err := ParseQueryToolInput(raw); err == nil {
		t.Fatal("esperaba error cuando aggregate.field está vacío")
	}
}

func TestParseQueryToolInput_LimitFueraDeRangoSeAjustaAlTope(t *testing.T) {
	raw := json.RawMessage(`{"conditionGroup":{"logic":"AND","conditions":[]},"limit":999999}`)
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
