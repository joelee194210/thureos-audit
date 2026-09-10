package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	anthropicparam "github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ErrConversationNotFound cubre tanto "no existe" como "no es tuya" — el
// handler nunca debe poder distinguir las dos por el mensaje de error, ni
// filtrar cuál de las dos ocurrió (ver Global Constraints: conversaciones
// privadas por usuario).
var ErrConversationNotFound = errors.New("conversación no encontrada")

// ErrMonitorNotFound cubre un monitor borrado mientras una conversación
// seguía apuntando a él — mismo criterio que ErrConversationNotFound:
// nunca se filtra el error crudo de Mongo en la respuesta.
var ErrMonitorNotFound = errors.New("monitor no encontrado")

// queryToolDescription y queryToolJSONSchema definen el contrato de la
// única herramienta que el chatbot expone — el mismo texto/schema se usa
// para Anthropic y DeepSeek (formas de request distintas, mismo contrato).
const queryToolDescription = `Ejecuta una consulta REAL contra los datos del monitor actual. Usar SIEMPRE que la pregunta necesite datos reales (conteos, sumas, promedios, filtros, listados) — nunca inventar números. Se puede llamar varias veces si hace falta investigar en pasos (ej. primero contar, después agrupar por región si el total es alto).

Para traer filas usar "conditionGroup" (opcional "limit", tope 1000). Para totales agrupados (suma/conteo/promedio/mín/máx por categoría) usar "aggregate" — SIEMPRE devuelve TODOS los grupos, no hace falta pedir un umbral.

Cada llamada consulta UN monitor: indicá cuál en "monitor" con su alias exacto. Para comparar entre monitores, llamá una vez por cada uno.`

var queryToolJSONSchema = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"monitor": map[string]interface{}{
			"type":        "string",
			"description": "Alias del monitor a consultar. Debe ser uno de los alias listados en el prompt.",
		},
		"conditionGroup": map[string]interface{}{
			"type":        "object",
			"description": "Filtro simple para traer filas. Omitir si se usa 'aggregate'.",
			"properties": map[string]interface{}{
				"logic": map[string]interface{}{"type": "string", "enum": []string{"AND", "OR"}},
				"conditions": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"field":    map[string]interface{}{"type": "string"},
							"operator": map[string]interface{}{"type": "string", "enum": []string{"eq", "neq", "gt", "lt", "gte", "lte", "contains", "regex", "in", "not_in", "between", "is_null", "is_not_null", "starts_with", "ends_with"}},
							"value":    map[string]interface{}{},
						},
						"required": []string{"field", "operator", "value"},
					},
				},
			},
		},
		"aggregate": map[string]interface{}{
			"type":        "object",
			"description": "Agregación agrupada. Devuelve TODOS los grupos, sin importar su valor.",
			"properties": map[string]interface{}{
				"field":      map[string]interface{}{"type": "string"},
				"function":   map[string]interface{}{"type": "string", "enum": []string{"sum", "count", "avg", "min", "max"}},
				"groupBy":    map[string]interface{}{"type": "string"},
				"timeField":  map[string]interface{}{"type": "string"},
				"timeWindow": map[string]interface{}{"type": "string", "description": "ej. '24h', '7d', '30d'"},
			},
			"required": []string{"field", "function"},
		},
		"limit": map[string]interface{}{
			"type":        "integer",
			"description": "Tope de filas para conditionGroup (máx 1000).",
		},
	},
	"required": []string{"monitor"},
}

var artifactBlockRe = regexp.MustCompile(`(?s)<artifact>\s*(.*?)\s*</artifact>`)

// deepSeekSpecialTokenRune ('｜', U+FF5C — barra vertical fullwidth) es el
// delimitador que DeepSeek usa para sus tokens especiales internos (ej.
// "<｜｜DSML｜｜tool_calls>..."), que a veces se filtran como texto crudo en
// "content" en vez de ir por el campo estructurado tool_calls — un quirk
// conocido de su API, no algo arreglable del lado del proveedor. Un texto
// normal en español nunca contiene este carácter, así que su sola
// presencia basta para detectar el leak sin depender de la forma exacta
// del tag. Nunca se le muestra esto al usuario: se trata como respuesta
// inválida, igual que un JSON roto.
const deepSeekSpecialTokenRune = '｜'

func hasLeakedDeepSeekToolSyntax(content string) bool {
	return strings.ContainsRune(content, deepSeekSpecialTokenRune)
}

