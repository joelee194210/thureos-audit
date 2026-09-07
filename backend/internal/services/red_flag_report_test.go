package services

import (
	"bytes"
	"testing"
	"time"

	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestRenderRedFlagReportPDF_Row(t *testing.T) {
	rf := &models.RedFlag{
		ID:          primitive.NewObjectID(),
		RuleName:    "Monto inusual",
		MonitorName: "Tarjetas de crédito",
		Severity:    models.SeverityHigh,
		Status:      models.RedFlagNew,
		RedFlagType: models.RedFlagTypeRow,
		Message:     "Se detectaron 2 registros con monto superior a 10,000",
		MatchCount:  2,
		MatchedRecords: []map[string]interface{}{
			{"cliente": "Juan Pérez", "monto": 15000.5, "_internal": "oculto"},
			{"cliente": "María López", "monto": 12000.0},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	pdf, err := RenderRedFlagReportPDF(rf)
	if err != nil {
		t.Fatalf("RenderRedFlagReportPDF: %v", err)
	}
	if len(pdf) == 0 {
		t.Fatal("expected non-empty PDF bytes")
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatalf("expected PDF signature, got: %q", pdf[:min(20, len(pdf))])
	}
}

func TestRenderRedFlagReportPDF_Aggregate(t *testing.T) {
	rf := &models.RedFlag{
		ID:           primitive.NewObjectID(),
		RuleName:     "Suma diaria excesiva",
		MonitorName:  "Transferencias",
		Severity:     models.SeverityCritical,
		Status:       models.RedFlagNew,
		RedFlagType:  models.RedFlagTypeAggregate,
		AggField:     "monto",
		AggFunction:  "sum",
		GroupByField: "cliente",
		GroupByValue: "Juan Pérez",
		AggValue:     55000,
		Threshold:    50000,
		MatchCount:   5,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	pdf, err := RenderRedFlagReportPDF(rf)
	if err != nil {
		t.Fatalf("RenderRedFlagReportPDF: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatalf("expected PDF signature, got: %q", pdf[:min(20, len(pdf))])
	}
}
