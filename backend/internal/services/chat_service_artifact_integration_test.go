package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/thureos/compliance/internal/database"
	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Este test necesita un MongoDB de verdad y se saltea salvo que se lo
// pidan EXPLÍCITAMENTE por MONGO_TEST_URI — mismo criterio y mismo
// comando que internal/repository/chat_repo_integration_test.go:
//
//	docker compose up -d
//	MONGO_TEST_URI=mongodb://localhost:27019 go test ./internal/services/ -run Integracion -v
//
// Usa una base descartable PROPIA (thureos_services_test, no
// thureos_repo_test): go test ./... corre paquetes distintos en
// paralelo, y el Drop() de setup de este helper pisaría la base del
// paquete repository si compartieran nombre.
const testServicesDBName = "thureos_services_test"

func conectarParaTestServicios(t *testing.T) (*database.MongoDB, func()) {
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
	db := client.Database(testServicesDBName)
	if err := db.Drop(ctx); err != nil {
		t.Fatalf("limpiando %s: %v", testServicesDBName, err)
	}
	return &database.MongoDB{Client: client, Database: db}, func() {
		bg := context.Background()
		_ = db.Drop(bg)
		_ = client.Disconnect(bg)
	}
}

// dbYaDesconectada arma un *database.MongoDB apuntado a un cliente que ya
// se desconectó — para forzar, sin tocar código de producción, que
// artifactRepo.Create falle contra Mongo real (InsertOne sobre un cliente
// desconectado devuelve mongo.ErrClientDisconnected). NewChatArtifactRepository
// puede llamarse sobre este handle sin fallar: ensureIndexes descarta su
// propio error.
func dbYaDesconectada(t *testing.T, uri string) *database.MongoDB {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("conectando (para desconectar) a %s: %v", uri, err)
	}
	if err := client.Disconnect(ctx); err != nil {
		t.Fatalf("desconectando el cliente: %v", err)
	}
	return &database.MongoDB{Client: client, Database: client.Database(testServicesDBName)}
}

