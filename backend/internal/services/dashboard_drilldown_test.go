package services

import (
	"fmt"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

// El caso reportado: un widget agrupado por "cliente", que es numérico. La
// gráfica rotula el segmento 178, el navegador lo manda de vuelta como
// "178", y comparar texto contra número no matchea nada. Se hacía clic en
// una barra con registros y el detalle salía vacío, sin ningún error.
func TestGroupValueFilter_CruzaTiposBSON(t *testing.T) {
	got := groupValueFilter("178")

	m, ok := got.(bson.M)
	if !ok {
		t.Fatalf("esperaba un operador con candidatos, quedó %#v", got)
	}
	cands, ok := m["$in"].([]interface{})
	if !ok {
		t.Fatalf("esperaba $in con lista, quedó %#v", m)
	}
	if fmt.Sprint(cands) != fmt.Sprint([]interface{}{"178", float64(178)}) {
		t.Errorf("candidatos = %v, quería el texto y el número", cands)
	}
}

// Un valor que no es un número se compara directo: agrupar por tarjeta o
// por comercio tiene que seguir funcionando exactamente igual que antes.
func TestGroupValueFilter_TextoQuedaIgual(t *testing.T) {
	for _, valor := range []string{
		"999000******4001",
		"Pharaohs Casino",
		"AUTORIZADA",
		"0012", // identificador con ceros: no debe volverse el número 12
	} {
		if got := groupValueFilter(valor); got != interface{}(valor) {
			t.Errorf("con %q esperaba comparación directa, quedó %#v", valor, got)
		}
	}
}

// El vacío es el segmento de los registros sin valor en ese campo, y se
// filtra por igualdad con la cadena vacía como siempre.
func TestGroupValueFilter_VacioQuedaIgual(t *testing.T) {
	if got := groupValueFilter(""); got != interface{}("") {
		t.Errorf("esperaba la cadena vacía, quedó %#v", got)
	}
}
