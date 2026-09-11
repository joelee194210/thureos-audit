package services

import (
	"strings"
	"testing"

	"github.com/thureos/compliance/internal/models"
)

// El caso real que rompió: un monitor de movimientos con `importe` a dos
// decimales implícitos. El modelo veía sample "500000" + impliedDecimals 2,
// deducía que cinco mil se escribe 500000, y emitía `importe > 500000`
// contra documentos guardados como 5000. Cero matches, cero errores.
func TestImpliedDecimalsNote_TraduceLaEscalaGuardada(t *testing.T) {
	schema := []models.SchemaField{
		{Name: "importe", Type: models.FieldNumber, Sample: "500000", ImpliedDecimals: 2},
		{Name: "mcc", Type: models.FieldNumber, Sample: "7995"},
	}

	nota := impliedDecimalsNote(schema)

	if !strings.Contains(nota, `"importe"`) {
		t.Errorf("la nota no menciona el campo con decimales:\n%s", nota)
	}
	if !strings.Contains(nota, "stored as 5000") {
		t.Errorf("la nota no traduce 500000 a la escala guardada:\n%s", nota)
	}
	if strings.Contains(nota, `"mcc"`) {
		t.Errorf("la nota menciona un campo sin decimales implícitos:\n%s", nota)
	}
}

// Sin campos con decimales implícitos el mensaje al modelo no cambia en
// nada: es el caso de la enorme mayoría de los monitores.
func TestImpliedDecimalsNote_VaciaSinDecimales(t *testing.T) {
	if nota := impliedDecimalsNote(testSchema()); nota != "" {
		t.Errorf("esperaba nota vacía, quedó:\n%s", nota)
	}
	if msg := buildUserMessage(testSchema(), "", "hola", nil); strings.Contains(msg, "Implied decimals") {
		t.Errorf("agregó la sección de decimales sin campos que la necesiten:\n%s", msg)
	}
}

// Un sample no numérico no puede convertirse; la advertencia de escala
// tiene que aparecer igual, porque el campo sigue teniendo decimales.
func TestImpliedDecimalsNote_SampleNoNumerico(t *testing.T) {
	schema := []models.SchemaField{
		{Name: "importe", Type: models.FieldNumber, Sample: "", ImpliedDecimals: 2},
	}

	nota := impliedDecimalsNote(schema)

	if !strings.Contains(nota, `"importe"`) {
		t.Errorf("la nota omitió el campo por no poder convertir el sample:\n%s", nota)
	}
	if strings.Contains(nota, "stored as") {
		t.Errorf("inventó una conversión sin sample numérico:\n%s", nota)
	}
}

// La nota viaja en el mensaje real, no solo en el helper.
func TestBuildUserMessage_IncluyeLaNotaDeDecimales(t *testing.T) {
	schema := []models.SchemaField{
		{Name: "importe", Type: models.FieldNumber, Sample: "500000", ImpliedDecimals: 2},
	}

	msg := buildUserMessage(schema, "", "casino sobre 5000", nil)

	if !strings.Contains(msg, "stored as 5000") {
		t.Errorf("el mensaje no lleva la traducción de escala:\n%s", msg)
	}
}
