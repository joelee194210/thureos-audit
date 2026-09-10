package services

import (
	"strings"
	"testing"
	"time"

	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestMonitorAlias_NormalizaNombre(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"Transacciones Bancolombia", "transacciones_bancolombia"},
		{"Alertas SWIFT", "alertas_swift"},
		{"Operaciones Región Caribe", "operaciones_region_caribe"},
		{"  espacios   raros  ", "espacios_raros"},
		{"Guiones-y.puntos", "guiones_y_puntos"},
		{"ÁÉÍÓÚñÑ", "aeiounn"},
	}
	for _, tc := range cases {
		if got := monitorAlias(tc.name); got != tc.want {
			t.Errorf("monitorAlias(%q) = %q, quiero %q", tc.name, got, tc.want)
		}
	}
}

func TestMonitorAlias_RecortaA40(t *testing.T) {
	long := "monitor de transacciones internacionales del area metropolitana"
	got := monitorAlias(long)
	if len(got) > 40 {
		t.Fatalf("alias de %d caracteres, tope 40: %q", len(got), got)
	}
	if got[len(got)-1] == '_' {
		t.Errorf("el alias no debe terminar en guion bajo: %q", got)
	}
}

func TestBuildMonitorAliases_ResuelveColisiones(t *testing.T) {
	monitors := []models.Monitor{
		{ID: primitive.NewObjectID(), Name: "Transacciones"},
		{ID: primitive.NewObjectID(), Name: "transacciones"},
		{ID: primitive.NewObjectID(), Name: "TRANSACCIONES!!"},
	}
	aliases := buildMonitorAliases(monitors)

	if len(aliases) != 3 {
		t.Fatalf("quiero 3 alias distintos, tengo %d: %v", len(aliases), keysOf(aliases))
	}
	for _, want := range []string{"transacciones", "transacciones_2", "transacciones_3"} {
		if _, ok := aliases[want]; !ok {
			t.Errorf("falta el alias %q; tengo %v", want, keysOf(aliases))
		}
	}
	// Cada alias apunta al monitor correcto, en orden de entrada.
	if aliases["transacciones"].ID != monitors[0].ID {
		t.Error("transacciones debe apuntar al primer monitor")
	}
	if aliases["transacciones_2"].ID != monitors[1].ID {
		t.Error("transacciones_2 debe apuntar al segundo monitor")
	}
}

func TestBuildMonitorAliases_NombreSinCaracteresUtiles(t *testing.T) {
	monitors := []models.Monitor{
		{ID: primitive.NewObjectID(), Name: "***"},
		{ID: primitive.NewObjectID(), Name: "###"},
	}
	aliases := buildMonitorAliases(monitors)
	for _, want := range []string{"monitor_1", "monitor_2"} {
		if _, ok := aliases[want]; !ok {
			t.Errorf("falta el alias de respaldo %q; tengo %v", want, keysOf(aliases))
		}
	}
}

func TestBuildMonitorAliases_VacioDevuelveMapaVacio(t *testing.T) {
	aliases := buildMonitorAliases(nil)
	if len(aliases) != 0 {
		t.Fatalf("quiero mapa vacío, tengo %v", keysOf(aliases))
	}
}

func keysOf(m map[string]*models.Monitor) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestBuildSystemPrompt_UnBloquePorMonitor(t *testing.T) {
	monitors := []models.Monitor{
		{ID: primitive.NewObjectID(), Name: "Transacciones Bancolombia", Schema: []models.SchemaField{
			{Name: "monto", Type: models.FieldNumber},
			{Name: "fecha", Type: models.FieldDate},
		}},
		{ID: primitive.NewObjectID(), Name: "Alertas SWIFT", Schema: []models.SchemaField{
			{Name: "referencia", Type: models.FieldString},
		}},
	}
	prompt := buildSystemPrompt(buildMonitorAliases(monitors))

	for _, want := range []string{
		"transacciones_bancolombia", "Transacciones Bancolombia", "monto", "fecha",
		"alertas_swift", "Alertas SWIFT", "referencia",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("el prompt debería contener %q\n---\n%s", want, prompt)
		}
	}
}

