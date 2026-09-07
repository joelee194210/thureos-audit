package services

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
)

// NotificationService envía notificaciones de red flags nuevas: email por
// SMTP o por el API de Resend (elección en la config) y webhooks HTTP
// firmados. El envío real corre en el worker vía la cola; TriggerRedFlag
// solo encola y nunca bloquea la creación de la alerta.
type NotificationService struct {
	queue      *JobQueue
	configRepo *repository.SystemConfigRepository
	httpClient *http.Client // inyectable en tests
	resendURL  string       // inyectable en tests; vacío = API real
}

func NewNotificationService(queue *JobQueue, configRepo *repository.SystemConfigRepository) *NotificationService {
	return &NotificationService{
		queue:      queue,
		configRepo: configRepo,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// SetQueue conecta la cola cuando Redis quedó disponible después de
// construir el servicio (main lo llama dentro del bloque de Redis).
func (s *NotificationService) SetQueue(q *JobQueue) {
	s.queue = q
}

const resendAPIURL = "https://api.resend.com/emails"

// NotifyPayload es la instantánea mínima del red flag para renderizar la
// notificación sin releer Mongo en el worker.
type NotifyPayload struct {
	Type        string `json:"type"`
	RedFlagID   string `json:"redFlagId"`
	MonitorName string `json:"monitorName"`
	RuleName    string `json:"ruleName"`
	Severity    string `json:"severity"`
	Message     string `json:"message"`
	MatchCount  int    `json:"matchCount"`
}

const (
	NotifyTypeRedFlagCreated = "red_flag.created"
	NotifyTypeSLABreach      = "sla_breach"
)

// enqueueNotification centraliza el chequeo de canales habilitados y el
// encolado — TriggerRedFlag, TriggerSLABreach y TriggerTest son el mismo
// camino con distinto payload. "Sin canales habilitados" también es
// error: para TriggerSLABreach eso le dice al caller que nada se envió
// de verdad, así que no debe marcar el caso como notificado — mejor
// reintentar en el próximo tick que perder el aviso en silencio si más
// tarde se configura un canal.
func (s *NotificationService) enqueueNotification(payload *NotifyPayload) error {
	if s == nil || s.queue == nil || s.configRepo == nil {
		return fmt.Errorf("la cola de notificaciones no está disponible")
	}
	cfg, err := s.configRepo.Get(context.Background())
	if err != nil {
		return fmt.Errorf("leyendo config de notificaciones: %w", err)
	}
	if !cfg.Notifications.EmailProviderEnabled() && !cfg.Notifications.WebhooksEnabled() {
		return fmt.Errorf("no hay ningún canal de notificación habilitado")
	}
	job := EvalJob{Kind: JobKindNotify, Notify: payload}
	if err := s.queue.Enqueue(context.Background(), job); err != nil {
		return fmt.Errorf("encolando notificación: %w", err)
	}
	return nil
}

// TriggerRedFlag encola la notificación de una red flag nueva. Fallo se
// loguea y no sube: la alerta ya está confirmada en Mongo, y no hay nadie
// esperando una respuesta síncrona (a diferencia de TriggerSLABreach, que
// el cron de escalamiento necesita para decidir si marca el caso como
// notificado).
func (s *NotificationService) TriggerRedFlag(rf models.RedFlag) {
	if err := s.enqueueNotification(&NotifyPayload{
		Type:        NotifyTypeRedFlagCreated,
		RedFlagID:   rf.ID.Hex(),
		MonitorName: rf.MonitorName,
		RuleName:    rf.RuleName,
		Severity:    string(rf.Severity),
		Message:     rf.Message,
		MatchCount:  rf.MatchCount,
	}); err != nil {
		log.Printf("WARNING: notifications: aviso de red flag %s: %v", rf.ID.Hex(), err)
	}
}

// TriggerSLABreach encola el aviso de que el plazo de SLA de un caso
// venció. No cambia status ni fuerza escalamiento — solo avisa; mover el
// caso a "escalated" sigue siendo decisión del analista. Devuelve error
// (a diferencia de TriggerRedFlag) porque el cron de escalamiento solo
// debe marcar el caso como notificado si el aviso salió de verdad.
func (s *NotificationService) TriggerSLABreach(rf models.RedFlag) error {
	return s.enqueueNotification(&NotifyPayload{
		Type:        NotifyTypeSLABreach,
		RedFlagID:   rf.ID.Hex(),
		MonitorName: rf.MonitorName,
		RuleName:    rf.RuleName,
		Severity:    string(rf.Severity),
		Message:     fmt.Sprintf("SLA vencido: %s", rf.Message),
		MatchCount:  rf.MatchCount,
	})
}

// testNotifyPayload es la notificación sintética que dispara el botón
// "Probar" en Configuración — mismos campos que una red flag real para
// que el email/webhook de prueba se vea igual al de producción.
func testNotifyPayload() *NotifyPayload {
	return &NotifyPayload{
		Type:        NotifyTypeRedFlagCreated,
		RedFlagID:   "test",
		MonitorName: "Monitor de prueba",
		RuleName:    "Regla de prueba",
		Severity:    "medium",
		Message:     "Esta es una notificación de prueba enviada desde Configuración.",
		MatchCount:  1,
	}
}

// TriggerTest encola una notificación sintética para validar la
// configuración de canales sin esperar una red flag real. Mismo camino
// que TriggerRedFlag (cola → worker → ProcessNotification), a diferencia
// de esa función SÍ reporta el error al caller: el admin está mirando la
// pantalla y necesita saber si falló.
func (s *NotificationService) TriggerTest(ctx context.Context) error {
	return s.enqueueNotification(testNotifyPayload())
}

// ProcessNotification entrega la notificación por todos los canales
// habilitados. Devuelve error si el canal email está configurado y falla,
// o si falla algún webhook — el worker reintenta por eso.
func (s *NotificationService) ProcessNotification(ctx context.Context, payload *NotifyPayload) error {
	cfg, err := s.configRepo.Get(ctx)
	if err != nil {
		return fmt.Errorf("leyendo config de notificaciones: %w", err)
	}

	var errs []string
	if cfg.Notifications.EmailProviderEnabled() {
		subject, body := renderRedFlagEmail(payload)
		if err := s.sendEmail(ctx, &cfg.Notifications, subject, body); err != nil {
			errs = append(errs, fmt.Sprintf("email (%s): %v", cfg.Notifications.EmailProvider, err))
		}
	}

	body, err := json.Marshal(map[string]interface{}{
		"type":       payload.Type,
		"redFlagId":  payload.RedFlagID,
		"monitor":    payload.MonitorName,
		"rule":       payload.RuleName,
		"severity":   payload.Severity,
		"message":    payload.Message,
		"matchCount": payload.MatchCount,
		"sentAt":     time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("serializando payload de webhook: %w", err)
	}
	for _, wh := range cfg.Notifications.Webhooks {
		if !wh.Enabled || wh.URL == "" {
			continue
		}
		if err := s.deliverWebhook(ctx, wh, body); err != nil {
			errs = append(errs, fmt.Sprintf("webhook %s: %v", wh.URL, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("fallos de entrega: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (s *NotificationService) sendEmail(ctx context.Context, cfg *models.NotificationConfig, subject, body string) error {
	recipients := cfg.ToEmails
	switch cfg.EmailProvider {
	case models.EmailProviderSMTP:
		return sendViaSMTP(cfg.SMTP, recipients, subject, body)
	case models.EmailProviderResend:
		return s.sendViaResend(ctx, cfg.Resend, recipients, subject, body)
	default:
		return fmt.Errorf("proveedor de email no soportado: %s", cfg.EmailProvider)
	}
}

// sendViaSMTP usa net/smtp con AUTH PLAIN solo si hay credenciales.
func sendViaSMTP(cfg models.SMTPConfig, to []string, subject, body string) error {
	if cfg.Port <= 0 {
		cfg.Port = 587
	}
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	msg := bytes.Buffer{}
	fmt.Fprintf(&msg, "From: %s\r\n", cfg.From)
	fmt.Fprintf(&msg, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&msg, "Subject: %s\r\n", subject)
	msg.WriteString("MIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n")
	msg.WriteString(body)

	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}
	return smtp.SendMail(addr, auth, cfg.From, to, msg.Bytes())
}

// sendViaResend llama al API REST de Resend; net/http stdlib basta, no
// conviene sumar un SDK para un POST.
func (s *NotificationService) sendViaResend(ctx context.Context, cfg models.ResendConfig, to []string, subject, body string) error {
	url := s.resendURL
	if url == "" {
		url = resendAPIURL
	}
	payload, err := json.Marshal(map[string]interface{}{
		"from":    cfg.From,
		"to":      to,
		"subject": subject,
		"text":    body,
	})
	if err != nil {
		return fmt.Errorf("serializando payload de Resend: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("llamando a Resend: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("resend respondió %d", resp.StatusCode)
	}
	return nil
}

func (s *NotificationService) deliverWebhook(ctx context.Context, wh models.WebhookConfig, payload []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, wh.URL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if wh.Secret != "" {
		req.Header.Set("X-Thureos-Signature", SignPayload(wh.Secret, payload))
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("POST: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("respondió %d", resp.StatusCode)
	}
	return nil
}

// SignPayload calcula HMAC-SHA256 hexadecimal del cuerpo con el secreto
// del webhook — el receptor valida el origen con el mismo secreto.
func SignPayload(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func renderRedFlagEmail(p *NotifyPayload) (string, string) {
	subject := fmt.Sprintf("[Thureos] Bandera roja %s — %s", strings.ToUpper(p.Severity), p.RuleName)
	body := strings.Join([]string{
		"Nueva bandera roja detectada.",
		"",
		fmt.Sprintf("Monitor:   %s", p.MonitorName),
		fmt.Sprintf("Regla:     %s", p.RuleName),
		fmt.Sprintf("Severidad: %s", p.Severity),
		fmt.Sprintf("Coincidencias: %d", p.MatchCount),
		fmt.Sprintf("Detalle:   %s", p.Message),
		"",
		fmt.Sprintf("ID: %s", p.RedFlagID),
	}, "\n")
	return subject, body
}