const chatSystemPromptTemplate = `Sos un analista de datos que responde preguntas sobre los monitores listados abajo.

%s
Tenés una herramienta, query_monitor_data, para traer datos REALES — nunca
inventes números. Usala todas las veces que necesites para responder bien.
Cada llamada consulta UN monitor: indicá cuál en el campo "monitor" usando
su alias exacto de la lista. Para comparar entre monitores, llamá una vez
por cada uno y correlacioná los resultados al responder.

Si la respuesta se presta a visualizarse (series de tiempo, comparaciones,
rankings, distribuciones), agregá al FINAL de tu respuesta un bloque
delimitado exactamente así, con un JSON válido dentro:

<artifact>
{"type": "chart", "title": "...", "chartSpec": {"chartType": "bar", "data": [{"x": "...", "y": 1}], "xKey": "x", "yKeys": ["y"]}}
</artifact>

Para una tabla simple: "type": "table" con el mismo "chartSpec" (sin
chartType). Si necesitás algo que un gráfico de barra/línea/torta no puede
expresar, usá "type": "custom" con "code": HTML/JS autocontenido que
renderice la visualización — ese código corre aislado en un iframe sin
acceso a la sesión ni a la red, así que tiene que traer sus propios datos
embebidos en el propio "code" (no puede hacer fetch a nada).

Si no hace falta artefacto, no incluyas el bloque <artifact>. Respondé
siempre en español.`

// buildSystemPrompt emite un bloque de schema por monitor, encabezado por
// el alias con el que el LLM debe nombrarlo.
//
// Recorre los alias ORDENADOS: el orden de recorrido de un mapa en Go es
// aleatorio, y un prompt que cambia de texto en cada turno invalida el
// caché de prompt del proveedor y hace irreproducible cualquier bug.
func buildSystemPrompt(aliases map[string]*models.Monitor) string {
	names := make([]string, 0, len(aliases))
	for alias := range aliases {
		names = append(names, alias)
	}
	sort.Strings(names)

	var blocks strings.Builder
	blocks.WriteString("Monitores disponibles (usá el alias en el campo \"monitor\"):\n\n")
	for _, alias := range names {
		monitor := aliases[alias]
		fmt.Fprintf(&blocks, "### %s — %q\n", alias, monitor.Name)
		for _, f := range monitor.Schema {
			fmt.Fprintf(&blocks, "- %s: %s\n", f.Name, f.Type)
		}
		blocks.WriteString("\n")
	}
	return fmt.Sprintf(chatSystemPromptTemplate, blocks.String())
}

// chatConversationTitleMaxLen topea el título autogenerado — el primer
// mensaje del usuario puede ser largo, el título es solo para la lista.
const chatConversationTitleMaxLen = 60

// chatConversationTitle arma el título de una conversación a partir de su
// primera pregunta — trunca sin cortar una palabra a la mitad cuando es
// posible.
func chatConversationTitle(firstMessage string) string {
	trimmed := strings.TrimSpace(firstMessage)
	if len(trimmed) <= chatConversationTitleMaxLen {
		return trimmed
	}
	cut := trimmed[:chatConversationTitleMaxLen]
	if idx := strings.LastIndex(cut, " "); idx > 0 {
		cut = cut[:idx]
	}
	return cut + "…"
}

// extractArtifactAndText separa el bloque <artifact> (si lo hay) del texto
// de la respuesta. Un artefacto que no parsea se descarta sin abortar el
// texto — ver Global Constraints.
func extractArtifactAndText(raw string) (string, *models.ChatArtifact) {
	match := artifactBlockRe.FindStringSubmatch(raw)
	if match == nil {
		return strings.TrimSpace(raw), nil
	}
	text := strings.TrimSpace(artifactBlockRe.ReplaceAllString(raw, ""))
	artifact, err := ParseArtifact(json.RawMessage(match[1]))
	if err != nil {
		return text, nil
	}
	return text, artifact
}

type ChatService struct {
	chatRepo    *repository.ChatRepository
	monitorRepo *repository.MonitorRepository
	configRepo  *repository.SystemConfigRepository
}

func NewChatService(chatRepo *repository.ChatRepository, monitorRepo *repository.MonitorRepository, configRepo *repository.SystemConfigRepository) *ChatService {
	return &ChatService{chatRepo: chatRepo, monitorRepo: monitorRepo, configRepo: configRepo}
}

