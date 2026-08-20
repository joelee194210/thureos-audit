package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AIProvider represents the AI provider used for rule generation.
type AIProvider string

const (
	AIProviderAnthropic AIProvider = "anthropic"
	AIProviderDeepSeek  AIProvider = "deepseek"
)

// AIProviderModels maps each provider to its available models.
var AIProviderModels = map[AIProvider][]string{
	AIProviderAnthropic: {"claude-sonnet-4-20250514", "claude-3-5-sonnet-latest"},
	AIProviderDeepSeek:  {"deepseek-chat", "deepseek-reasoner"},
}

// DefaultAIBaseURLs defines the default API base URL per provider.
var DefaultAIBaseURLs = map[AIProvider]string{
	AIProviderDeepSeek: "https://api.deepseek.com",
}

// AIConfig holds the configurable AI provider settings.
type AIConfig struct {
	Provider AIProvider `bson:"provider" json:"provider"`
	Model    string     `bson:"model" json:"model"`
	APIKey   string     `bson:"api_key" json:"-"` // never exposed via JSON
	BaseURL  string     `bson:"base_url,omitempty" json:"baseUrl,omitempty"`
}

// SystemConfig is a singleton document storing system-wide settings.
type SystemConfig struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	AI        AIConfig           `bson:"ai" json:"ai"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updatedAt"`
	UpdatedBy string             `bson:"updated_by" json:"updatedBy"`
}

// MaskAPIKey returns a masked version of the API key for display.
func MaskAPIKey(key string) string {
	if len(key) <= 8 {
		return "••••••••"
	}
	return key[:4] + "••••" + key[len(key)-4:]
}
