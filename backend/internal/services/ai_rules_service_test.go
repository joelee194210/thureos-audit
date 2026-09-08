package services

import (
	"testing"

	"github.com/thureos/compliance/internal/models"
)

func testSchema() []models.SchemaField {
	return []models.SchemaField{
		{Name: "monto", Type: models.FieldNumber},
		{Name: "cliente", Type: models.FieldString},
		{Name: "fecha", Type: models.FieldDate},
	}
}

func TestFilterValidSuggestions_KeepsSuggestionWithValidConditionGroupOnly(t *testing.T) {
	suggestions := []AIRuleSuggestion{
		{
			Name: "Monto alto",
			ConditionGroup: models.ConditionGroup{
				Logic:      "AND",
				Conditions: []models.Condition{{Field: "monto", Operator: models.OpGreaterThan, Value: 10000}},
			},
		},
	}
	got := filterValidSuggestions(suggestions, testSchema())
	if len(got) != 1 {
		t.Fatalf("got %d suggestions, want 1 (debería aceptarse — todos los campos existen)", len(got))
	}
}

func TestFilterValidSuggestions_DropsUnknownConditionGroupField(t *testing.T) {
	suggestions := []AIRuleSuggestion{
		{
			Name: "Campo inventado",
			ConditionGroup: models.ConditionGroup{
				Logic:      "AND",
				Conditions: []models.Condition{{Field: "group_count_gte_5_seconds_lte_60", Operator: models.OpGreaterThan, Value: 5}},
			},
		},
	}
	got := filterValidSuggestions(suggestions, testSchema())
	if len(got) != 0 {
		t.Fatalf("got %d suggestions, want 0 (el campo no existe en el schema)", len(got))
	}
}

func TestFilterValidSuggestions_KeepsValidAggregateCondition(t *testing.T) {
	suggestions := []AIRuleSuggestion{
		{
			Name:           "Velocidad",
			ConditionGroup: models.ConditionGroup{Logic: "AND", Conditions: []models.Condition{}},
			AggregateConditions: []models.AggregateCondition{
				{Field: "cliente", Function: models.AggFuncCount, GroupBy: "cliente", TimeField: "fecha", TimeWindow: "60s", Operator: models.OpGreaterEqual, Threshold: 5},
			},
		},
	}
	got := filterValidSuggestions(suggestions, testSchema())
	if len(got) != 1 {
		t.Fatalf("got %d suggestions, want 1 (aggregateCondition válida, todos los campos existen)", len(got))
	}
}

func TestFilterValidSuggestions_DropsUnknownAggregateField(t *testing.T) {
	suggestions := []AIRuleSuggestion{
		{
			Name: "Campo agregado inventado",
			AggregateConditions: []models.AggregateCondition{
				{Field: "same_per_group", Function: models.AggFuncCount, GroupBy: "cliente", TimeField: "fecha", TimeWindow: "60s", Operator: models.OpGreaterEqual, Threshold: 3},
			},
		},
	}
	got := filterValidSuggestions(suggestions, testSchema())
	if len(got) != 0 {
		t.Fatalf("got %d suggestions, want 0 (field agregado no existe en el schema)", len(got))
	}
}

func TestFilterValidSuggestions_DropsUnknownGroupBy(t *testing.T) {
	suggestions := []AIRuleSuggestion{
		{
			Name: "GroupBy inventado",
			AggregateConditions: []models.AggregateCondition{
				{Field: "cliente", Function: models.AggFuncCount, GroupBy: "campo_inexistente", TimeField: "fecha", TimeWindow: "60s", Operator: models.OpGreaterEqual, Threshold: 3},
			},
		},
	}
	got := filterValidSuggestions(suggestions, testSchema())
	if len(got) != 0 {
		t.Fatalf("got %d suggestions, want 0 (groupBy no existe en el schema)", len(got))
	}
}

func TestFilterValidSuggestions_DropsUnknownTimeField(t *testing.T) {
	suggestions := []AIRuleSuggestion{
		{
			Name: "TimeField inventado",
			AggregateConditions: []models.AggregateCondition{
				{Field: "cliente", Function: models.AggFuncCount, GroupBy: "cliente", TimeField: "campo_inexistente", TimeWindow: "60s", Operator: models.OpGreaterEqual, Threshold: 3},
			},
		},
	}
	got := filterValidSuggestions(suggestions, testSchema())
	if len(got) != 0 {
		t.Fatalf("got %d suggestions, want 0 (timeField no existe en el schema)", len(got))
	}
}

func TestFilterValidSuggestions_DropsInvalidTimeWindowFormat(t *testing.T) {
	suggestions := []AIRuleSuggestion{
		{
			Name: "Ventana inválida",
			AggregateConditions: []models.AggregateCondition{
				{Field: "cliente", Function: models.AggFuncCount, GroupBy: "cliente", TimeField: "fecha", TimeWindow: "5x", Operator: models.OpGreaterEqual, Threshold: 3},
			},
		},
	}
	got := filterValidSuggestions(suggestions, testSchema())
	if len(got) != 0 {
		t.Fatalf("got %d suggestions, want 0 (timeWindow con formato inválido)", len(got))
	}
}

func TestFilterValidSuggestions_DropsAmbiguousMonthsUnit(t *testing.T) {
	// "m" nunca se le enseña a la IA — si aparece, se rechaza en vez de
	// arriesgarse a que signifique "minutos" para quien lo generó.
	suggestions := []AIRuleSuggestion{
		{
			Name: "Unidad ambigua",
			AggregateConditions: []models.AggregateCondition{
				{Field: "cliente", Function: models.AggFuncCount, GroupBy: "cliente", TimeField: "fecha", TimeWindow: "5m", Operator: models.OpGreaterEqual, Threshold: 3},
			},
		},
	}
	got := filterValidSuggestions(suggestions, testSchema())
	if len(got) != 0 {
		t.Fatalf("got %d suggestions, want 0 ('m' es ambiguo, no está en el vocabulario permitido para la IA)", len(got))
	}
}

func TestFilterValidSuggestions_MixedListKeepsOnlyValid(t *testing.T) {
	suggestions := []AIRuleSuggestion{
		{
			Name:           "Válida",
			ConditionGroup: models.ConditionGroup{Logic: "AND", Conditions: []models.Condition{{Field: "monto", Operator: models.OpGreaterThan, Value: 1000}}},
		},
		{
			Name:           "Inválida",
			ConditionGroup: models.ConditionGroup{Logic: "AND", Conditions: []models.Condition{{Field: "campo_fantasma", Operator: models.OpGreaterThan, Value: 1000}}},
		},
	}
	got := filterValidSuggestions(suggestions, testSchema())
	if len(got) != 1 {
		t.Fatalf("got %d suggestions, want 1 (solo la válida sobrevive)", len(got))
	}
	if got[0].Name != "Válida" {
		t.Errorf("got %q, want %q", got[0].Name, "Válida")
	}
}