// executeQuery ejecuta lo que el LLM pidió vía query_monitor_data contra
// los datos reales del monitor que nombró, y devuelve el resultado
// serializado.
//
// El alias se resuelve contra `aliases` — la allowlist de la conversación
// — ANTES de tocar Mongo. Ver resolveQueryMonitor y Global Constraints.
func (s *ChatService) executeQuery(ctx context.Context, aliases map[string]*models.Monitor, raw json.RawMessage) (string, error) {
	input, err := ParseQueryToolInput(raw)
	if err != nil {
		return "", err
	}

	monitor, err := resolveQueryMonitor(aliases, input.Monitor)
	if err != nil {
		return "", err
	}

	if input.Aggregate != nil {
		pipeline := buildDataAggregatePipeline(*input.Aggregate)
		results, err := s.monitorRepo.AggregateData(ctx, monitor.CollectionID, pipeline)
		if err != nil {
			return "", fmt.Errorf("ejecutando agregación: %w", err)
		}
		out, _ := json.Marshal(results)
		return string(out), nil
	}

	filter := bson.M{}
	if input.ConditionGroup != nil {
		filter = BuildMongoFilter(*input.ConditionGroup)
	}
	results, err := s.monitorRepo.QueryData(ctx, monitor.CollectionID, filter, int64(input.Limit))
	if err != nil {
		return "", fmt.Errorf("ejecutando consulta: %w", err)
	}
	out, _ := json.Marshal(results)
	return string(out), nil
}

// Ask procesa una pregunta del usuario: la persiste, arma/loopea la
// llamada al proveedor configurado, y persiste + devuelve la respuesta.
// conversation.UserID != userID nunca debe ocurrir salvo que alguien
// intente leer la conversación de otro usuario — se trata como "no
// encontrada", no se filtra la existencia (ver Global Constraints).
func (s *ChatService) Ask(ctx context.Context, conversationID, userID primitive.ObjectID, userMessage string) (models.ChatMessage, error) {
	// La conversación se lee ANTES de fijar el timeout: cuántos monitores
	// abarca es lo que define cuánto puede tardar.
	conv, err := s.chatRepo.GetConversation(ctx, conversationID)
	if err != nil {
		return models.ChatMessage{}, ErrConversationNotFound
	}
	if conv.UserID != userID {
		return models.ChatMessage{}, ErrConversationNotFound
	}

	ctx, cancel := context.WithTimeout(ctx, chatAskTimeout(len(conv.MonitorIDs)))
	defer cancel()

	// Un monitor borrado mientras la conversación seguía apuntando a él se
	// excluye en silencio: la conversación sigue sirviendo con el resto.
	// Solo si no queda ninguno es un error.
	monitors := make([]models.Monitor, 0, len(conv.MonitorIDs))
	for _, id := range conv.MonitorIDs {
		monitor, err := s.monitorRepo.FindByID(ctx, id)
		if err != nil {
			continue
		}
		monitors = append(monitors, *monitor)
	}
	if len(monitors) == 0 {
		return models.ChatMessage{}, ErrMonitorNotFound
	}
	aliases := buildMonitorAliases(monitors)

	history, err := s.chatRepo.ListMessagesByConversation(ctx, conversationID)
	if err != nil {
		return models.ChatMessage{}, fmt.Errorf("cargando historial: %w", err)
	}

	isFirstMessage := len(history) == 0

	userMsg := models.ChatMessage{ConversationID: conversationID, Role: models.ChatRoleUser, Content: userMessage}
	if err := s.chatRepo.CreateMessage(ctx, &userMsg); err != nil {
		return models.ChatMessage{}, fmt.Errorf("guardando mensaje: %w", err)
	}
	history = append(history, userMsg)

	// El título se autogenera una sola vez, a partir de la primera
	// pregunta — no hay UI de renombrado (no se pidió, ver el spec).
	if isFirstMessage {
		_ = s.chatRepo.SetTitle(ctx, conversationID, chatConversationTitle(userMessage))
	}

	cfg, err := s.configRepo.Get(ctx)
	if err != nil {
		return models.ChatMessage{}, fmt.Errorf("cargando config de IA: %w", err)
	}
	if cfg.AI.APIKey == "" {
		assistantMsg := models.ChatMessage{
			ConversationID: conversationID,
			Role:           models.ChatRoleAssistant,
			Content:        "Configurá una API key de IA en Configuración → APIs para poder usar el chatbot.",
		}
		if err := s.chatRepo.CreateMessage(ctx, &assistantMsg); err != nil {
			return models.ChatMessage{}, fmt.Errorf("guardando respuesta: %w", err)
		}
		return assistantMsg, nil
	}

	var text string
	var artifact *models.ChatArtifact
	if cfg.AI.Provider == models.AIProviderDeepSeek {
		text, artifact, err = s.askDeepSeek(ctx, cfg.AI, aliases, historyToDeepSeek(history))
	} else {
		text, artifact, err = s.askAnthropic(ctx, cfg.AI, aliases, historyToAnthropic(history))
	}
	if err != nil {
		assistantMsg := models.ChatMessage{
			ConversationID: conversationID,
			Role:           models.ChatRoleAssistant,
			Content:        fmt.Sprintf("Hubo un error consultando al proveedor de IA: %s", err.Error()),
		}
		if err := s.chatRepo.CreateMessage(ctx, &assistantMsg); err != nil {
			return models.ChatMessage{}, fmt.Errorf("guardando respuesta: %w", err)
		}
		return assistantMsg, nil
	}

	assistantMsg := models.ChatMessage{
		ConversationID: conversationID,
		Role:           models.ChatRoleAssistant,
		Content:        text,
		Artifact:       artifact,
	}
	if err := s.chatRepo.CreateMessage(ctx, &assistantMsg); err != nil {
		return models.ChatMessage{}, fmt.Errorf("guardando respuesta: %w", err)
	}
	_ = s.chatRepo.TouchConversation(ctx, conversationID)
	return assistantMsg, nil
}

