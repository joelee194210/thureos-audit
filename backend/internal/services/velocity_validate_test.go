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

func TestValidateVelocityCondition_MinEventsMayorADosNoSoportado(t *testing.T) {
	cond := baseVelocityCondition()
	cond.MinEvents = 3
	if err := ValidateVelocityCondition(cond, schemaConTimestamp()); err == nil {
		t.Error("MinEvents > 2 debería rechazarse mientras el pipeline solo mida pares")
	}
}

// Omitir minEvents es lo natural para quien solo quiere "dos transacciones
// muy seguidas": debe valer 2, no dar un 400 sorpresivo.
func TestNormalizeVelocityConditions_MinEventsOmitidoValeDos(t *testing.T) {
	cond := baseVelocityCondition()
	cond.MinEvents = 0

	got := NormalizeVelocityConditions([]models.VelocityCondition{cond})

	if len(got) != 1 {
		t.Fatalf("esperaba una condición, got %d", len(got))
	}
	if got[0].MinEvents != 2 {
		t.Errorf("MinEvents = %d, want 2 por defecto", got[0].MinEvents)
	}
	if err := ValidateVelocityCondition(got[0], schemaConTimestamp()); err != nil {
		t.Errorf("una condición normalizada debería validar, got %v", err)
	}
}

func TestNormalizeVelocityConditions_NoPisaUnValorExplicito(t *testing.T) {
	cond := baseVelocityCondition()
	cond.MinEvents = 2

	got := NormalizeVelocityConditions([]models.VelocityCondition{cond})
	if got[0].MinEvents != 2 {
		t.Errorf("MinEvents = %d, want 2", got[0].MinEvents)
	}
}

func TestNormalizeVelocityConditions_SinCondicionesDevuelveNil(t *testing.T) {
	if got := NormalizeVelocityConditions(nil); got != nil {
		t.Errorf("sin condiciones debería devolver nil, got %+v", got)
	}
}
