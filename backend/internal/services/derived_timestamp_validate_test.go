package services

import (
	"testing"

	"github.com/thureos/compliance/internal/models"
)

func schemaConFechaYHora() []models.SchemaField {
	return []models.SchemaField{
		{Name: "fechapoliza", Type: models.FieldNumber},
		{Name: "horapoliza", Type: models.FieldNumber},
		{Name: "importe", Type: models.FieldNumber},
	}
}

func TestValidateDerivedTimestamp_ConfigValida(t *testing.T) {
	cfg := models.DerivedTimestampConfig{
		DateField: "fechapoliza", DateFormat: "YYYYMMDD",
		TimeField: "horapoliza", TimeFormat: "HHMMSS",
		TargetName: "timestamp",
	}
	if err := ValidateDerivedTimestamp(cfg, schemaConFechaYHora()); err != nil {
		t.Errorf("esperaba configuración válida, got %v", err)
	}
}

func TestValidateDerivedTimestamp_CampoInexistente(t *testing.T) {
	cfg := models.DerivedTimestampConfig{
		DateField: "no_existe", DateFormat: "YYYYMMDD",
		TimeField: "horapoliza", TimeFormat: "HHMMSS",
		TargetName: "timestamp",
	}
	if err := ValidateDerivedTimestamp(cfg, schemaConFechaYHora()); err == nil {
		t.Error("un campo fuera del esquema debería rechazarse")
	}
}

func TestValidateDerivedTimestamp_FormatoNoSoportado(t *testing.T) {
	cfg := models.DerivedTimestampConfig{
		DateField: "fechapoliza", DateFormat: "DD/MM/YYYY",
		TimeField: "horapoliza", TimeFormat: "HHMMSS",
		TargetName: "timestamp",
	}
	if err := ValidateDerivedTimestamp(cfg, schemaConFechaYHora()); err == nil {
		t.Error("un formato fuera de los soportados debería rechazarse")
	}
}

// El campo derivado no puede pisar una columna real del archivo.
func TestValidateDerivedTimestamp_TargetColisionaConCampoExistente(t *testing.T) {
	cfg := models.DerivedTimestampConfig{
		DateField: "fechapoliza", DateFormat: "YYYYMMDD",
		TimeField: "horapoliza", TimeFormat: "HHMMSS",
		TargetName: "importe",
	}
	if err := ValidateDerivedTimestamp(cfg, schemaConFechaYHora()); err == nil {
		t.Error("un TargetName que pisa un campo existente debería rechazarse")
	}
}

func TestValidateDerivedTimestamp_TargetVacio(t *testing.T) {
	cfg := models.DerivedTimestampConfig{
		DateField: "fechapoliza", DateFormat: "YYYYMMDD",
		TimeField: "horapoliza", TimeFormat: "HHMMSS",
		TargetName: "",
	}
	if err := ValidateDerivedTimestamp(cfg, schemaConFechaYHora()); err == nil {
		t.Error("un TargetName vacío debería rechazarse")
	}
}
