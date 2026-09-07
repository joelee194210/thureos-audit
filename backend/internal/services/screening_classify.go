package services

import "github.com/thureos/compliance/internal/models"

// ScreeningOutcome es la clasificación pura de un resultado de screening.
type ScreeningOutcome struct {
	Status         models.ScreeningStatus
	StrongestMatch string // EXACT | STRONG | MEDIUM | WEAK | "" si no hay matches
}

// matchStrength convierte un score 0.0-1.0 de Watchman a una categoría —
// mismos umbrales que la referencia de thureos-main.
func matchStrength(score float64) string {
	switch {
	case score >= 0.95:
		return "EXACT"
	case score >= 0.85:
		return "STRONG"
	case score >= 0.7:
		return "MEDIUM"
	default:
		return "WEAK"
	}
}

// ClassifyScreeningOutcome clasifica un resultado de screening:
//   - error del proveedor → REVIEW (fail-closed, nunca CLEAR)
//   - sin matches → CLEAR
//   - el match más fuerte es EXACT/STRONG → MATCH
//   - el match más fuerte es MEDIUM/WEAK → REVIEW
func ClassifyScreeningOutcome(matches []models.NormalizedMatch, searchErr error) ScreeningOutcome {
	if searchErr != nil {
		return ScreeningOutcome{Status: models.ScreeningReview}
	}
	if len(matches) == 0 {
		return ScreeningOutcome{Status: models.ScreeningClear}
	}

	strongest := matches[0]
	for _, m := range matches[1:] {
		if m.Score > strongest.Score {
			strongest = m
		}
	}
	strength := matchStrength(strongest.Score)
	status := models.ScreeningReview
	if strength == "EXACT" || strength == "STRONG" {
		status = models.ScreeningMatch
	}
	return ScreeningOutcome{Status: status, StrongestMatch: strength}
}