// fakeDeepSeekServer sirve una única respuesta de chat completion fija, en
// el formato que askDeepSeek espera. finishReason "stop" (no "tool_calls")
// hace que el turno termine sin pasar por executeQuery — no hace falta
// simular la herramienta para probar la persistencia del artefacto.
func fakeDeepSeekServer(t *testing.T, content string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatDeepSeekResponse{
			Choices: []struct {
				Message      chatDeepSeekMessage `json:"message"`
				FinishReason string              `json:"finish_reason"`
			}{
				{Message: chatDeepSeekMessage{Role: "assistant", Content: content}, FinishReason: "stop"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func sembrarMonitorYConversacionParaChat(t *testing.T, ctx context.Context, monitorRepo *repository.MonitorRepository, chatRepo *repository.ChatRepository) (*models.Monitor, *models.ChatConversation) {
	t.Helper()
	monitor := &models.Monitor{
		Name:         "Transacciones",
		SourceType:   models.SourceCSV,
		CollectionID: "data_test_transacciones",
		Schema: []models.SchemaField{
			{Name: "monto", Type: models.FieldNumber},
			{Name: "region", Type: models.FieldString},
		},
		OwnerID: primitive.NewObjectID(),
	}
	if err := monitorRepo.Create(ctx, monitor); err != nil {
		t.Fatalf("creando monitor: %v", err)
	}

	conv := &models.ChatConversation{
		UserID:     primitive.NewObjectID(),
		MonitorIDs: []primitive.ObjectID{monitor.ID},
	}
	if err := chatRepo.CreateConversation(ctx, conv); err != nil {
		t.Fatalf("creando conversación: %v", err)
	}
	return monitor, conv
}

// TestIntegracionAsk_ArtifactoSePersisteYSeEnlazaAlMensaje es el
// contrapeso del tripwire de CreateMessage: antes de esta tarea, CUALQUIER
// turno cuya respuesta trajera un artefacto fallaba (CreateMessage
// rechazaba Artifact sin ArtifactID) y el usuario se quedaba sin
// respuesta. Este test prueba, de punta a punta contra Mongo real, que
// hoy: el artefacto queda en su propia colección, el mensaje lo referencia
// por id, y esa referencia sobrevive una relectura del hilo.
func TestIntegracionAsk_ArtifactoSePersisteYSeEnlazaAlMensaje(t *testing.T) {
	db, cerrar := conectarParaTestServicios(t)
	defer cerrar()
	ctx := context.Background()

	artifactRepo := repository.NewChatArtifactRepository(db)
	chatRepo := repository.NewChatRepository(db, artifactRepo)
	monitorRepo := repository.NewMonitorRepository(db)
	configRepo := repository.NewSystemConfigRepository(db)

	monitor, conv := sembrarMonitorYConversacionParaChat(t, ctx, monitorRepo, chatRepo)

	llmContent := `Acá está el monto total por región.

<artifact>
{"type": "chart", "title": "Monto por región",
 "sources": [{"monitor": "transacciones", "label": "Bancolombia",
              "query": {"aggregate": {"field": "monto", "function": "sum", "groupBy": "region"}}}],
 "chartSpec": {"chartType": "bar", "xKey": "_id", "yKeys": ["aggValue"],
               "labels": {"_id": "Región", "aggValue": "Monto total"}}}
</artifact>`
	server := fakeDeepSeekServer(t, llmContent)
	defer server.Close()

	if err := configRepo.Upsert(ctx, &models.SystemConfig{
		AI: models.AIConfig{Provider: models.AIProviderDeepSeek, APIKey: "test-key", BaseURL: server.URL, Model: "deepseek-v4-pro"},
	}); err != nil {
		t.Fatalf("guardando config de IA: %v", err)
	}

	chatService := NewChatService(chatRepo, monitorRepo, configRepo, artifactRepo)

	assistantMsg, err := chatService.Ask(ctx, conv.ID, conv.UserID, "¿Cuánto es el monto total por región?")
	if err != nil {
		t.Fatalf("Ask no debe fallar: %v", err)
	}

	if assistantMsg.Artifact == nil {
		t.Fatal("la respuesta debe traer el artefacto")
	}
	if assistantMsg.ArtifactID == nil || *assistantMsg.ArtifactID != assistantMsg.Artifact.ID {
		t.Fatalf("ArtifactID debe apuntar al artefacto persistido; tengo ArtifactID=%v Artifact.ID=%s", assistantMsg.ArtifactID, assistantMsg.Artifact.ID.Hex())
	}
	if len(assistantMsg.Artifact.MonitorIDs) != 1 || assistantMsg.Artifact.MonitorIDs[0] != monitor.ID {
		t.Fatalf("MonitorIDs debe salir de las sources del artefacto ([%s]); tengo %v", monitor.ID.Hex(), assistantMsg.Artifact.MonitorIDs)
	}
	if assistantMsg.Artifact.Saved {
		t.Error("un artefacto recién creado no debe nacer guardado")
	}
	if assistantMsg.Artifact.UserID != conv.UserID || assistantMsg.Artifact.ConversationID != conv.ID {
		t.Errorf("UserID/ConversationID del artefacto deben ser los de esta conversación; tengo %+v", assistantMsg.Artifact)
	}

	// Releer directo de la colección propia: el artefacto existe como
	// entidad, no solo como campo de salida en memoria.
	persisted, err := artifactRepo.GetByID(ctx, assistantMsg.Artifact.ID)
	if err != nil {
		t.Fatalf("el artefacto debe existir en chat_artifacts: %v", err)
	}
	if persisted.MessageID != assistantMsg.ID {
		t.Errorf("el artefacto persistido debe apuntar de vuelta al mensaje; tengo %s, quiero %s", persisted.MessageID.Hex(), assistantMsg.ID.Hex())
	}
	if len(persisted.Sources) != 1 || persisted.Sources[0].Monitor != "transacciones" {
		t.Errorf("las sources deben sobrevivir la persistencia; tengo %+v", persisted.Sources)
	}

	// GetByID decodifica un "saved" AUSENTE como false igual que uno
	// escrito en false — no alcanza para probar que el campo se escribió.
	// Se lee el documento crudo: DeleteUnsavedByConversation filtra
	// {"saved": false}, que en Mongo NO matchea un campo ausente, así que
	// si "saved" se omitiera al crear, los artefactos efímeros nunca se
	// limpiarían al borrar la conversación.
	var crudo bson.M
	if err := db.Collection("chat_artifacts").FindOne(ctx, bson.M{"_id": assistantMsg.Artifact.ID}).Decode(&crudo); err != nil {
		t.Fatalf("releyendo el artefacto crudo: %v", err)
	}
	if saved, presente := crudo["saved"]; !presente || saved != false {
		t.Errorf("\"saved\" debe escribirse explícitamente en false; tengo presente=%v valor=%v", presente, saved)
	}

	// Releer el hilo completo: es el camino que usa el handler al abrir
	// una conversación, y donde la referencia rota se manifestaría.
	history, err := chatRepo.ListMessagesByConversation(ctx, conv.ID)
	if err != nil {
		t.Fatalf("ListMessagesByConversation: %v", err)
	}
	var reloaded *models.ChatMessage
	for i := range history {
		if history[i].ID == assistantMsg.ID {
			reloaded = &history[i]
		}
	}
	if reloaded == nil {
		t.Fatal("el mensaje del asistente debe estar en el historial")
	}
	if reloaded.Artifact == nil || reloaded.Artifact.ID != assistantMsg.Artifact.ID {
		t.Fatalf("el artefacto debe seguir enlazado tras una relectura del hilo; tengo %+v", reloaded.Artifact)
	}
}

// TestIntegracionAsk_FalloAlPersistirArtefactoNoPierdeLaRespuesta prueba
// la mitad más estricta del Global Constraint: si guardar el artefacto
// falla, el usuario NO debe perder el texto que estaba esperando. Se
// fuerza el fallo pasándole a ChatService un ChatArtifactRepository
// montado sobre un cliente Mongo YA DESCONECTADO — sin tocar código de
// producción ni introducir un mock — mientras el ChatRepository sigue
// usando la conexión viva para el mensaje.
func TestIntegracionAsk_FalloAlPersistirArtefactoNoPierdeLaRespuesta(t *testing.T) {
	uri := os.Getenv("MONGO_TEST_URI")
	if uri == "" {
		t.Skip("MONGO_TEST_URI sin definir: test de integración salteado a propósito")
	}
	db, cerrar := conectarParaTestServicios(t)
	defer cerrar()
	ctx := context.Background()

	liveArtifactRepo := repository.NewChatArtifactRepository(db)
	chatRepo := repository.NewChatRepository(db, liveArtifactRepo)
	monitorRepo := repository.NewMonitorRepository(db)
	configRepo := repository.NewSystemConfigRepository(db)

	_, conv := sembrarMonitorYConversacionParaChat(t, ctx, monitorRepo, chatRepo)

	llmContent := `Acá está el monto total por región.

<artifact>
{"type": "chart", "title": "Monto por región",
 "sources": [{"monitor": "transacciones",
              "query": {"aggregate": {"field": "monto", "function": "sum", "groupBy": "region"}}}],
 "chartSpec": {"chartType": "bar", "xKey": "_id", "yKeys": ["aggValue"],
               "labels": {"_id": "Región", "aggValue": "Monto total"}}}
</artifact>`
	server := fakeDeepSeekServer(t, llmContent)
	defer server.Close()

	if err := configRepo.Upsert(ctx, &models.SystemConfig{
		AI: models.AIConfig{Provider: models.AIProviderDeepSeek, APIKey: "test-key", BaseURL: server.URL, Model: "deepseek-v4-pro"},
	}); err != nil {
		t.Fatalf("guardando config de IA: %v", err)
	}

	deadDB := dbYaDesconectada(t, uri)
	deadArtifactRepo := repository.NewChatArtifactRepository(deadDB)

	// ChatService recibe el repo de artefactos MUERTO; el resto de las
	// dependencias son las vivas de siempre.
	chatService := NewChatService(chatRepo, monitorRepo, configRepo, deadArtifactRepo)

	assistantMsg, err := chatService.Ask(ctx, conv.ID, conv.UserID, "¿Cuánto es el monto total por región?")
	if err != nil {
		t.Fatalf("Ask no debe fallar aunque el artefacto no se pueda guardar: %v", err)
	}
	if assistantMsg.Content == "" {
		t.Fatal("la respuesta de texto no debe perderse cuando el artefacto no se puede persistir")
	}
	if assistantMsg.Artifact != nil {
		t.Errorf("un artefacto que no se pudo guardar no debe viajar en la respuesta; tengo %+v", assistantMsg.Artifact)
	}
	if assistantMsg.ArtifactID != nil {
		t.Errorf("sin artefacto persistido, ArtifactID debe quedar nil; tengo %v", *assistantMsg.ArtifactID)
	}

	// Releer de la base viva: el mensaje debe haber quedado guardado SIN
	// artifact_id — no a medio enlazar con un artefacto que no existe.
	history, err := chatRepo.ListMessagesByConversation(ctx, conv.ID)
	if err != nil {
		t.Fatalf("ListMessagesByConversation: %v", err)
	}
	var reloaded *models.ChatMessage
	for i := range history {
		if history[i].ID == assistantMsg.ID {
			reloaded = &history[i]
		}
	}
	if reloaded == nil {
		t.Fatal("el mensaje del asistente debe existir en el historial")
	}
	if reloaded.Content == "" {
		t.Error("el texto persistido no debe estar vacío")
	}
	if reloaded.ArtifactID != nil {
		t.Errorf("el mensaje persistido no debe tener artifact_id; tengo %v", *reloaded.ArtifactID)
	}
}
