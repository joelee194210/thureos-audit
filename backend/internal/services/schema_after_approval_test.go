package services

import (
	"testing"

	"github.com/thureos/compliance/internal/models"
)

func schemaDetectadoDelArchivo() []models.SchemaField {
	return []models.SchemaField{
		{Name: "tarjeta", Type: models.FieldString},
		{Name: "fechapoliza", Type: models.FieldNumber},
		{Name: "horapoliza", Type: models.FieldNumber},
	}
}

func cfgTimestamp(target string) *models.DerivedTimestampConfig {
	return &models.DerivedTimestampConfig{
		DateField: "fechapoliza", DateFormat: "YYYYMMDD",
		TimeField: "horapoliza", TimeFormat: "HHMMSS",
		TargetName: target,
	}
}

// Aprobar una carga rechazada resetea el schema del monitor al del archivo
// aprobado. Sin esto, el campo derivado desaparecía del schema mientras la
// configuración seguía viva: las reglas que lo usaban dejaban de validar y el
// campo desaparecía de los selectores, aunque los documentos sí lo tenían.
func TestSchemaAfterApproval_ReagregaElCampoDerivado(t *testing.T) {
	schema, conflicto := SchemaAfterApproval(schemaDetectadoDelArchivo(), cfgTimestamp("timestamp"))
	if conflicto {
		t.Fatal("no debería haber conflicto: el archivo no trae una columna llamada timestamp")
	}
	var derivado *models.SchemaField
	for i := range schema {
		if schema[i].Name == "timestamp" {
			derivado = &schema[i]
		}
	}
	if derivado == nil {
		t.Fatal("el campo derivado debería volver al schema tras aprobar")
	}
	if derivado.Type != models.FieldDate {
		t.Errorf("el campo derivado debería ser de tipo date, got %s", derivado.Type)
	}
	if len(schema) != 4 {
		t.Errorf("esperaba 3 columnas del archivo + el derivado = 4, got %d", len(schema))
	}
}

// Si el archivo aprobado trae de verdad una columna con ese nombre, el campo
// derivado no puede pisarla: gana la columna real del archivo y el llamador
// tiene que resolver la configuración en vez de dejar un estado ambiguo.
func TestSchemaAfterApproval_ColumnaRealDelArchivoGana(t *testing.T) {
	detectado := append(schemaDetectadoDelArchivo(), models.SchemaField{Name: "timestamp", Type: models.FieldString})

	schema, conflicto := SchemaAfterApproval(detectado, cfgTimestamp("timestamp"))
	if !conflicto {
		t.Fatal("esperaba que se reportara el conflicto")
	}
	if len(schema) != 4 {
		t.Errorf("el schema debería quedar tal cual lo detectado (4 campos), got %d", len(schema))
	}
	for _, f := range schema {
		if f.Name == "timestamp" && f.Type != models.FieldString {
			t.Errorf("la columna real del archivo debería conservar su tipo string, got %s", f.Type)
		}
	}
}

func TestSchemaAfterApproval_SinConfigDevuelveLoDetectado(t *testing.T) {
	detectado := schemaDetectadoDelArchivo()
	schema, conflicto := SchemaAfterApproval(detectado, nil)
	if conflicto {
		t.Error("sin configuración no puede haber conflicto")
	}
	if len(schema) != len(detectado) {
		t.Errorf("sin configuración el schema no debería cambiar, got %d want %d", len(schema), len(detectado))
	}
}