// ---------------------------------------------------------------------------
// Anthropic
// ---------------------------------------------------------------------------

func historyToAnthropic(history []models.ChatMessage) []anthropic.MessageParam {
	messages := make([]anthropic.MessageParam, 0, len(history))
	for _, m := range history {
		block := anthropic.NewTextBlock(m.Content)
		if m.Role == models.ChatRoleAssistant {
			messages = append(messages, anthropic.NewAssistantMessage(block))
		} else {
			messages = append(messages, anthropic.NewUserMessage(block))
		}
	}
	return messages
}

func (s *ChatService) askAnthropic(ctx context.Context, ai models.AIConfig, aliases map[string]*models.Monitor, history []anthropic.MessageParam) (string, *models.ChatArtifact, error) {
	client := anthropic.NewClient(option.WithAPIKey(ai.APIKey))
	model := ai.Model
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}

	tool := anthropic.ToolUnionParam{OfTool: &anthropic.ToolParam{
		Name:        "query_monitor_data",
		Description: anthropicparam.NewOpt(queryToolDescription),
		// El "required" de queryToolJSONSchema no viaja acá: el
		// ToolInputSchemaParam del SDK pinneado (v0.2.0-beta.3) no expone
		// ese campo. DeepSeek sí lo recibe, porque se le manda el schema
		// entero como Parameters. El costo del faltante es una iteración
		// perdida, no un error: ParseQueryToolInput rechaza un input sin
		// "monitor" y el LLM se corrige en la vuelta siguiente.
		InputSchema: anthropic.ToolInputSchemaParam{
			Type:       "object",
			Properties: queryToolJSONSchema["properties"],
		},
	}}

	systemPrompt := buildSystemPrompt(aliases)
	messages := append([]anthropic.MessageParam{}, history...)

	for i := 0; i < chatToolIterations(len(aliases)); i++ {
		message, err := client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     model,
			MaxTokens: 4096,
			System:    []anthropic.TextBlockParam{{Text: systemPrompt, Type: "text"}},
			Messages:  messages,
			Tools:     []anthropic.ToolUnionParam{tool},
		})
		if err != nil {
			return "", nil, fmt.Errorf("calling Anthropic API: %w", err)
		}

		if message.StopReason != anthropic.MessageStopReasonToolUse {
			return extractAnthropicFinalAnswer(message)
		}

		assistantBlocks := make([]anthropic.ContentBlockParamUnion, 0, len(message.Content))
		var toolResults []anthropic.ContentBlockParamUnion
		for _, block := range message.Content {
			assistantBlocks = append(assistantBlocks, block.ToParam())
			if block.Type == "tool_use" {
				result, err := s.executeQuery(ctx, aliases, block.Input)
				content := result
				isError := false
				if err != nil {
					content = err.Error()
					isError = true
				}
				toolResults = append(toolResults, anthropic.NewToolResultBlock(block.ID, content, isError))
			}
		}
		messages = append(messages, anthropic.NewAssistantMessage(assistantBlocks...))
		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}

	// Se alcanzó el tope de iteraciones sin una respuesta final — una
	// última llamada SIN el tool para forzar una respuesta de texto con lo
	// que el LLM ya tenga, en vez de devolver un error crudo.
	message, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: 4096,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt, Type: "text"}},
		Messages:  messages,
	})
	if err != nil {
		return "", nil, fmt.Errorf("se alcanzó el máximo de %d consultas y no se pudo obtener una respuesta final: %w", chatToolIterations(len(aliases)), err)
	}
	return extractAnthropicFinalAnswer(message)
}

