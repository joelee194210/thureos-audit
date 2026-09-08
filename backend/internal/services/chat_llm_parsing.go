package services

import (
	"encoding/json"
	"fmt"

	"github.com/thureos/compliance/internal/models"
)

// chatQueryMaxLimit topea cuántas filas puede pedir una consulta simple
// (no agregada) del chatbot — nunca se le permite pedir todo el dataset.
const chatQueryMaxLimit = 1000

// ChatQueryInput es la forma exacta del "input" que el LLM manda al llamar
// a la herramienta query_monitor_data (ver el contrato en chat_service.go).
type ChatQueryInput struct {
	ConditionGroup *models.ConditionGroup `json:"conditionGroup,omitempty"`
	Aggregate      *ChatAggregateSpec     `json:"aggregate,omitempty"`
	Limit          int                    `json:"limit,omitempty"`
}

// ParseQueryToolInput valida y normaliza el input crudo del tool call.
// Nunca deja pasar un Limit fuera de rango ni un input sin conditionGroup
// ni aggregate.
func ParseQueryToolInput(raw json.RawMessage) (ChatQueryInput, error) {
	var input ChatQueryInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return ChatQueryInput{}, fmt.Errorf("input de query_monitor_data inválido: %w", err)
	}
	if input.ConditionGroup == nil && input.Aggregate == nil {
		return ChatQueryInput{}, fmt.Errorf("query_monitor_data requiere conditionGroup o aggregate")
	}
	if input.Aggregate != nil && input.Aggregate.Field == "" {
		return ChatQueryInput{}, fmt.Errorf("aggregate.field es obligatorio")
	}
	if input.Limit <= 0 || input.Limit > chatQueryMaxLimit {
		input.Limit = chatQueryMaxLimit
	}
	return input, nil
}

// ParseArtifact valida el bloque de artefacto que el LLM devuelve junto a
// su respuesta final. Un artefacto inválido nunca aborta la respuesta —
// el llamador (chat_service.go) trata un error acá como "sin artefacto",
// nunca como falla de toda la pregunta.
func ParseArtifact(raw json.RawMessage) (*models.ChatArtifact, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var artifact models.ChatArtifact
	if err := json.Unmarshal(raw, &artifact); err != nil {
		return nil, fmt.Errorf("artefacto inválido: %w", err)
	}
	switch artifact.Type {
	case models.ChatArtifactChart:
		if artifact.ChartSpec == nil || len(artifact.ChartSpec.Data) == 0 {
			return nil, fmt.Errorf("artefacto chart requiere chartSpec con datos")
		}
		switch artifact.ChartSpec.ChartType {
		case models.ChatChartBar, models.ChatChartLine, models.ChatChartPie:
		default:
			return nil, fmt.Errorf("chartType inválido: %q", artifact.ChartSpec.ChartType)
		}
	case models.ChatArtifactTable:
		if artifact.ChartSpec == nil || len(artifact.ChartSpec.Data) == 0 {
			return nil, fmt.Errorf("artefacto table requiere chartSpec con datos")
		}
	case models.ChatArtifactCustom:
		if artifact.Code == "" {
			return nil, fmt.Errorf("artefacto custom requiere code")
		}
	default:
		return nil, fmt.Errorf("tipo de artefacto inválido: %q", artifact.Type)
	}
	return &artifact, nil
}
