package services

import (
	"testing"
	"time"

	"github.com/thureos/compliance/internal/models"
)

func TestCanTransition_Legales(t *testing.T) {
	legales := []struct {
		from, to models.RedFlagStatus
	}{
		{models.RedFlagNew, models.RedFlagAcknowledged},
		{models.RedFlagNew, models.RedFlagResolved},
		{models.RedFlagNew, models.RedFlagDismissed},
		{models.RedFlagAcknowledged, models.RedFlagEscalated},
		{models.RedFlagAcknowledged, models.RedFlagResolved},
		{models.RedFlagAcknowledged, models.RedFlagDismissed},
		{models.RedFlagEscalated, models.RedFlagAcknowledged}, // de-escalar
		{models.RedFlagEscalated, models.RedFlagResolved},
		{models.RedFlagEscalated, models.RedFlagDismissed},
	}
	for _, tc := range legales {
		if !CanTransition(tc.from, tc.to) {
			t.Errorf("%s → %s debería ser legal", tc.from, tc.to)
		}
	}
}

func TestCanTransition_Ilegales(t *testing.T) {
	ilegales := []struct {
		from, to models.RedFlagStatus
	}{
		{models.RedFlagResolved, models.RedFlagNew},        // terminal
		{models.RedFlagResolved, models.RedFlagEscalated},  // terminal
		{models.RedFlagDismissed, models.RedFlagNew},       // terminal
		{models.RedFlagNew, models.RedFlagNew},             // auto-loop
		{models.RedFlagAcknowledged, models.RedFlagNew},    // hacia atrás
		{models.RedFlagEscalated, models.RedFlagEscalated}, // auto-loop
	}
	for _, tc := range ilegales {
		if CanTransition(tc.from, tc.to) {
			t.Errorf("%s → %s debería ser ilegal", tc.from, tc.to)
		}
	}
}

// El cierre exige disposition; una transición no-cierre la rechaza.
func TestValidTransition_DispositionSoloAlCerrar(t *testing.T) {
	if err := ValidTransition(models.RedFlagNew, models.RedFlagResolved, ""); err == nil {
		t.Error("cerrar sin disposition debe fallar")
	}
	if err := ValidTransition(models.RedFlagNew, models.RedFlagResolved, models.DispositionConfirmedROS); err != nil {
		t.Errorf("cerrar con confirmed_ros: %v", err)
	}
	if err := ValidTransition(models.RedFlagNew, models.RedFlagAcknowledged, models.DispositionConfirmedROS); err == nil {
		t.Error("transitionar a acknowledged con disposition debe fallar")
	}
	if err := ValidTransition(models.RedFlagNew, models.RedFlagResolved, "cualquiera"); err == nil {
		t.Error("disposition inválida al cerrar debe fallar")
	}
}

func baseTime() time.Time { return time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC) }

func TestSLADueAt_Y_PriorityPorSeveridad(t *testing.T) {
	cases := []struct {
		sev      models.Severity
		priority int
		due      int64 // horas
	}{
		{models.SeverityCritical, 1, 24},
		{models.SeverityHigh, 2, 72},
		{models.SeverityMedium, 3, 168},
		{models.SeverityLow, 4, 720},
	}
	for _, tc := range cases {
		rf := &models.RedFlag{Severity: tc.sev, CreatedAt: baseTime()}
		due := SLADueAt(rf)
		hours := int64(due.Sub(rf.CreatedAt).Hours())
		// margen por redondeo de horas float
		if hours < tc.due-1 || hours > tc.due+1 {
			t.Errorf("%s: SLA = %dh, want ~%dh", tc.sev, hours, tc.due)
		}
		if got := PriorityForSeverity(tc.sev); got != tc.priority {
			t.Errorf("%s: priority = %d, want %d", tc.sev, got, tc.priority)
		}
	}
}