// chatEmptyArtifactOnlyText es el placeholder cuando la respuesta del LLM
// es solo el bloque <artifact> sin texto alrededor — nunca se persiste un
// ChatMessage con Content vacío (en Anthropic, reenviar un bloque de texto
// vacío en el historial del turno siguiente puede romper la conversación
// para siempre; en DeepSeek, un texto vacío hoy descarta un artefacto
// válido con un error).
const chatEmptyArtifactOnlyText = "Generé la visualización que se muestra en el panel."

func finalizeArtifactText(text string, artifact *models.ChatArtifact) string {
	if text == "" && artifact != nil {
		return chatEmptyArtifactOnlyText
	}
	return text
}

func extractAnthropicFinalAnswer(message *anthropic.Message) (string, *models.ChatArtifact, error) {
	var sb strings.Builder
	for _, block := range message.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}
	if sb.Len() == 0 {
		return "", nil, fmt.Errorf("respuesta vacía del proveedor")
	}
	text, artifact := extractArtifactAndText(sb.String())
	return finalizeArtifactText(text, artifact), artifact, nil
}

// ---------------------------------------------------------------------------
// DeepSeek — API compatible con OpenAI, tipos propios del chat (no se
// tocan los de ai_rules_service.go, que no necesitan tool-calling).
// ---------------------------------------------------------------------------

type chatDeepSeekToolFunc struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type chatDeepSeekTool struct {
	Type     string               `json:"type"`
	Function chatDeepSeekToolFunc `json:"function"`
}

type chatDeepSeekToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatDeepSeekToolCall struct {
	ID       string                   `json:"id"`
	Type     string                   `json:"type"`
	Function chatDeepSeekToolCallFunc `json:"function"`
}

type chatDeepSeekMessage struct {
	Role       string                 `json:"role"`
	Content    string                 `json:"content,omitempty"`
	ToolCalls  []chatDeepSeekToolCall `json:"tool_calls,omitempty"`
	ToolCallID string                 `json:"tool_call_id,omitempty"`
}

type chatDeepSeekRequest struct {
	Model     string                `json:"model"`
	Messages  []chatDeepSeekMessage `json:"messages"`
	Tools     []chatDeepSeekTool    `json:"tools,omitempty"`
	MaxTokens int                   `json:"max_tokens"`
}

