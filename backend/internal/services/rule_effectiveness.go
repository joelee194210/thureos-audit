package services

import (
	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// EffectivenessStats resume qué tan ruidosa o certera es una regla (o una
// tipología, agregando las reglas que nacieron de ella): cuántas red flags
// generó, cómo se resolvieron, y qué tan rápido se cerraron.
type EffectivenessStats struct {
	Total             int     `json:"total"`
	Open              int     `json:"open"`
	FalsePositive     int     `json:"falsePositive"`
	ConfirmedROS      int     `json:"confirmedRos"`
	NoAction          int     `json:"noAction"`
	FalsePositiveRate float64 `json:"falsePositiveRate"` // sobre cerradas; 0 si ninguna cerró
	AvgCloseHours     float64 `json:"avgCloseHours"`     // sobre cerradas; 0 si ninguna cerró
}

// ComputeEffectiveness agrega un conjunto de red flags (todas de la misma
// regla, o de todas las reglas de una tipología) en EffectivenessStats.
// Pura y sin Mongo a propósito: el fetch vive en el repo, esto solo cuenta.
func ComputeEffectiveness(flags []models.RedFlag) EffectivenessStats {
	var s EffectivenessStats
	s.Total = len(flags)

	var closed int
	var totalCloseHours float64
	for _, f := range flags {
		if f.ClosedAt == nil {
			s.Open++
			continue
		}
		closed++
		totalCloseHours += f.ClosedAt.Sub(f.CreatedAt).Hours()
		switch f.Disposition {
		case models.DispositionFalsePositive:
			s.FalsePositive++
		case models.DispositionConfirmedROS:
			s.ConfirmedROS++
		case models.DispositionNoAction:
			s.NoAction++
		}
	}

	if closed > 0 {
		s.FalsePositiveRate = float64(s.FalsePositive) / float64(closed)
		s.AvgCloseHours = totalCloseHours / float64(closed)
	}
	return s
}

// GroupRuleIDsByTemplate agrupa los IDs de las reglas que nacieron de cada
// tipología (TemplateID vacío = regla manual, no cuenta para ninguna
// tipología). Pura, para poder testear el agrupado sin Mongo — el fetch de
// red flags por cada grupo vive en el repo.
func GroupRuleIDsByTemplate(rules []models.Rule) map[string][]primitive.ObjectID {
	groups := make(map[string][]primitive.ObjectID)
	for _, r := range rules {
		if r.TemplateID == "" {
			continue
		}
		groups[r.TemplateID] = append(groups[r.TemplateID], r.ID)
	}
	return groups
}
