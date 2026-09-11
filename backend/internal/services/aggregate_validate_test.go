package services

import (
	"strings"
	"testing"

	"github.com/thureos/compliance/internal/models"
)

func esquemaAgg() []models.SchemaField {
	return []models.SchemaField{
		{Name: "importe", Type: models.FieldNumber},
		{Name: "tarjeta", Type: models.FieldString},
		{Name: "fechapoliza", Type: models.FieldNumber},
		{Name: "timestamp", Type: models.FieldDate},
	}
}

func TestValidateAggregateCondition_Acepta(t *testing.T) {
	casos := []struct {
		nombre string
		cond   models.AggregateCondition
	}{
		{"suma con ventana", models.AggregateCondition{
			Field: "importe", Function: models.AggFuncSum, GroupBy: "tarjeta",
			TimeField: "timestamp", TimeWindow: "24h",
			Operator: models.OpGreaterThan, Threshold: 5000,
		}},
		{"agregado global", models.AggregateCondition{
			Field: "importe", Function: models.AggFuncSum, GroupBy: "",
			TimeField: "timestamp", TimeWindow: "24h",
			Operator: models.OpGreaterThan, Threshold: 5000,
		}},
		{"count sin field", models.AggregateCondition{
			Function: models.AggFuncCount, GroupBy: "tarjeta",
			Operator: models.OpGreaterEqual, Threshold: 3,
		}},
		{"sin ventana", models.AggregateCondition{
			Field: "importe", Function: models.AggFuncSum, GroupBy: "tarjeta",
			Operator: models.OpGreaterThan, Threshold: 5000,
		}},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if err := ValidateAggregateCondition(c.cond, esquemaAgg()); err != nil {
				t.Errorf("la rechazó: %v", err)
			}
		})
	}
}

// El caso exacto de la única regla viva en producción: ventana de 60s sobre
// un campo numérico. $match compara contra un time.Time, no matchea nada y
// la regla no dispara nunca.
func TestValidateAggregateCondition_RechazaTimeFieldNoDate(t *testing.T) {
	cond := models.AggregateCondition{
		Field: "importe", Function: models.AggFuncSum, GroupBy: "tarjeta",
		TimeField: "fechapoliza", TimeWindow: "60s",
		Operator: models.OpGreaterThan, Threshold: 5000,
	}
	err := ValidateAggregateCondition(cond, esquemaAgg())
	if err == nil {
		t.Fatal("la aceptó; tenía que rechazarla")
	}
	if !strings.Contains(err.Error(), "date") {
		t.Errorf("el mensaje no dice que hace falta un campo date: %v", err)
	}
}

func TestValidateAggregateCondition_Rechaza(t *testing.T) {
	casos := []struct {
		nombre string
		cond   models.AggregateCondition
	}{
		{"función inventada", models.AggregateCondition{
			Field: "importe", Function: models.AggFunction("distinct"), GroupBy: "tarjeta",
			Operator: models.OpGreaterThan, Threshold: 1,
		}},
		{"función vacía", models.AggregateCondition{
			Field: "importe", GroupBy: "tarjeta",
			Operator: models.OpGreaterThan, Threshold: 1,
		}},
		{"field vacío sin ser count", models.AggregateCondition{
			Function: models.AggFuncSum, GroupBy: "tarjeta",
			Operator: models.OpGreaterThan, Threshold: 1,
		}},
		{"field inexistente", models.AggregateCondition{
			Field: "no_existe", Function: models.AggFuncSum, GroupBy: "tarjeta",
			Operator: models.OpGreaterThan, Threshold: 1,
		}},
		{"groupBy inexistente", models.AggregateCondition{
			Field: "importe", Function: models.AggFuncSum, GroupBy: "no_existe",
			Operator: models.OpGreaterThan, Threshold: 1,
		}},
		{"timeField inexistente", models.AggregateCondition{
			Field: "importe", Function: models.AggFuncSum, GroupBy: "tarjeta",
			TimeField: "no_existe", TimeWindow: "24h",
			Operator: models.OpGreaterThan, Threshold: 1,
		}},
		{"ventana incompleta", models.AggregateCondition{
			Field: "importe", Function: models.AggFuncSum, GroupBy: "tarjeta",
			TimeField: "timestamp",
			Operator:  models.OpGreaterThan, Threshold: 1,
		}},
		{"ventana de cero", models.AggregateCondition{
			Field: "importe", Function: models.AggFuncSum, GroupBy: "tarjeta",
			TimeField: "timestamp", TimeWindow: "0s",
			Operator: models.OpGreaterThan, Threshold: 1,
		}},
		{"ventana con unidad ambigua", models.AggregateCondition{
			Field: "importe", Function: models.AggFuncSum, GroupBy: "tarjeta",
			TimeField: "timestamp", TimeWindow: "5m",
			Operator: models.OpGreaterThan, Threshold: 1,
		}},
		{"filtro sobre campo inexistente", models.AggregateCondition{
			Field: "importe", Function: models.AggFuncSum, GroupBy: "tarjeta",
			Operator: models.OpGreaterThan, Threshold: 1,
			Filter: []models.Condition{{Field: "no_existe", Operator: models.OpEqual, Value: 1}},
		}},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if err := ValidateAggregateCondition(c.cond, esquemaAgg()); err == nil {
				t.Error("la aceptó; tenía que rechazarla")
			}
		})
	}
}
