package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
)

// aiRequestTimeout acota cuánto puede tardar una llamada al proveedor de IA.
// Sin deadline, la cadena entera queda colgada si el proveedor no responde:
// http.DefaultClient no tiene timeout, el contexto de Fiber tampoco, y el
// frontend se queda en "Generando..." para siempre sin mostrar error. Las
// llamadas reales tardan 7-9s; 90s deja margen amplio sin dejar de acotar.
const aiRequestTimeout = 90 * time.Second

// AIRuleSuggestion represents a single AI-generated rule suggestion.
type AIRuleSuggestion struct {
	Name                string                      `json:"name"`
	Description         string                      `json:"description"`
	ConditionGroup      models.ConditionGroup       `json:"conditionGroup"`
	AggregateConditions []models.AggregateCondition `json:"aggregateConditions,omitempty"`
	Severity            models.Severity             `json:"severity"`
	Actions             []models.ActionType         `json:"actions"`
	Reasoning           string                      `json:"reasoning"`
}

// systemPrompt is shared across all providers — the contract stays the same.
// The product has no language switcher anywhere (UI, settings, user prefs
// are all Spanish-only), so "match the system's language" means Spanish,
// full stop — there's no other language to match.
const systemPrompt = `You are a data monitoring rules expert. Given a data schema and optional sample data, generate monitoring rules that detect anomalies, suspicious patterns, or business-critical conditions.

Write every "name", "description", and "reasoning" value in Spanish — the product this feeds into has no language switcher and is Spanish-only throughout. Field names, operators, and JSON keys stay in English exactly as specified below.

Return ONLY a JSON array of rule suggestions. Each rule must follow this exact structure:
{
  "name": "Nombre de la regla, en español",
  "description": "Qué detecta esta regla, en español",
  "conditionGroup": {
    "logic": "AND" or "OR",
    "conditions": [
      {"field": "field_name", "operator": "op", "value": value}
    ]
  },
  "aggregateConditions": [
    {
      "field": "field_name",
      "function": "sum|count|avg|min|max",
      "groupBy": "field_name",
      "timeField": "field_name",
      "timeWindow": "60s|5min|24h|7d",
      "operator": "op",
      "threshold": value
    }
  ],
  "severity": "low|medium|high|critical",
  "actions": ["red_flag", "flag", "log"],
  "reasoning": "Por qué esta regla es importante, en español"
}

"conditionGroup" is REQUIRED (use an empty "conditions" array if the rule is
purely aggregate-based). "aggregateConditions" is OPTIONAL — omit it entirely
for simple single-record threshold rules.

Every "field", "groupBy", and "timeField" value MUST be one of the exact
field names present in the schema you were given. NEVER invent a field name
to represent a concept the schema doesn't have a column for — if you can't
express an idea with an existing field, don't include that rule.

Use "conditionGroup" for a threshold check on a single record (e.g. "amount
> 10000"). Use "aggregateConditions" for velocity/frequency/grouping
patterns — "5 or more transactions within 60 seconds", "the same amount
repeated 3+ times by the same client in 24h", structuring/smurfing
detection, etc. For "function": "count", "field" is ignored by the engine
but the JSON key is still required — reuse the same field you put in
"groupBy". "timeWindow" only accepts these units: "s" (seconds), "min"
(minutes), "h" (hours), "d" (days) — e.g. "60s", "5min", "24h", "7d". Never
use "m" for minutes or months; it is not a valid unit for this field.

Available operators: eq, neq, gt, lt, gte, lte, contains, regex, in, not_in, between, is_null, is_not_null, starts_with, ends_with

Generate 3-5 meaningful rules based on the data structure and context.`

// ---------------------------------------------------------------------------
// AIRulesService — reads provider config from DB on each call
// ---------------------------------------------------------------------------

type AIRulesService struct {
	configRepo *repository.SystemConfigRepository
}

func NewAIRulesService(configRepo *repository.SystemConfigRepository) *AIRulesService {
	return &AIRulesService{configRepo: configRepo}
}

