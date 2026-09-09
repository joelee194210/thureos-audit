package services

import (
	"testing"

	"github.com/thureos/compliance/internal/models"
)

func schemaConTimestamp() []models.SchemaField {
	return []models.SchemaField{
		{Name: "timestamp", Type: models.FieldDate},
		{Name: "tarjeta", Type: models.FieldString},
		{Name: "importe", Type: models.FieldNumber},
	}
}

func baseVelocityCondition() models.VelocityCondition {
	return models.VelocityCondition{
		TimeField: "timestamp",
		MaxGap:    "35s",
		GroupBy:   "tarjeta",
		MinEvents: 2,
	}
}

func TestValidateVelocityCondition_Valida(t *testing.T) {
	if err := ValidateVelocityCondition(baseVelocityCondition(), schemaConTimestamp()); err != nil {
		t.Errorf("esperaba condición válida, got %v", err)
	}
}

func TestValidateVelocityCondition_TimeFieldVacio(t *testing.T) {
	cond := baseVelocityCondition()
	cond.TimeField = ""
	if err := ValidateVelocityCondition(cond, schemaConTimestamp()); err == nil {
		t.Error("sin TimeField debería rechazarse")
	}
}

// El motor mide diferencias de tiempo: un campo que no es date no sirve, y
// aceptarlo en silencio produciría una regla que nunca dispara.
func TestValidateVelocityCondition_TimeFieldNoEsDate(t *testing.T) {
	cond := baseVelocityCondition()
	cond.TimeField = "importe"
	if err := ValidateVelocityCondition(cond, schemaConTimestamp()); err == nil {
		t.Error("un TimeField que no es de tipo date debería rechazarse")
	}
}

func TestValidateVelocityCondition_MaxGapInvalido(t *testing.T) {
	cond := baseVelocityCondition()
	cond.MaxGap = "35x"
	if err := ValidateVelocityCondition(cond, schemaConTimestamp()); err == nil {
		t.Error("un MaxGap que no parsea debería rechazarse")
	}
}

func TestValidateVelocityCondition_GroupByInexistente(t *testing.T) {
	cond := baseVelocityCondition()
	cond.GroupBy = "no_existe"
	if err := ValidateVelocityCondition(cond, schemaConTimestamp()); err == nil {
		t.Error("un GroupBy fuera del esquema debería rechazarse")
	}
}

func TestValidateVelocityCondition_MinEventsMenorADos(t *testing.T) {
	cond := baseVelocityCondition()
	cond.MinEvents = 1
	if err := ValidateVelocityCondition(cond, schemaConTimestamp()); err == nil {
		t.Error("MinEvents < 2 no tiene sentido: hace falta un par para medir un gap")
	}
}
