package handlers

import (
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// El defecto que este test fija: CreateConversation recibía monitorIds sin
// deduplicar, así que mandar el mismo monitor siete veces (a) pasaba
// intacto a buildMonitorAliases, que le asignaba un alias distinto a cada
// copia del mismo monitor, y (b) agotaba el tope de
// MaxMonitorsPerConversation contando entradas repetidas en vez de
// monitores distintos — una conversación así nunca podía sumar un segundo
// monitor real vía AddMonitor. dedupeMonitorIDs es la corrección: siete
// copias del mismo id deben colapsar en un monitor, no rechazarse por tope.
func TestDedupeMonitorIDs_SieteRepetidosColapsanEnUno(t *testing.T) {
	id := primitive.NewObjectID()
	raw := make([]string, 7)
	for i := range raw {
		raw[i] = id.Hex()
	}

	got, err := dedupeMonitorIDs(raw)
	if err != nil {
		t.Fatalf("dedupeMonitorIDs no debería fallar con hex válidos repetidos: %v", err)
	}
	want := []primitive.ObjectID{id}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("dedupeMonitorIDs(7x el mismo id) = %v, se esperaba %v (un solo monitor)", got, want)
	}
}

// El orden de primera aparición debe conservarse: buildMonitorAliases lo
// usa para decidir qué monitor se queda con el alias base cuando dos
// nombres colisionan (ver chat_monitors.go). Un dedupe que reordene
// haría inestables esos alias.
func TestDedupeMonitorIDs_PreservaOrdenDePrimeraAparicion(t *testing.T) {
	a := primitive.NewObjectID()
	b := primitive.NewObjectID()
	c := primitive.NewObjectID()
	raw := []string{b.Hex(), a.Hex(), b.Hex(), c.Hex(), a.Hex()}

	got, err := dedupeMonitorIDs(raw)
	if err != nil {
		t.Fatalf("dedupeMonitorIDs no debería fallar: %v", err)
	}
	want := []primitive.ObjectID{b, a, c}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("dedupeMonitorIDs(%v) = %v, se esperaba %v en orden de primera aparición", raw, got, want)
	}
}

func TestDedupeMonitorIDs_SinRepetidosNoCambiaNada(t *testing.T) {
	a := primitive.NewObjectID()
	b := primitive.NewObjectID()
	raw := []string{a.Hex(), b.Hex()}

	got, err := dedupeMonitorIDs(raw)
	if err != nil {
		t.Fatalf("dedupeMonitorIDs no debería fallar: %v", err)
	}
	want := []primitive.ObjectID{a, b}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("dedupeMonitorIDs(%v) = %v, se esperaba %v", raw, got, want)
	}
}

func TestDedupeMonitorIDs_HexInvalidoPropagaError(t *testing.T) {
	if _, err := dedupeMonitorIDs([]string{"no-es-un-object-id"}); err == nil {
		t.Error("dedupeMonitorIDs debería fallar con un hex inválido")
	}
}
