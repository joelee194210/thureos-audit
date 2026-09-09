package handlers

import (
	"testing"

	"github.com/thureos/compliance/internal/models"
)

func TestCambiosDeEscala(t *testing.T) {
	actual := []models.SchemaField{
		{Name: "importe", ImpliedDecimals: 2},
		{Name: "comision", ImpliedDecimals: 0},
		{Name: "tarjeta"},
	}

	casos := []struct {
		nombre   string
		nuevo    []models.SchemaField
		esperado map[string]int
	}{
		{"sin cambios", actual, map[string]int{}},
		{"un campo suma decimales", []models.SchemaField{
			{Name: "importe", ImpliedDecimals: 3},
			{Name: "comision", ImpliedDecimals: 0},
			{Name: "tarjeta"},
		}, map[string]int{"importe": 1}},
		{"un campo los saca", []models.SchemaField{
			{Name: "importe", ImpliedDecimals: 0},
			{Name: "comision", ImpliedDecimals: 0},
			{Name: "tarjeta"},
		}, map[string]int{"importe": -2}},
		{"dos campos a la vez", []models.SchemaField{
			{Name: "importe", ImpliedDecimals: 4},
			{Name: "comision", ImpliedDecimals: 2},
			{Name: "tarjeta"},
		}, map[string]int{"importe": 2, "comision": 2}},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			got := cambiosDeEscala(actual, c.nuevo)
			if len(got) != len(c.esperado) {
				t.Fatalf("cambios = %v, quería %v", got, c.esperado)
			}
			for campo, delta := range c.esperado {
				if got[campo] != delta {
					t.Errorf("delta de %q = %d, quería %d", campo, got[campo], delta)
				}
			}
		})
	}
}
