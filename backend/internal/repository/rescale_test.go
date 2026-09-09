package repository

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

// Son montos: la conversión tiene que usar potencias de diez enteras. Con
// `$multiply` por 0.1, 5000 da 500.00000000000006 en float64.
func TestRescaleExpression_UsaPotenciasEnteras(t *testing.T) {
	casos := []struct {
		nombre   string
		delta    int
		operador string
		factor   int64
	}{
		{"de 2 a 3 decimales: el valor se achica", 1, "$divide", 10},
		{"de 0 a 2 decimales", 2, "$divide", 100},
		{"de 2 a 0 decimales: el valor se agranda", -2, "$multiply", 100},
		{"de 3 a 2 decimales", -1, "$multiply", 10},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			expr := rescaleExpression("importe", c.delta)

			raw, err := bson.Marshal(expr)
			if err != nil {
				t.Fatalf("no serializa: %v", err)
			}
			var m bson.M
			if err := bson.Unmarshal(raw, &m); err != nil {
				t.Fatalf("no deserializa: %v", err)
			}

			operandos, ok := m[c.operador].(bson.A)
			if !ok {
				t.Fatalf("esperaba %s, quedó %#v", c.operador, m)
			}
			if operandos[0] != "$importe" {
				t.Errorf("primer operando = %v, quería $importe", operandos[0])
			}
			if got, ok := operandos[1].(int64); !ok || got != c.factor {
				t.Errorf("factor = %#v, quería el entero %d", operandos[1], c.factor)
			}
		})
	}
}

// delta 0 no es un cambio de escala: no hay nada que convertir y devolver
// una expresión igual sería reescribir toda la colección para nada.
func TestRescaleExpression_DeltaCeroEsNil(t *testing.T) {
	if expr := rescaleExpression("importe", 0); expr != nil {
		t.Errorf("con delta 0 esperaba nil, quedó %#v", expr)
	}
}
