package services

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/thureos/compliance/internal/models"
)

// chatQueryMaxLimit topea cuántas filas puede pedir una consulta simple
// (no agregada) del chatbot — nunca se le permite pedir todo el dataset.
const chatQueryMaxLimit = 1000

// ParseQueryToolInput valida y normaliza el input crudo del tool call.
// Nunca deja pasar un Limit fuera de rango ni un input sin conditionGroup
// ni aggregate.
func ParseQueryToolInput(raw json.RawMessage) (ChatQueryInput, error) {
	var input ChatQueryInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return ChatQueryInput{}, fmt.Errorf("input de query_monitor_data inválido: %w", err)
	}
	input.Monitor = strings.TrimSpace(input.Monitor)
	if input.Monitor == "" {
		return ChatQueryInput{}, fmt.Errorf("query_monitor_data requiere 'monitor' (el alias del monitor a consultar)")
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

// llmArtifactInput es el shape angosto del bloque <artifact> que el LLM
// escribe junto a su respuesta. Contiene SOLO los campos que el modelo
// tiene motivo para proponer.
//
// El texto de ese bloque lo redacta el LLM, pero su salida está moldeada
// por el contenido de los CSV/Excel que el usuario sube — una inyección
// escondida en una celda puede terminar influyendo lo que el modelo
// escribe ahí. Si se deserializara directo en models.ChatArtifact (la
// entidad persistida), esa inyección podría colar "monitorIds" (la
// allowlist de autorización contra la que se resuelven los alias) o
// "saved":true/"savedName" (nada más pone Saved en false: eso plantaría
// el artefacto en la biblioteca del usuario sin pasar por el endpoint de
// guardado). Pasando por este DTO, esos campos no tienen dónde aterrizar
// — no hay nada que recordar sobreescribir.
type llmArtifactInput struct {
	Type      models.ChatArtifactType `json:"type"`
	Title     string                  `json:"title"`
	ChartSpec *models.ChartSpec       `json:"chartSpec,omitempty"`
	Code      string                  `json:"code,omitempty"`
	Sources   []models.ArtifactSource `json:"sources,omitempty"`
}

// ParseArtifact valida el bloque de artefacto que el LLM devuelve junto a
// su respuesta final. Un artefacto inválido nunca aborta la respuesta —
// el llamador (chat_service.go) trata un error acá como "sin artefacto",
// nunca como falla de toda la pregunta.
func ParseArtifact(raw json.RawMessage) (*models.ChatArtifact, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var input llmArtifactInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, fmt.Errorf("artefacto inválido: %w", err)
	}
	// Mapeo explícito a la entidad: todo lo que llmArtifactInput no trae
	// (ID, UserID, MonitorIDs, Saved, CachedData, ...) queda en su cero.
	artifact := models.ChatArtifact{
		Type:      input.Type,
		Title:     input.Title,
		ChartSpec: input.ChartSpec,
		Code:      input.Code,
		Sources:   input.Sources,
	}

	switch artifact.Type {
	case models.ChatArtifactChart, models.ChatArtifactTable:
		// Válido por cualquiera de los dos caminos: sources (se ejecuta) o
		// data inline (instantánea/legacy). Exigir data como antes
		// rechazaría todos los artefactos del contrato nuevo.
		hasData := artifact.ChartSpec != nil && len(artifact.ChartSpec.Data) > 0
		if len(artifact.Sources) == 0 && !hasData {
			return nil, fmt.Errorf("un artefacto %s requiere 'sources' o 'chartSpec.data'", artifact.Type)
		}
		for i := range artifact.Sources {
			src := &artifact.Sources[i]
			if strings.TrimSpace(src.Monitor) == "" {
				return nil, fmt.Errorf("la fuente %d no indica 'monitor'", i)
			}
			if src.Query.Monitor == "" {
				src.Query.Monitor = src.Monitor
			}
			// La query se valida con el mismo parser que el tool call: un
			// solo criterio de qué es una consulta bien formada. Se guarda
			// el resultado NORMALIZADO, no el crudo: RunArtifact (Task 6)
			// reconstruye el pipeline directo desde src.Query sin volver a
			// pasar por ParseQueryToolInput, así que si acá se descartara
			// la normalización, un alias con espacios o un limit fuera de
			// rango quedarían persistidos tal cual y solo se manifestarían
			// como bug al re-ejecutar la fuente.
			encoded, err := json.Marshal(src.Query)
			if err != nil {
				return nil, fmt.Errorf("la fuente %d tiene una query inválida: %w", i, err)
			}
			normalized, err := ParseQueryToolInput(encoded)
			if err != nil {
				return nil, fmt.Errorf("la fuente %d tiene una query inválida: %w", i, err)
			}
			src.Query = normalized
			// Si src.Monitor y src.Query.Monitor difirieran, se prefiere
			// el de la query: es el que de verdad se ejecuta.
			src.Monitor = normalized.Monitor
		}
		if artifact.Type == models.ChatArtifactChart {
			if artifact.ChartSpec == nil {
				return nil, fmt.Errorf("artefacto chart requiere chartSpec")
			}
			switch artifact.ChartSpec.ChartType {
			case models.ChatChartBar, models.ChatChartLine, models.ChatChartPie:
			default:
				return nil, fmt.Errorf("chartType inválido: %q", artifact.ChartSpec.ChartType)
			}
			// Varias fuentes se pivotean sobre XKey (una serie por
			// fuente); sin XKey no hay sobre qué pivotear.
			if len(artifact.Sources) > 1 && artifact.ChartSpec.XKey == "" {
				return nil, fmt.Errorf("un chart con varias fuentes requiere chartSpec.xKey")
			}
			// ...y sin YKeys no hay sobre qué pivotear tampoco: el valor
			// que el pivote lee de cada fuente es YKeys[0], así que con
			// YKeys vacío la clave de valor queda en "" y el gráfico sale
			// VACÍO sin que nadie se entere. Un fallo ruidoso acá es mejor
			// que un artefacto que se guarda y nunca dibuja nada.
			if len(artifact.Sources) > 1 && len(artifact.ChartSpec.YKeys) == 0 {
				return nil, fmt.Errorf("un chart con varias fuentes requiere chartSpec.yKeys")
			}
		}
	case models.ChatArtifactCustom:
		if artifact.Code == "" {
			return nil, fmt.Errorf("artefacto custom requiere code")
		}
		// Un custom trae su HTML/JS con los datos YA embebidos y nunca se
		// re-ejecuta (ver models.ChatArtifact.IsRerunnable) — 'sources' no
		// se valida en este caso (arriba, ese bloque es exclusivo de
		// chart/table), así que una fuente con monitor vacío pasaría sin
		// chequeo alguno. Se limpia en vez de rechazar el artefacto entero
		// por un campo que no le correspondía: lo que importa es que no
		// sobreviva para volver "re-ejecutable" (IsRerunnable == len(Sources) > 0)
		// algo que el spec dice explícitamente que no lo es.
		artifact.Sources = nil
	default:
		return nil, fmt.Errorf("tipo de artefacto inválido: %q", artifact.Type)
	}
	return &artifact, nil
}