func (s *AIRulesService) GenerateRules(ctx context.Context, schema []models.SchemaField, dataSample string, userPrompt string) ([]AIRuleSuggestion, error) {
	// Read current AI config from DB (reflects admin changes in real time)
	cfg, err := s.configRepo.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading AI config: %w", err)
	}

	if cfg.AI.APIKey == "" {
		return nil, fmt.Errorf("AI API key not configured — go to Settings to add one")
	}

	userMessage := buildUserMessage(schema, dataSample, userPrompt)

	// El deadline cubre a ambos proveedores: los dos reciben este ctx.
	ctx, cancel := context.WithTimeout(ctx, aiRequestTimeout)
	defer cancel()

	var responseText string
	switch cfg.AI.Provider {
	case models.AIProviderDeepSeek:
		responseText, err = callDeepSeek(ctx, cfg.AI, userMessage)
	default: // anthropic
		responseText, err = callAnthropic(ctx, cfg.AI, userMessage)
	}
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("the AI provider did not respond within %s — try again, or check the provider settings", aiRequestTimeout)
		}
		return nil, err
	}

	suggestions, err := parseRuleSuggestions(responseText)
	if err != nil {
		return nil, err
	}

	return filterValidSuggestions(suggestions, schema), nil
}

// ---------------------------------------------------------------------------
// Provider implementations
// ---------------------------------------------------------------------------

func callAnthropic(ctx context.Context, ai models.AIConfig, userMessage string) (string, error) {
	client := anthropic.NewClient(
		option.WithAPIKey(ai.APIKey),
	)

	model := ai.Model
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}

	message, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: 2048,
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt, Type: "text"},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(
				anthropic.NewTextBlock(userMessage),
			),
		},
	})
	if err != nil {
		return "", fmt.Errorf("calling Anthropic API: %w", err)
	}

	for _, block := range message.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}
	return "", fmt.Errorf("empty response from Anthropic")
}

// deepSeekRequest / deepSeekResponse follow the OpenAI-compatible format.
type deepSeekRequest struct {
	Model     string            `json:"model"`
	Messages  []deepSeekMessage `json:"messages"`
	MaxTokens int               `json:"max_tokens"`
}

type deepSeekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type deepSeekResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func callDeepSeek(ctx context.Context, ai models.AIConfig, userMessage string) (string, error) {
	baseURL := ai.BaseURL
	if baseURL == "" {
		baseURL = models.DefaultAIBaseURLs[models.AIProviderDeepSeek]
	}

	model := ai.Model
	if model == "" {
		model = "deepseek-chat"
	}

	reqBody := deepSeekRequest{
		Model:     model,
		MaxTokens: 2048,
		Messages: []deepSeekMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling DeepSeek request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating DeepSeek request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ai.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling DeepSeek API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading DeepSeek response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("DeepSeek API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var dsResp deepSeekResponse
	if err := json.Unmarshal(respBody, &dsResp); err != nil {
		return "", fmt.Errorf("parsing DeepSeek response: %w", err)
	}

	if dsResp.Error != nil {
		return "", fmt.Errorf("DeepSeek API error: %s", dsResp.Error.Message)
	}

	if len(dsResp.Choices) == 0 {
		return "", fmt.Errorf("empty response from DeepSeek")
	}

	return dsResp.Choices[0].Message.Content, nil
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

func buildUserMessage(schema []models.SchemaField, dataSample, userPrompt string) string {
	schemaJSON, _ := json.Marshal(schema)
	msg := fmt.Sprintf("Schema:\n%s\n", string(schemaJSON))
	if dataSample != "" {
		msg += fmt.Sprintf("\nSample data:\n%s\n", dataSample)
	}
	if userPrompt != "" {
		msg += fmt.Sprintf("\nUser context: %s", userPrompt)
	}
	return msg
}

func parseRuleSuggestions(responseText string) ([]AIRuleSuggestion, error) {
	var suggestions []AIRuleSuggestion
	if err := json.Unmarshal([]byte(responseText), &suggestions); err != nil {
		// Try extracting JSON from markdown code block
		cleaned := extractJSON(responseText)
		if err2 := json.Unmarshal([]byte(cleaned), &suggestions); err2 != nil {
			return nil, fmt.Errorf("parsing AI response: %w", err2)
		}
	}
	return suggestions, nil
}

// validAITimeWindow matches only the units taught to the AI (s, min, h, d).
// "m" (months) is deliberately excluded — it's never in the AI's vocabulary,
// so a suggestion using it is treated as a hallucinated/ambiguous unit
// rather than risk it meaning "minutes" to whoever generated it.
var validAITimeWindow = regexp.MustCompile(`^\d+(s|min|h|d)$`)

// filterValidSuggestions drops any suggestion that references a field name
// not present in the monitor's schema — the AI has no other vocabulary to
// express concepts the schema doesn't have a column for, and in the past
// invented fake field names (e.g. "group_count_gte_5_seconds_lte_60") to
// work around that instead. A suggestion with any invalid reference is
// dropped whole, not partially repaired — a half-fixed rule is worse than
// no suggestion.
func filterValidSuggestions(suggestions []AIRuleSuggestion, schema []models.SchemaField) []AIRuleSuggestion {
	valid := make([]AIRuleSuggestion, 0, len(suggestions))
	for _, s := range suggestions {
		if reason := discardReason(s, schema); reason != "" {
			log.Printf("ai-rules: descartando sugerencia %q — %s", s.Name, reason)
			continue
		}
		valid = append(valid, s)
	}
	return valid
}

// discardReason devuelve por qué una sugerencia no se puede guardar, o ""
// si es guardable. Solo se valida lo que está PRESENTE: un groupBy vacío es
// un agregado global (rule_engine.go:657-661, _id:null) y una ventana vacía
// es un agregado sobre todo el histórico (rule_engine.go:624). Tratar el
// vacío como campo inexistente descartaba sugerencias perfectamente válidas
// y dejaba al usuario mirando una lista vacía.
func discardReason(s AIRuleSuggestion, schema []models.SchemaField) string {
	fieldNames := make(map[string]bool, len(schema))
	for _, f := range schema {
		fieldNames[f.Name] = true
	}

	for _, cond := range s.ConditionGroup.Conditions {
		if !fieldNames[cond.Field] {
			return fmt.Sprintf("la condición usa el campo %q, que no está en el esquema", cond.Field)
		}
	}

	for _, agg := range s.AggregateConditions {
		// Para "count" el motor ignora Field (buildAggExpr → $sum:1), así
		// que puede venir vacío; para el resto es lo que se agrega.
		if agg.Function != models.AggFuncCount && agg.Field == "" {
			return "el agregado no dice qué campo agregar"
		}
		if agg.Field != "" && !fieldNames[agg.Field] {
			return fmt.Sprintf("el agregado usa el campo %q, que no está en el esquema", agg.Field)
		}
		if agg.GroupBy != "" && !fieldNames[agg.GroupBy] {
			return fmt.Sprintf("el agregado agrupa por %q, que no está en el esquema", agg.GroupBy)
		}
		if agg.TimeField != "" && !fieldNames[agg.TimeField] {
			return fmt.Sprintf("el agregado mide el tiempo sobre %q, que no está en el esquema", agg.TimeField)
		}
		// Media ventana es intención a medio expresar: sin el par completo
		// el motor ignora la ventana y "5 en 60s" se vuelve "5 alguna vez".
		if (agg.TimeField == "") != (agg.TimeWindow == "") {
			return "la ventana de tiempo está incompleta: hacen falta el campo de fecha y la duración"
		}
		if agg.TimeWindow != "" && !validAITimeWindow.MatchString(agg.TimeWindow) {
			return fmt.Sprintf("la ventana %q no usa una unidad válida (s, min, h, d)", agg.TimeWindow)
		}
	}

	return ""
}

func extractJSON(text string) string {
	start := -1
	end := -1
	for i, c := range text {
		if c == '[' && start == -1 {
			start = i
		}
		if c == ']' {
			end = i + 1
		}
	}
	if start >= 0 && end > start {
		return text[start:end]
	}
	return text
}
