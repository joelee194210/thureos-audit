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
	got := partitionSuggestions(suggestions, testSchema()).Suggestions
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
	got := partitionSuggestions(suggestions, testSchema()).Suggestions
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
	got := partitionSuggestions(suggestions, testSchema()).Suggestions
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
	got := partitionSuggestions(suggestions, testSchema()).Suggestions
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
	got := partitionSuggestions(suggestions, testSchema()).Suggestions
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
	got := partitionSuggestions(suggestions, testSchema()).Suggestions
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
	got := partitionSuggestions(suggestions, testSchema()).Suggestions
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
	got := partitionSuggestions(suggestions, testSchema()).Suggestions
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
	got := partitionSuggestions(suggestions, testSchema()).Suggestions
	if len(got) != 1 {
		t.Fatalf("got %d suggestions, want 1 (solo la válida sobrevive)", len(got))
	}
	if got[0].Name != "Válida" {
		t.Errorf("got %q, want %q", got[0].Name, "Válida")
	}
}

// Un agregado global (groupBy vacío) es válido para el motor: agrupa con
// _id:null. Descartarlo entero era la causa más frecuente de "generé reglas
// y no salió ninguna".
func TestDiscardReason_AceptaAgregadoGlobal(t *testing.T) {
	s := AIRuleSuggestion{
		Name: "Total diario",
		AggregateConditions: []models.AggregateCondition{{
			Field:      "monto",
			Function:   models.AggFuncSum,
			GroupBy:    "",
			TimeField:  "fecha",
			TimeWindow: "24h",
			Operator:   models.OpGreaterThan,
			Threshold:  50000,
		}},
	}
	if got := discardReason(s, testSchema()); got != "" {
		t.Errorf("discardReason = %q, quería aceptarla", got)
	}
}

// Sin timeField ni timeWindow el motor no aplica ventana: es un agregado
// sobre todo el histórico, perfectamente expresable.
func TestDiscardReason_AceptaAgregadoSinVentana(t *testing.T) {
	s := AIRuleSuggestion{
		Name: "Conteo por cliente",
		AggregateConditions: []models.AggregateCondition{{
			Field:     "monto",
			Function:  models.AggFuncCount,
			GroupBy:   "cliente",
			Operator:  models.OpGreaterEqual,
			Threshold: 5,
		}},
	}
	if got := discardReason(s, testSchema()); got != "" {
		t.Errorf("discardReason = %q, quería aceptarla", got)
	}
}

// Media ventana es intención a medio expresar: "5 transacciones en 60s" que
// pierde el "en 60s" se convierte en "5 transacciones alguna vez", que es una
// regla mucho más ruidosa que la pedida. Se descarta.
func TestDiscardReason_DescartaVentanaIncompleta(t *testing.T) {
	s := AIRuleSuggestion{
		Name: "Ventana a medias",
		AggregateConditions: []models.AggregateCondition{{
			Field:     "monto",
			Function:  models.AggFuncCount,
			GroupBy:   "cliente",
			TimeField: "fecha",
			Operator:  models.OpGreaterEqual,
			Threshold: 5,
		}},
	}
	if got := discardReason(s, testSchema()); got == "" {
		t.Error("discardReason = \"\", quería descartarla por ventana incompleta")
	}
}

// Cuando todo se descarta, la respuesta tiene que poder explicar por qué:
// el usuario ve terminar "Generando..." y necesita saber qué corregir.
func TestPartitionSuggestions_ReportaGeneradasYDescartadas(t *testing.T) {
	suggestions := []AIRuleSuggestion{
		{
			Name: "Válida",
			ConditionGroup: models.ConditionGroup{
				Logic:      "AND",
				Conditions: []models.Condition{{Field: "monto", Operator: models.OpGreaterThan, Value: 10000}},
			},
		},
		{
			Name: "Campo inventado",
			ConditionGroup: models.ConditionGroup{
				Logic:      "AND",
				Conditions: []models.Condition{{Field: "no_existe", Operator: models.OpGreaterThan, Value: 1}},
			},
		},
	}

	got := partitionSuggestions(suggestions, testSchema())

	if got.Generated != 2 {
		t.Errorf("Generated = %d, quería 2", got.Generated)
	}
	if len(got.Suggestions) != 1 || got.Suggestions[0].Name != "Válida" {
		t.Errorf("Suggestions = %+v, quería solo la válida", got.Suggestions)
	}
	if len(got.Discarded) != 1 {
		t.Fatalf("Discarded = %+v, quería una", got.Discarded)
	}
	if got.Discarded[0].Name != "Campo inventado" || got.Discarded[0].Reason == "" {
		t.Errorf("Discarded[0] = %+v, quería el nombre y un motivo no vacío", got.Discarded[0])
	}
}

func TestDiscardReason_DescartaCamposInexistentes(t *testing.T) {
	casos := []struct {
		nombre string
		s      AIRuleSuggestion
	}{
		{"campo de condición inventado", AIRuleSuggestion{
			ConditionGroup: models.ConditionGroup{
				Logic:      "AND",
				Conditions: []models.Condition{{Field: "no_existe", Operator: models.OpGreaterThan, Value: 1}},
			},
		}},
		{"groupBy inventado", AIRuleSuggestion{
			AggregateConditions: []models.AggregateCondition{{
				Field: "monto", Function: models.AggFuncSum, GroupBy: "no_existe",
				Operator: models.OpGreaterThan, Threshold: 1,
			}},
		}},
		{"ventana con unidad ambigua", AIRuleSuggestion{
			AggregateConditions: []models.AggregateCondition{{
				Field: "monto", Function: models.AggFuncSum, GroupBy: "cliente",
				TimeField: "fecha", TimeWindow: "5m",
				Operator: models.OpGreaterThan, Threshold: 1,
			}},
		}},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := discardReason(c.s, testSchema()); got == "" {
				t.Error("discardReason = \"\", quería descartarla")
			}
		})
	}
}
