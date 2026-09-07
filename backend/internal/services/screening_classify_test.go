package services

import (
	"errors"
	"testing"

	"github.com/thureos/compliance/internal/models"
)

func TestClassifyScreeningOutcome_ErrorDelProveedorEsReviewNuncaClear(t *testing.T) {
	out := ClassifyScreeningOutcome(nil, errors.New("watchman caído"))
	if out.Status != models.ScreeningReview {
		t.Errorf("Status = %q, want %q (fail-closed)", out.Status, models.ScreeningReview)
	}
}

func TestClassifyScreeningOutcome_SinMatchesEsClear(t *testing.T) {
	out := ClassifyScreeningOutcome(nil, nil)
	if out.Status != models.ScreeningClear {
		t.Errorf("Status = %q, want %q", out.Status, models.ScreeningClear)
	}
	if out.StrongestMatch != "" {
		t.Errorf("StrongestMatch = %q, want vacío", out.StrongestMatch)
	}
}

func TestClassifyScreeningOutcome_MatchFuerteEsMatch(t *testing.T) {
	matches := []models.NormalizedMatch{
		{Name: "Juan Perez", Score: 0.6},
		{Name: "Juan Andres Perez", Score: 0.97}, // EXACT, no es el primero de la lista
	}
	out := ClassifyScreeningOutcome(matches, nil)
	if out.Status != models.ScreeningMatch {
		t.Errorf("Status = %q, want %q", out.Status, models.ScreeningMatch)
	}
	if out.StrongestMatch != "EXACT" {
		t.Errorf("StrongestMatch = %q, want EXACT", out.StrongestMatch)
	}
}

func TestClassifyScreeningOutcome_MatchDebilEsReview(t *testing.T) {
	matches := []models.NormalizedMatch{{Name: "Juan Perez", Score: 0.72}} // MEDIUM
	out := ClassifyScreeningOutcome(matches, nil)
	if out.Status != models.ScreeningReview {
		t.Errorf("Status = %q, want %q", out.Status, models.ScreeningReview)
	}
	if out.StrongestMatch != "MEDIUM" {
		t.Errorf("StrongestMatch = %q, want MEDIUM", out.StrongestMatch)
	}
}

func TestClassifyScreeningOutcome_UmbralesDeFuerza(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{0.95, "EXACT"},
		{0.85, "STRONG"},
		{0.7, "MEDIUM"},
		{0.1, "WEAK"},
	}
	for _, tc := range cases {
		out := ClassifyScreeningOutcome([]models.NormalizedMatch{{Score: tc.score}}, nil)
		if out.StrongestMatch != tc.want {
			t.Errorf("score %.2f: StrongestMatch = %q, want %q", tc.score, out.StrongestMatch, tc.want)
		}
	}
}
