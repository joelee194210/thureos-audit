package services

import (
	"fmt"
	"time"

	"github.com/thureos/compliance/internal/models"
)

// Transiciones legales del ciclo de vida de un caso. Extendemos el status
// existente: acknowledged ES "en investigación" (quien reconoce queda
// asignado), escalated es el nuevo estado de escalamiento. Los estados
// cerrados (resolved/dismissed) son terminales.
var caseTransitions = map[models.RedFlagStatus][]models.RedFlagStatus{
	models.RedFlagNew:          {models.RedFlagAcknowledged, models.RedFlagResolved, models.RedFlagDismissed},
	models.RedFlagAcknowledged: {models.RedFlagEscalated, models.RedFlagResolved, models.RedFlagDismissed},
	models.RedFlagEscalated:    {models.RedFlagAcknowledged, models.RedFlagResolved, models.RedFlagDismissed},
}

// CanTransition dice si el salto de estado es legal.
func CanTransition(from, to models.RedFlagStatus) bool {
	for _, next := range caseTransitions[from] {
		if next == to {
			return true
		}
	}
	return false
}

// DispositionRequiredForClose: toda transición a estado cerrado exige la
// disposición explícita — es la evidencia que auditoría y el futuro
// módulo regulatorio van a consumir.
func DispositionRequiredForClose(to models.RedFlagStatus) bool {
	return to == models.RedFlagResolved || to == models.RedFlagDismissed
}

// ValidTransition valida un salto de estado y devuelve la disposition
// requerida ("", si no aplica) o el error que explica por qué no.
func ValidTransition(from, to models.RedFlagStatus, disposition models.RedFlagDisposition) error {
	if !CanTransition(from, to) {
		return fmt.Errorf("transición de estado %q → %q no permitida", from, to)
	}
	if DispositionRequiredForClose(to) {
		switch disposition {
		case models.DispositionFalsePositive, models.DispositionConfirmedROS, models.DispositionNoAction:
			return nil
		default:
			return fmt.Errorf("cerrar con estado %q exige disposition (false_positive, confirmed_ros o no_action)", to)
		}
	}
	if disposition != "" {
		return fmt.Errorf("la disposition solo se registra al cerrar el caso, no en %q", to)
	}
	return nil
}

// TransitionSideEffects son los campos que un cierre legal setea en el
// red flag (para que el repo haga un único UpdateByID).
type TransitionSideEffects struct {
	Disposition models.RedFlagDisposition
	ClosedAt    *time.Time
}

// SLADueAt calcula el vencimiento del SLA de primera respuesta desde la
// creación de la alerta, según su severidad.
func SLADueAt(rf *models.RedFlag) time.Time {
	return rf.CreatedAt.Add(models.SLADefaultForSeverity(rf.Severity))
}

// PriorityForSeverity deriva la prioridad de cola (1 = más urgente) de la
// severidad de la alerta.
func PriorityForSeverity(s models.Severity) int {
	switch s {
	case models.SeverityCritical:
		return 1
	case models.SeverityHigh:
		return 2
	case models.SeverityMedium:
		return 3
	default:
		return 4
	}
}
