package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
)

// AIRuleSuggestion represents a single AI-generated rule suggestion.
type AIRuleSuggestion struct {
	Name           string               `json:"name"`
	Description    string               `json:"description"`
	ConditionGroup models.ConditionGroup `json:"conditionGroup"`
	Severity       models.Severity      `json:"severity"`
	Actions        []models.ActionType  `json:"actions"`
	Reasoning      string               `json:"reasoning"`
}

// systemPrompt is shared across all providers — the contract stays the same.
const systemPrompt = `You are a data monitoring rules expert. Given a data schema and optional sample data, generate monitoring rules that detect anomalies, suspicious patterns, or business-critical conditions.

Return ONLY a JSON array of rule suggestions. Each rule must follow this exact structure:
{
  "name": "Rule name",
  "description": "What this rule detects",
  "conditionGroup": {
    "logic": "AND" or "OR",
    "conditions": [
      {"field": "field_name", "operator": "op", "value": value}
    ]
  },
  "severity": "low|medium|high|critical",
  "actions": ["alert", "flag", "log"],
  "reasoning": "Why this rule is important"
}

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

	var responseText string
	switch cfg.AI.Provider {
	case models.AIProviderDeepSeek:
		responseText, err = callDeepSeek(ctx, cfg.AI, userMessage)
	default: // anthropic
		responseText, err = callAnthropic(ctx, cfg.AI, userMessage)
	}
	if err != nil {
		return nil, err
	}

	return parseRuleSuggestions(responseText)
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
	Model    string              `json:"model"`
	Messages []deepSeekMessage   `json:"messages"`
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
	defer resp.Body.Close()

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