type chatDeepSeekResponse struct {
	Choices []struct {
		Message      chatDeepSeekMessage `json:"message"`
		FinishReason string              `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func historyToDeepSeek(history []models.ChatMessage) []chatDeepSeekMessage {
	messages := make([]chatDeepSeekMessage, 0, len(history))
	for _, m := range history {
		messages = append(messages, chatDeepSeekMessage{Role: string(m.Role), Content: m.Content})
	}
	return messages
}

func (s *ChatService) askDeepSeek(ctx context.Context, ai models.AIConfig, aliases map[string]*models.Monitor, history []chatDeepSeekMessage) (string, *models.ChatArtifact, error) {
	baseURL := ai.BaseURL
	if baseURL == "" {
		baseURL = models.DefaultAIBaseURLs[models.AIProviderDeepSeek]
	}
	model := ai.Model
	if model == "" {
		model = "deepseek-chat"
	}

	tool := chatDeepSeekTool{
		Type: "function",
		Function: chatDeepSeekToolFunc{
			Name:        "query_monitor_data",
			Description: queryToolDescription,
			Parameters:  queryToolJSONSchema,
		},
	}

	messages := append([]chatDeepSeekMessage{
		{Role: "system", Content: buildSystemPrompt(aliases)},
	}, history...)

	for i := 0; i < chatToolIterations(len(aliases)); i++ {
		reqBody := chatDeepSeekRequest{Model: model, Messages: messages, Tools: []chatDeepSeekTool{tool}, MaxTokens: 4096}
		body, err := json.Marshal(reqBody)
		if err != nil {
			return "", nil, fmt.Errorf("armando la petición a DeepSeek: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			return "", nil, fmt.Errorf("armando la petición a DeepSeek: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+ai.APIKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", nil, fmt.Errorf("llamando a DeepSeek: %w", err)
		}
		var parsed chatDeepSeekResponse
		decodeErr := json.NewDecoder(resp.Body).Decode(&parsed)
		_ = resp.Body.Close()
		if decodeErr != nil {
			return "", nil, fmt.Errorf("la respuesta de DeepSeek no es JSON válido: %w", decodeErr)
		}
		if parsed.Error != nil {
			return "", nil, fmt.Errorf("deepseek: %s", parsed.Error.Message)
		}
		if len(parsed.Choices) == 0 {
			return "", nil, fmt.Errorf("respuesta vacía de deepseek")
		}
		choice := parsed.Choices[0]

		if choice.FinishReason != "tool_calls" || len(choice.Message.ToolCalls) == 0 {
			if hasLeakedDeepSeekToolSyntax(choice.Message.Content) {
				return "", nil, fmt.Errorf("el proveedor devolvió una respuesta con formato interno inválido")
			}
			text, artifact := extractArtifactAndText(choice.Message.Content)
			text = finalizeArtifactText(text, artifact)
			if text == "" {
				return "", nil, fmt.Errorf("respuesta vacía del proveedor")
			}
			return text, artifact, nil
		}

		messages = append(messages, choice.Message)
		for _, call := range choice.Message.ToolCalls {
			result, err := s.executeQuery(ctx, aliases, json.RawMessage(call.Function.Arguments))
			content := result
			if err != nil {
				content = err.Error()
			}
			messages = append(messages, chatDeepSeekMessage{
				Role:       "tool",
				Content:    content,
				ToolCallID: call.ID,
			})
		}
	}

	// Se alcanzó el tope de iteraciones sin una respuesta final — una
	// última petición SIN el tool para forzar una respuesta de texto con
	// lo que el LLM ya tenga, en vez de devolver un error crudo.
	reqBody := chatDeepSeekRequest{Model: model, Messages: messages, MaxTokens: 4096}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, fmt.Errorf("se alcanzó el máximo de %d consultas y no se pudo armar la respuesta final: %w", chatToolIterations(len(aliases)), err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", nil, fmt.Errorf("se alcanzó el máximo de %d consultas y no se pudo armar la respuesta final: %w", chatToolIterations(len(aliases)), err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ai.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("se alcanzó el máximo de %d consultas y no se pudo obtener una respuesta final: %w", chatToolIterations(len(aliases)), err)
	}
	var parsed chatDeepSeekResponse
	decodeErr := json.NewDecoder(resp.Body).Decode(&parsed)
	_ = resp.Body.Close()
	if decodeErr != nil {
		return "", nil, fmt.Errorf("se alcanzó el máximo de %d consultas y la respuesta final no es JSON válido: %w", chatToolIterations(len(aliases)), decodeErr)
	}
	if parsed.Error != nil {
		return "", nil, fmt.Errorf("se alcanzó el máximo de %d consultas — deepseek: %s", chatToolIterations(len(aliases)), parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", nil, fmt.Errorf("se alcanzó el máximo de %d consultas y la respuesta final vino vacía", chatToolIterations(len(aliases)))
	}
	if hasLeakedDeepSeekToolSyntax(parsed.Choices[0].Message.Content) {
		return "", nil, fmt.Errorf("se alcanzó el máximo de %d consultas y la respuesta final vino con formato interno inválido", chatToolIterations(len(aliases)))
	}
	text, artifact := extractArtifactAndText(parsed.Choices[0].Message.Content)
	text = finalizeArtifactText(text, artifact)
	if text == "" {
		return "", nil, fmt.Errorf("se alcanzó el máximo de %d consultas y la respuesta final vino vacía", chatToolIterations(len(aliases)))
	}
	return text, artifact, nil
}
