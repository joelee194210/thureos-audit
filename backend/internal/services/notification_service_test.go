package services

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thureos/compliance/internal/models"
)

func TestSignPayload_EsHMACSHA256Hexadecimal(t *testing.T) {
	sig := SignPayload("secreto", []byte(`{"a":1}`))
	if len(sig) != 64 {
		t.Errorf("firma = %q, want 64 hex chars (SHA-256)", sig)
	}
	// determinística y distinta por secreto/payload
	if sig != SignPayload("secreto", []byte(`{"a":1}`)) {
		t.Error("HMAC no determinístico")
	}
	if sig == SignPayload("otro", []byte(`{"a":1}`)) {
		t.Error("secretos distintos produjeron la misma firma")
	}
}

func TestRenderRedFlagEmail_TraeLosDatosClave(t *testing.T) {
	subject, body := renderRedFlagEmail(&NotifyPayload{
		MonitorName: "Tarjetas Q3",
		RuleName:    "Velocidad",
		Severity:    "high",
		Message:     "3 coincidencias",
		MatchCount:  3,
	})
	if !strings.Contains(subject, "HIGH") || !strings.Contains(subject, "Velocidad") {
		t.Errorf("subject = %q", subject)
	}
	for _, want := range []string{"Tarjetas Q3", "Severidad: high", "3", "3 coincidencias"} {
		if !strings.Contains(body, want) {
			t.Errorf("body no contiene %q:\n%s", want, body)
		}
	}
}

// El webhook sale firmado con el secreto del canal y el receptor ve el
// payload JSON con los datos del red flag.
func TestDeliverWebhook_FirmaYPayload(t *testing.T) {
	var gotSig string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-Thureos-Signature")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	s := &NotificationService{httpClient: srv.Client()}
	err := s.deliverWebhook(context.Background(),
		models.WebhookConfig{URL: srv.URL, Secret: "s3cr3t", Enabled: true},
		[]byte(`{"redFlagId":"abc"}`))
	if err != nil {
		t.Fatalf("deliverWebhook: %v", err)
	}
	if gotSig != SignPayload("s3cr3t", []byte(`{"redFlagId":"abc"}`)) {
		t.Errorf("firma del header no coincide con HMAC del body")
	}
	if !strings.Contains(string(gotBody), "abc") {
		t.Errorf("body = %s", gotBody)
	}
}

func TestDeliverWebhook_ErrorHTTPSube(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	s := &NotificationService{httpClient: srv.Client()}
	if err := s.deliverWebhook(context.Background(),
		models.WebhookConfig{URL: srv.URL, Enabled: true}, []byte(`{}`)); err == nil {
		t.Error("un 500 del receptor debe ser error para que el worker reintente")
	}
}

// Resend: el request lleva Bearer con la API key y el cuerpo con from/to/
// subject/text; un 2xx se considera entregado.
func TestSendViaResend_RequestYAuth(t *testing.T) {
	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	s := &NotificationService{httpClient: srv.Client(), resendURL: srv.URL}
	err := s.sendViaResend(context.Background(),
		models.ResendConfig{APIKey: "re_123", From: "alertas@thureos.io"},
		[]string{"oficial@banco.com"}, "Asunto", "Cuerpo")
	if err != nil {
		t.Fatalf("sendViaResend: %v", err)
	}
	if gotAuth != "Bearer re_123" {
		t.Errorf("Authorization = %q, want Bearer re_123", gotAuth)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(gotBody), &payload); err != nil {
		t.Fatalf("payload no es JSON: %v", err)
	}
	if payload["from"] != "alertas@thureos.io" {
		t.Errorf("from = %v", payload["from"])
	}
}

func TestTestNotifyPayload_TraeDatosDeMuestraCompletos(t *testing.T) {
	p := testNotifyPayload()
	if p.RedFlagID == "" || p.MonitorName == "" || p.RuleName == "" || p.Message == "" {
		t.Fatalf("payload de prueba con campos vacíos: %+v", p)
	}
	if p.MatchCount < 1 {
		t.Errorf("MatchCount = %d, want >= 1", p.MatchCount)
	}
}

func TestTriggerTest_SinCanalesHabilitadosNoEncola(t *testing.T) {
	// Sin queue ni configRepo (nil): TriggerTest debe fallar por servicio
	// no disponible en vez de paniquear.
	s := &NotificationService{}
	if err := s.TriggerTest(context.Background()); err == nil {
		t.Fatal("esperaba error con servicio no inicializado")
	}
}

func TestNotificationConfig_HabilitacionPorCanal(t *testing.T) {
	n := models.NotificationConfig{}
	if n.EmailProviderEnabled() || n.WebhooksEnabled() {
		t.Fatal("config vacía no debe estar habilitada")
	}
	n.EmailProvider = models.EmailProviderResend
	n.Resend = models.ResendConfig{APIKey: "re_x", From: "a@b.c"}
	n.ToEmails = []string{"d@e.f"}
	if !n.EmailProviderEnabled() {
		t.Error("resend con api key + from + destinatarios debe estar habilitado")
	}
	// smtp elegido pero sin host: no habilita
	n2 := models.NotificationConfig{EmailProvider: models.EmailProviderSMTP}
	if n2.EmailProviderEnabled() {
		t.Error("smtp sin host no debe habilitar el canal email")
	}
	n.Webhooks = []models.WebhookConfig{{URL: "http://x", Enabled: true}, {URL: "http://y"}}
	if !n.WebhooksEnabled() {
		t.Error("un webhook enabled con URL debe habilitar el canal")
	}
}
