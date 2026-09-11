package repository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/thureos/compliance/internal/database"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Este test necesita un MongoDB de verdad y se saltea salvo que se lo
// pidan EXPLÍCITAMENTE por MONGO_TEST_URI. El gate es la variable, no la
// alcanzabilidad: un skip "si no hay Mongo" se volvería un no-op silencioso
// el día que el contenedor no esté y nadie se enteraría.
//
//	docker compose up -d
//	MONGO_TEST_URI=mongodb://localhost:27019 go test ./internal/repository/ -run Integracion -v
//
// Usa una base descartable propia, NUNCA thureos_compliance, y la borra al
// terminar.
const testDBName = "thureos_repo_test"

func conectarParaTest(t *testing.T) (*database.MongoDB, func()) {
	t.Helper()
	uri := os.Getenv("MONGO_TEST_URI")
	if uri == "" {
		t.Skip("MONGO_TEST_URI sin definir: test de integración salteado a propósito")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("conectando a %s: %v", uri, err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		t.Fatalf("ping a %s: %v", uri, err)
	}
	db := client.Database(testDBName)
	if err := db.Drop(ctx); err != nil {
		t.Fatalf("limpiando %s: %v", testDBName, err)
	}
	return &database.MongoDB{Client: client, Database: db}, func() {
		bg := context.Background()
		_ = db.Drop(bg)
		_ = client.Disconnect(bg)
	}
}

// El hallazgo, de punta a punta contra Mongo real: una conversación legacy
// (monitor_id escalar, sin monitor_ids) no debe perder su monitor original
// al agregarle uno nuevo. Es el único lugar donde se puede demostrar que
// {"monitor_ids": nil} matchea un campo AUSENTE — la semántica de la que
// depende toda la rama legacy del compare-and-set, y que ningún test de
// helper puro puede proteger.
func TestIntegracionAddMonitor_ConversacionLegacyConservaSuMonitorOriginal(t *testing.T) {
	db, cerrar := conectarParaTest(t)
	defer cerrar()
	ctx := context.Background()
	repo := NewChatRepository(db)

	convID := primitive.NewObjectID()
	original := primitive.NewObjectID()
	nuevo := primitive.NewObjectID()
	tercero := primitive.NewObjectID()

	// Documento legacy tal cual lo escribía la versión mono-monitor.
	if _, err := db.Collection("chat_conversations").InsertOne(ctx, bson.M{
		"_id":        convID,
		"user_id":    primitive.NewObjectID(),
		"title":      "conversación previa al multi-monitor",
		"monitor_id": original,
		"created_at": time.Now(),
		"updated_at": time.Now(),
	}); err != nil {
		t.Fatalf("sembrando el documento legacy: %v", err)
	}

	conv, err := repo.GetConversation(ctx, convID)
	if err != nil {
		t.Fatalf("GetConversation: %v", err)
	}
	if len(conv.MonitorIDs) != 1 || conv.MonitorIDs[0] != original {
		t.Fatalf("Normalize debe colapsar el escalar en MonitorIDs; tengo %v", conv.MonitorIDs)
	}

	next, err := repo.AddMonitor(ctx, convID, conv.MonitorIDs, nuevo)
	if err != nil {
		t.Fatalf("AddMonitor sobre un documento legacy: %v", err)
	}
	if len(next) != 2 || next[0] != original || next[1] != nuevo {
		t.Fatalf("quiero [original nuevo] en ese orden; tengo %v", next)
	}

	var crudo bson.M
	if err := db.Collection("chat_conversations").FindOne(ctx, bson.M{"_id": convID}).Decode(&crudo); err != nil {
		t.Fatalf("releyendo el documento crudo: %v", err)
	}
	if _, sigue := crudo["monitor_id"]; sigue {
		t.Errorf("el campo legacy monitor_id debe quedar borrado; tengo %v", crudo)
	}

	// La lectura siguiente es donde el monitor original desaparecía.
	releida, err := repo.GetConversation(ctx, convID)
	if err != nil {
		t.Fatalf("GetConversation posterior: %v", err)
	}
	if len(releida.MonitorIDs) != 2 || releida.MonitorIDs[0] != original || releida.MonitorIDs[1] != nuevo {
		t.Fatalf("la relectura perdió el monitor original: tengo %v, quiero [%s %s]",
			releida.MonitorIDs, original.Hex(), nuevo.Hex())
	}

	// Idempotencia sobre el documento ya migrado.
	otra, err := repo.AddMonitor(ctx, convID, releida.MonitorIDs, nuevo)
	if err != nil {
		t.Fatalf("re-agregar un monitor presente debe ser un no-op: %v", err)
	}
	if len(otra) != 2 {
		t.Errorf("re-agregar no debe cambiar el conjunto; tengo %v", otra)
	}

	// Compare-and-set: escribir contra un estado observado ya viejo no
	// debe pisar nada. Es la carrera entre la validación del tope en el
	// handler y esta escritura.
	if _, err := repo.AddMonitor(ctx, convID, []primitive.ObjectID{original}, tercero); !errors.Is(err, ErrConversationModified) {
		t.Fatalf("un observed obsoleto debe dar ErrConversationModified; tengo %v", err)
	}
	final, err := repo.GetConversation(ctx, convID)
	if err != nil {
		t.Fatalf("GetConversation final: %v", err)
	}
	if len(final.MonitorIDs) != 2 {
		t.Errorf("el CAS rechazado no debe haber escrito nada; tengo %v", final.MonitorIDs)
	}
}
