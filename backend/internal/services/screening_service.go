package services

import (
	"context"
	"fmt"
	"log"

	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ScreeningService orquesta un screening: busca en Watchman, clasifica,
// persiste. No decide CUÁNDO screenear (eso lo decide el caller — el
// motor de reglas para el automático, el handler para el manual).
type ScreeningService struct {
	client *WatchmanClient
	repo   *repository.ScreeningRepository
}

func NewScreeningService(client *WatchmanClient, repo *repository.ScreeningRepository) *ScreeningService {
	return &ScreeningService{client: client, repo: repo}
}

// extractScreeningValues toma, de los campos configurados en una regla,
// los que existen en matchedData, no están vacíos y son string — un
// monto o una fecha no es un nombre para screenear.
func extractScreeningValues(matchedData map[string]interface{}, fields []string) map[string]string {
	values := make(map[string]string)
	for _, field := range fields {
		raw, ok := matchedData[field]
		if !ok {
			continue
		}
		str, ok := raw.(string)
		if !ok || str == "" {
			continue
		}
		values[field] = str
	}
	return values
}

// ScreenAndPersist busca `query` en Watchman, clasifica el resultado y
// persiste un ScreeningResult. redFlagID es nulo para búsquedas manuales.
func (s *ScreeningService) ScreenAndPersist(ctx context.Context, redFlagID *primitive.ObjectID, field, query string) (*models.ScreeningResult, error) {
	matches, searchErr := s.client.Search(ctx, query, WatchmanSearchOptions{Limit: 20, MinMatch: 0.5})
	outcome := ClassifyScreeningOutcome(matches, searchErr)

	result := &models.ScreeningResult{
		Query:          query,
		RedFlagID:      redFlagID,
		Field:          field,
		Status:         outcome.Status,
		StrongestMatch: outcome.StrongestMatch,
		Matches:        matches,
	}
	if err := s.repo.Create(ctx, result); err != nil {
		return nil, fmt.Errorf("persistiendo screening result: %w", err)
	}
	return result, nil
}

// ScreenRuleFields dispara un ScreenAndPersist por cada campo configurado
// en la regla que tenga un valor screenable en matchedData — llamado por
// el motor de reglas en su propia goroutine (ver Tarea 6), nunca bloquea
// la creación del red flag.
func (s *ScreeningService) ScreenRuleFields(ctx context.Context, redFlagID primitive.ObjectID, screeningFields []string, matchedData map[string]interface{}) {
	values := extractScreeningValues(matchedData, screeningFields)
	for field, query := range values {
		if _, err := s.ScreenAndPersist(ctx, &redFlagID, field, query); err != nil {
			// No hay caller esperando esto (corre en su propia goroutine) —
			// se loguea y se sigue con el resto de los campos.
			logScreeningError(redFlagID, field, err)
		}
	}
}

func logScreeningError(redFlagID primitive.ObjectID, field string, err error) {
	log.Printf("WARNING: screening: campo %q del red flag %s: %v", field, redFlagID.Hex(), err)
}

// ScreenAndPersistEphemeral es la búsqueda manual: corre Search()+clasifica
// pero NO persiste — es una herramienta de investigación ad-hoc, no un
// registro de caso. Nombre distinto a ScreenAndPersist a propósito: que
// quede claro en el call site que este camino no deja rastro en Mongo.
func (s *ScreeningService) ScreenAndPersistEphemeral(ctx context.Context, query, dateOfBirth string) (*models.ScreeningResult, error) {
	matches, searchErr := s.client.Search(ctx, query, WatchmanSearchOptions{Limit: 20, MinMatch: 0.5, DateOfBirth: dateOfBirth})
	outcome := ClassifyScreeningOutcome(matches, searchErr)
	return &models.ScreeningResult{
		Query:          query,
		Status:         outcome.Status,
		StrongestMatch: outcome.StrongestMatch,
		Matches:        matches,
	}, nil
}
