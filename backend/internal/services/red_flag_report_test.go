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

	// Con función de agregación, la ficha SÍ lleva el bloque de agregación.
	rows := summaryRowsByLabel(redFlagSummaryRows(rf))
	if got := rows["Agregación"]; got != "SUM(monto) por cliente" {
		t.Errorf("Agregación = %q, want \"SUM(monto) por cliente\"", got)
	}
	for _, label := range []string{"Valor", "Umbral", "Grupo"} {
		if _, ok := rows[label]; !ok {
			t.Errorf("falta la fila %q en la ficha de una bandera agregada", label)
		}
	}
}

func summaryRowsByLabel(rows [][2]string) map[string]string {
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r[0]] = r[1]
	}
	return out
}

// Las banderas de las reglas de velocidad se guardan con RedFlagType
// "aggregate" pero sin AggField/AggFunction/AggValue/Threshold: el bloque de
// agregación se renderizaba igual, imprimiendo "Agregación: ( ) por tarjeta",
// "Valor: 0" y "Umbral: 0" en el PDF probatorio del caso.
func TestRedFlagSummaryRows_SinFuncionDeAgregacionOmiteElBloque(t *testing.T) {
	rf := &models.RedFlag{
		ID:           primitive.NewObjectID(),
		RuleName:     "dos compras en menos de 35s",
		MonitorName:  "Tarjetas",
		Severity:     models.SeverityHigh,
		Status:       models.RedFlagNew,
		RedFlagType:  models.RedFlagTypeAggregate,
		GroupByField: "tarjeta",
		GroupByValue: "T4111",
		MatchCount:   2,
		CreatedAt:    time.Now(),
	}

	rows := summaryRowsByLabel(redFlagSummaryRows(rf))
	for _, label := range []string{"Agregación", "Valor", "Umbral", "Sobre el umbral"} {
		if got, ok := rows[label]; ok {
			t.Errorf("la ficha no debería llevar %q sin función de agregación, got %q", label, got)
		}
	}
	// El grupo sí se conserva: es la entidad señalada por la regla.
	if got := rows["Grupo"]; got != "T4111" {
		t.Errorf("Grupo = %q, want \"T4111\"", got)
	}

	pdf, err := RenderRedFlagReportPDF(rf)
	if err != nil {
		t.Fatalf("RenderRedFlagReportPDF: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatalf("expected PDF signature, got: %q", pdf[:min(20, len(pdf))])
	}
}
