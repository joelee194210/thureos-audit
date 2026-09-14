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
//
// Los ids de DeepSeek se verificaron el 2026-09-14 contra GET /models de la
// API viva: el catálogo son deepseek-v4-pro y deepseek-flash. Los anteriores
// —deepseek-chat y deepseek-reasoner— ya NO existen, y una config guardada
// que todavía los nombre deja de funcionar. Este mapa es la única fuente de
// verdad: lo valida el router al guardar y lo consume el desplegable de
// Configuración, así que el frontend no repite la lista.
//
// El orden importa: el primero es el que el frontend autoselecciona al
// cambiar de proveedor. Va Pro primero a propósito. deepseek-flash
// (DeepSeek-V4.1-Flash) aceptaba la conexión y se quedaba colgado sin
// responder —cinco de cinco intentos— mientras Pro respondía cinco de
// cinco; es la cola saturada que el propio proveedor reporta como
// "Service is too busy". Cuando Flash se descongestione, elegirlo es un
// clic en Configuración, sin tocar código.
//
// Las entradas de Anthropic de abajo no se revisaron en esa pasada y
// probablemente arrastran el mismo desfase.
var AIProviderModels = map[AIProvider][]string{
	AIProviderAnthropic: {"claude-sonnet-4-20250514", "claude-3-5-sonnet-latest"},
	AIProviderDeepSeek:  {"deepseek-v4-pro", "deepseek-flash"},
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

// EmailProvider es el canal de envío de emails de alertas.
type EmailProvider string

const (
	EmailProviderSMTP   EmailProvider = "smtp"
	EmailProviderResend EmailProvider = "resend"
)

// SMTPConfig son los datos de conexión para envío directo por SMTP.
type SMTPConfig struct {
	Host     string `bson:"host" json:"host"`
	Port     int    `bson:"port" json:"port"`
	Username string `bson:"username" json:"username"`
	Password string `bson:"password" json:"-"`
	From     string `bson:"from" json:"from"`
}

// ResendConfig son los datos para el API de Resend (resend.com).
type ResendConfig struct {
	APIKey string `bson:"api_key" json:"-"`
	From   string `bson:"from" json:"from"`
}

// WebhookConfig es un destinatario HTTP de alertas. El payload se firma
// con HMAC-SHA256 del secreto en el header X-Thureos-Signature.
type WebhookConfig struct {
	URL     string `bson:"url" json:"url"`
	Secret  string `bson:"secret,omitempty" json:"-"`
	Enabled bool   `bson:"enabled" json:"enabled"`
}

// NotificationConfig configura cómo se notifican las red flags nuevas:
// email (SMTP o Resend, elegible) y webhooks HTTP.
type NotificationConfig struct {
	EmailProvider EmailProvider   `bson:"email_provider" json:"emailProvider"`
	SMTP          SMTPConfig      `bson:"smtp" json:"smtp"`
	Resend        ResendConfig    `bson:"resend" json:"resend"`
	ToEmails      []string        `bson:"to_emails" json:"toEmails"`
	Webhooks      []WebhookConfig `bson:"webhooks" json:"webhooks"`
}

// EmailProviderEnabled dice si el canal email tiene proveedor válido.
func (n NotificationConfig) EmailProviderEnabled() bool {
	switch n.EmailProvider {
	case EmailProviderSMTP:
		return n.SMTP.Host != "" && n.SMTP.From != "" && len(n.ToEmails) > 0
	case EmailProviderResend:
		return n.Resend.APIKey != "" && n.Resend.From != "" && len(n.ToEmails) > 0
	}
	return false
}

// WebhooksEnabled dice si hay al menos un webhook activo.
func (n NotificationConfig) WebhooksEnabled() bool {
	for _, w := range n.Webhooks {
		if w.Enabled && w.URL != "" {
			return true
		}
	}
	return false
}

// WatchmanConfig es la URL del servicio de screening de sanciones (Moov
// Watchman) — ya desplegado y operado fuera de esta app (VPS + cron de
// actualización de listas); acá solo se configura contra qué URL hablarle.
type WatchmanConfig struct {
	URL string `bson:"url" json:"url"`
}

// SystemConfig is a singleton document storing system-wide settings.
type SystemConfig struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	AI            AIConfig           `bson:"ai" json:"ai"`
	Notifications NotificationConfig `bson:"notifications" json:"notifications"`
	Watchman      WatchmanConfig     `bson:"watchman" json:"watchman"`
	UpdatedAt     time.Time          `bson:"updated_at" json:"updatedAt"`
	UpdatedBy     string             `bson:"updated_by" json:"updatedBy"`
}

// MaskAPIKey returns a masked version of the API key for display.
func MaskAPIKey(key string) string {
	if len(key) <= 8 {
		return "••••••••"
	}
	return key[:4] + "••••" + key[len(key)-4:]
}
