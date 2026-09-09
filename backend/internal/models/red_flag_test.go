package models

import "testing"

// Los seis sitios que preguntaban `== RedFlagTypeAggregate` querían saber si
// la alerta es agrupada —si trae groupByField, groupByValue y matchCount—, no
// de qué condición vino. Velocidad las trae igual.
func TestEsAgrupada(t *testing.T) {
	casos := []struct {
		tipo     RedFlagType
		esperado bool
	}{
		{RedFlagTypeAggregate, true},
		{RedFlagTypeVelocity, true},
		{RedFlagTypeRow, false},
		{RedFlagType(""), false},
	}
	for _, c := range casos {
		if got := c.tipo.EsAgrupada(); got != c.esperado {
			t.Errorf("EsAgrupada(%q) = %v, quería %v", c.tipo, got, c.esperado)
		}
	}
}
