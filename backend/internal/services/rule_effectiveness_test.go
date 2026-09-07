package services

import (
	"testing"
	"time"

	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func rf(disposition models.RedFlagDisposition, closed bool, closeAfter time.Duration) models.RedFlag {
	created := time.Now().Add(-30 * 24 * time.Hour)
	f := models.RedFlag{CreatedAt: created, Disposition: disposition}
	if closed {
		t := created.Add(closeAfter)
		f.ClosedAt = &t
	}
	return f
}

func TestComputeEffectiveness_ReglaSinRedFlags(t *testing.T) {
	s := ComputeEffectiveness(nil)
	if s.Total != 0 || s.Open != 0 || s.FalsePositiveRate != 0 || s.AvgCloseHours != 0 {
		t.Fatalf("regla sin red flags debe dar todo en cero, got %+v", s)
	}
}

func TestComputeEffectiveness_DesgloseYTasaSoloSobreCerradas(t *testing.T) {
	flags := []models.RedFlag{
		rf(models.DispositionFalsePositive, true, 2*time.Hour),
		rf(models.DispositionFalsePositive, true, 4*time.Hour),
		rf(models.DispositionConfirmedROS, true, 6*time.Hour),
		rf(models.DispositionNoAction, true, 12*time.Hour),
		rf("", false, 0), // abierta, no cuenta para la tasa
	}
	s := ComputeEffectiveness(flags)

	if s.Total != 5 {
		t.Errorf("Total = %d, want 5", s.Total)
	}
	if s.Open != 1 {
		t.Errorf("Open = %d, want 1", s.Open)
	}
	if s.FalsePositive != 2 || s.ConfirmedROS != 1 || s.NoAction != 1 {
		t.Errorf("desglose = %+v", s)
	}
	// 2 falsos positivos / 4 cerradas = 0.5 — la abierta no entra al denominador
	if s.FalsePositiveRate != 0.5 {
		t.Errorf("FalsePositiveRate = %v, want 0.5", s.FalsePositiveRate)
	}
	// (2+4+6+12)/4 = 6h
	if s.AvgCloseHours != 6 {
		t.Errorf("AvgCloseHours = %v, want 6", s.AvgCloseHours)
	}
}

func TestGroupRuleIDsByTemplate_AgrupaYExcluyeManuales(t *testing.T) {
	idA1, idA2, idB, idManual := primitive.NewObjectID(), primitive.NewObjectID(), primitive.NewObjectID(), primitive.NewObjectID()
	rules := []models.Rule{
		{ID: idA1, TemplateID: "velocity_24h"},
		{ID: idA2, TemplateID: "velocity_24h"},
		{ID: idB, TemplateID: "high_amount"},
		{ID: idManual, TemplateID: ""}, // regla manual, no nació de ninguna tipología
	}

	groups := GroupRuleIDsByTemplate(rules)

	if len(groups) != 2 {
		t.Fatalf("groups = %+v, want 2 tipologías", groups)
	}
	if len(groups["velocity_24h"]) != 2 {
		t.Errorf("velocity_24h = %v, want 2 reglas", groups["velocity_24h"])
	}
	if len(groups["high_amount"]) != 1 {
		t.Errorf("high_amount = %v, want 1 regla", groups["high_amount"])
	}
	for tpl, ids := range groups {
		for _, id := range ids {
			if id == idManual {
				t.Errorf("la regla manual no debería aparecer en el grupo %q", tpl)
			}
		}
	}
}

func TestComputeEffectiveness_TodoAbiertoNoDaDivisionPorCero(t *testing.T) {
	flags := []models.RedFlag{rf("", false, 0), rf("", false, 0)}
	s := ComputeEffectiveness(flags)
	if s.Total != 2 || s.Open != 2 {
		t.Fatalf("got %+v", s)
	}
	if s.FalsePositiveRate != 0 || s.AvgCloseHours != 0 {
		t.Errorf("sin cerradas, tasa y tiempo medio deben quedar en 0, got %+v", s)
	}
}
