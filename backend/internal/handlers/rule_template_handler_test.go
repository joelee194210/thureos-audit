package handlers

import (
	"reflect"
	"testing"

	"github.com/thureos/compliance/internal/models"
)

func schemaDePrueba() []models.SchemaField {
	return []models.SchemaField{
		{Name: "monto", Type: "number", Required: true},
		{Name: "cuenta", Type: "string", Required: true},
		{Name: "fecha", Type: "date", Required: true},
		{Name: "pais", Type: "string", Required: false},
	}
}

// Una columna que no está en el esquema del monitor dejaría una regla
// sorda: el binding crearía algo que nunca dispara. Por eso el handler
// rechaza antes de crear nada.
func TestUnknownColumns_ColumnaInexistenteSeDetecta(t *testing.T) {
	got := unknownColumns(
		map[string]string{"$AMOUNT": "monto", "$ACCOUNT": "banco_origen"},
		schemaDePrueba(),
	)
	if !reflect.DeepEqual(got, []string{"banco_origen"}) {
		t.Errorf("unknownColumns = %v, want [banco_origen]", got)
	}
}

func TestUnknownColumns_TodasConocidasPasan(t *testing.T) {
	got := unknownColumns(
		map[string]string{"$AMOUNT": "monto", "$ACCOUNT": "cuenta", "$DATE": "fecha"},
		schemaDePrueba(),
	)
	if len(got) != 0 {
		t.Errorf("unknownColumns = %v, want vacío", got)
	}
}

func TestUnknownColumns_VaciasIgnoradas(t *testing.T) {
	got := unknownColumns(
		map[string]string{"$AMOUNT": "monto", "$COUNTERPART": ""},
		schemaDePrueba(),
	)
	if len(got) != 0 {
		t.Errorf("unknownColumns = %v, want vacío", got)
	}
}
