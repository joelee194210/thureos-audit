package services

import (
	"testing"

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