// El prompt debe ser estable entre llamadas: un mapa de Go se recorre en
// orden aleatorio, así que hay que ordenar explícitamente o el prompt
// cambia en cada turno y rompe el caché del proveedor.
func TestBuildSystemPrompt_OrdenEstable(t *testing.T) {
	monitors := []models.Monitor{
		{ID: primitive.NewObjectID(), Name: "Alfa"},
		{ID: primitive.NewObjectID(), Name: "Beta"},
		{ID: primitive.NewObjectID(), Name: "Gamma"},
		{ID: primitive.NewObjectID(), Name: "Delta"},
	}
	aliases := buildMonitorAliases(monitors)
	first := buildSystemPrompt(aliases)
	for i := 0; i < 20; i++ {
		if got := buildSystemPrompt(aliases); got != first {
			t.Fatal("buildSystemPrompt debe devolver el mismo texto para el mismo mapa")
		}
	}
}

func TestResolveQueryMonitor_AliasValido(t *testing.T) {
	monitors := []models.Monitor{{ID: primitive.NewObjectID(), Name: "Transacciones"}}
	aliases := buildMonitorAliases(monitors)

	got, err := resolveQueryMonitor(aliases, "transacciones")
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if got.ID != monitors[0].ID {
		t.Errorf("resolvió al monitor equivocado")
	}
}

// AUTORIZACIÓN: un alias que no está en la allowlist de la conversación
// nunca resuelve. Ver Global Constraints.
func TestResolveQueryMonitor_AliasFueraDeLaAllowlist(t *testing.T) {
	aliases := buildMonitorAliases([]models.Monitor{
		{ID: primitive.NewObjectID(), Name: "Transacciones"},
	})

	if _, err := resolveQueryMonitor(aliases, "clientes_secretos"); err == nil {
		t.Fatal("un alias fuera de la allowlist debe fallar, no resolver")
	}
}

// AUTORIZACIÓN: un ObjectID crudo es un alias desconocido como cualquier
// otro. Nunca se acepta como forma de nombrar un monitor.
func TestResolveQueryMonitor_ObjectIDCrudoNoEsUnAlias(t *testing.T) {
	id := primitive.NewObjectID()
	aliases := buildMonitorAliases([]models.Monitor{{ID: id, Name: "Transacciones"}})

	if _, err := resolveQueryMonitor(aliases, id.Hex()); err == nil {
		t.Fatal("un ObjectID crudo no debe resolver, ni siquiera el del propio monitor")
	}
}

// El error debe listar los alias válidos para que el LLM pueda corregirse
// en la iteración siguiente, en vez de reintentar a ciegas.
func TestResolveQueryMonitor_ElErrorListaLosAliasValidos(t *testing.T) {
	aliases := buildMonitorAliases([]models.Monitor{
		{ID: primitive.NewObjectID(), Name: "Transacciones"},
		{ID: primitive.NewObjectID(), Name: "Alertas SWIFT"},
	})

	_, err := resolveQueryMonitor(aliases, "inexistente")
	if err == nil {
		t.Fatal("quiero error")
	}
	for _, want := range []string{"transacciones", "alertas_swift"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("el error debería listar %q; dice: %v", want, err)
		}
	}
}

func TestChatToolIterations_EscalaConLosMonitoresYTopea(t *testing.T) {
	cases := []struct{ monitors, want int }{
		{1, 8}, {2, 11}, {3, 14}, {5, 20}, {6, 20}, {50, 20},
	}
	for _, tc := range cases {
		if got := chatToolIterations(tc.monitors); got != tc.want {
			t.Errorf("chatToolIterations(%d) = %d, quiero %d", tc.monitors, got, tc.want)
		}
	}
}

func TestChatAskTimeout_EscalaConLosMonitoresYTopea(t *testing.T) {
	cases := []struct {
		monitors int
		want     time.Duration
	}{
		{1, 90 * time.Second}, {2, 120 * time.Second}, {6, 240 * time.Second}, {50, 240 * time.Second},
	}
	for _, tc := range cases {
		if got := chatAskTimeout(tc.monitors); got != tc.want {
			t.Errorf("chatAskTimeout(%d) = %v, quiero %v", tc.monitors, got, tc.want)
		}
	}
}
