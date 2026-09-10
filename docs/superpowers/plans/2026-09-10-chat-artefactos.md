# Artefactos de primera clase — Plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Que un artefacto generado por el asistente se pueda guardar, invocar después con datos frescos, y exportar a Excel, PDF, HTML, CSV o PNG.

**Architecture:** `ChatArtifact` sale de `ChatMessage` a una colección propia con id. El artefacto **declara las consultas que lo alimentan** (`sources`) en vez de sus datos, así que renderizarlo es ejecutarlas: un solo camino de código para el render inicial, la invocación y las exportaciones. La instantánea pasa a ser una caché del último resultado. XLSX se genera en el backend con `excelize`; PDF y HTML en el frontend, donde viven los tokens de marca.

**Tech Stack:** Go 1.26, Fiber v2, MongoDB, `excelize` (ya en `go.mod`), Next.js 16, React 19, `jspdf` + `jspdf-autotable` (ya en `package.json`).

**Spec:** `docs/superpowers/specs/2026-09-10-chat-artefactos-design.md`

**Dependencias entre planes:**
- **Requiere** `2026-09-10-chat-multimonitor.md` implementado: reutiliza `buildMonitorAliases`, `resolveQueryMonitor` y `ChatQueryInput.Monitor`.
- **Se beneficia de** `2026-09-10-chat-formato-rico.md` (la tabla formateada), pero no lo requiere: son módulos distintos. Si el de formato rico ya está, la Task 13 lo reutiliza; si no, la tabla sale sin formato y se puede mejorar después.

## Global Constraints

- **Autorización:** `RunArtifact` valida `art.UserID == userID` (o `ErrArtifactNotFound`) y resuelve cada `source.Monitor` contra los alias derivados de **`art.MonitorIDs`**, nunca contra otra lista. Alias desconocido ⇒ error **sin ejecutar consulta**. Nunca se acepta un ObjectID crudo.
- **`art.MonitorIDs` se fija al crear** el artefacto, resolviendo los alias contra `conv.MonitorIDs` de ese momento. Un artefacto nunca puede ganar acceso a un monitor que su conversación de origen no tenía.
- **`sources` vacío ⇒ instantánea.** Una sola regla cubre los artefactos legacy y los `custom`: no se re-ejecutan, no se puede pulsar «actualizar».
- **`custom` no exporta a PDF ni XLSX.** No se puede rasterizar un iframe sandboxeado cross-origin. La opción no se ofrece.
- **`RunArtifact` no persiste nada.** Solo `POST /run` escribe `CachedData`/`RanAt`. Exportar re-ejecuta pero **no** muta el artefacto.
- **Los artefactos `custom` siguen en su iframe** `sandbox="allow-scripts"` sin `allow-same-origin`. Este plan no toca ese camino.
- **Sin migración de datos.** Los mensajes viejos conservan su artefacto embebido y se leen como instantánea.
- **Errores envueltos con contexto:** `fmt.Errorf("operación: %w", err)`.
- **Ningún componente escribe un color literal.** Se usan utilidades de token o `@/lib/semantic-colors`.

---

## Estructura de archivos

**Backend — nuevos:**
- `backend/internal/services/artifact_shape.go` — proyección, etiquetas, concatenación y pivote. Funciones puras.
- `backend/internal/services/artifact_shape_test.go`
- `backend/internal/services/artifact_service.go` — `RunArtifact` y la autorización
- `backend/internal/services/artifact_service_test.go`
- `backend/internal/services/artifact_xlsx.go` — generación del Excel
- `backend/internal/services/artifact_xlsx_test.go`
- `backend/internal/repository/chat_artifact_repo.go`
- `backend/internal/handlers/artifact_handler.go`

**Backend — modificados:**
- `backend/internal/models/chat.go` — `ChatArtifact` de primera clase, `LegacyChatArtifact`, `ArtifactSource`, `ChartSpec.Columns/Labels`
- `backend/internal/services/chat_query.go` y `chat_llm_parsing.go` — mover `ChatQueryInput` y `ChatAggregateSpec` a `models`
- `backend/internal/services/chat_service.go` — prompt con `sources`, persistir el artefacto al responder
- `backend/internal/repository/chat_repo.go` — borrado en cascada de artefactos no guardados
- `backend/internal/router/router.go`

**Frontend — nuevos:**
- `frontend/src/lib/api/artifacts.ts`
- `frontend/src/lib/artifact-report.ts` — documento HTML con marca; alimenta también al PDF
- `frontend/src/components/chat/artifact-library.tsx`
- `frontend/src/components/chat/export-menu.tsx`

**Frontend — modificados:**
- `frontend/src/lib/chat-export.ts` — se extiende
- `frontend/src/components/chat/artifact-canvas.tsx`
- `frontend/src/app/(dashboard)/chatbot/page.tsx`

---

## Task 1: Mover `ChatQueryInput` y `ChatAggregateSpec` a `models`

Prerrequisito mecánico. `ArtifactSource` es un modelo persistido y necesita embeber `ChatQueryInput`; `models` no puede importar `services` porque `services` ya importa `models` — sería un ciclo y no compila.

**Files:**
- Create: `backend/internal/models/chat_query.go`
- Modify: `backend/internal/services/chat_query.go:11-23`, `backend/internal/services/chat_llm_parsing.go:17-22`

**Interfaces:**
- Produces: `models.ChatQueryInput`, `models.ChatAggregateSpec`

- [ ] **Step 1: Crear el archivo en `models`**

Crear `backend/internal/models/chat_query.go` moviendo los dos structs tal cual:

```go
package models

// ChatAggregateSpec agrupa y agrega sin umbral — a diferencia de
// AggregateCondition (pensada para el motor de reglas, que corta por
// threshold), esta trae TODOS los grupos: el chatbot quiere el dato, no
// una decisión de match/no-match.
//
// Vive en models y no en services porque ArtifactSource la persiste: un
// artefacto guarda la consulta que lo produjo, no sus números.
type ChatAggregateSpec struct {
	Field      string      `bson:"field" json:"field"`
	Function   AggFunction `bson:"function" json:"function"`
	GroupBy    string      `bson:"group_by,omitempty" json:"groupBy,omitempty"`
	TimeField  string      `bson:"time_field,omitempty" json:"timeField,omitempty"`
	TimeWindow string      `bson:"time_window,omitempty" json:"timeWindow,omitempty"`
	Filter     []Condition `bson:"filter,omitempty" json:"filter,omitempty"`
}

// ChatQueryInput es la forma exacta del "input" que el LLM manda al
// llamar a la herramienta query_monitor_data, y también lo que un
// ArtifactSource persiste.
type ChatQueryInput struct {
	// Monitor es el ALIAS del monitor a consultar, no su ObjectID.
	Monitor        string             `bson:"monitor" json:"monitor"`
	ConditionGroup *ConditionGroup    `bson:"condition_group,omitempty" json:"conditionGroup,omitempty"`
	Aggregate      *ChatAggregateSpec `bson:"aggregate,omitempty" json:"aggregate,omitempty"`
	Limit          int                `bson:"limit,omitempty" json:"limit,omitempty"`
}
```

- [ ] **Step 2: Borrar los originales y crear alias de compatibilidad**

En `backend/internal/services/chat_query.go`, borrar el struct `ChatAggregateSpec`. En `chat_llm_parsing.go`, borrar el struct `ChatQueryInput`. Agregar en `chat_query.go`, para no tener que reescribir todos los usos del paquete de una sola vez:

```go
// Alias hacia models: los tipos se mudaron allá porque ArtifactSource los
// persiste (models no puede importar services sin crear un ciclo).
type (
	ChatQueryInput    = models.ChatQueryInput
	ChatAggregateSpec = models.ChatAggregateSpec
)
```

Son alias de tipo (`=`), no definiciones nuevas: el código existente del paquete sigue compilando sin cambios.

- [ ] **Step 3: Compilar y correr los tests**

Run: `cd backend && go build ./... && go test ./internal/services/ ./internal/models/`
Expected: compila; todos los tests siguen pasando sin modificarse.

- [ ] **Step 4: Commit**

```bash
cd backend && gofmt -w internal/models/ internal/services/
git add backend/internal/models/chat_query.go backend/internal/services/chat_query.go backend/internal/services/chat_llm_parsing.go
git commit -m "refactor(chat): mover ChatQueryInput y ChatAggregateSpec a models"
```

---

## Task 2: Modelo del artefacto de primera clase

**Files:**
- Modify: `backend/internal/models/chat.go`
- Test: `backend/internal/models/chat_test.go` (existe si se hizo el plan multi-monitor; si no, crear)

**Interfaces:**
- Produces: `models.ArtifactSource`, `models.ChatArtifact` (entidad), `models.LegacyChatArtifact`, `ChartSpec.Columns`, `ChartSpec.Labels`, `func (a *ChatArtifact) IsRerunnable() bool`, `func FromLegacyArtifact(l *LegacyChatArtifact) *ChatArtifact`

- [ ] **Step 1: Escribir el test que falla**

Agregar a `backend/internal/models/chat_test.go`:

```go
func TestIsRerunnable_ConSourcesSi(t *testing.T) {
	art := ChatArtifact{
		Type:    ChatArtifactTable,
		Sources: []ArtifactSource{{Monitor: "transacciones"}},
	}
	if !art.IsRerunnable() {
		t.Error("un artefacto con sources debe ser re-ejecutable")
	}
}

// La regla del spec es una sola: sources vacío ⇒ instantánea. Cubre por
// igual a los artefactos legacy y a los custom.
func TestIsRerunnable_SinSourcesNo(t *testing.T) {
	legacy := ChatArtifact{
		Type:      ChatArtifactTable,
		ChartSpec: &ChartSpec{Data: []map[string]interface{}{{"a": 1}}},
	}
	if legacy.IsRerunnable() {
		t.Error("un artefacto sin sources es instantánea, no re-ejecutable")
	}

	custom := ChatArtifact{Type: ChatArtifactCustom, Code: "<p>hola</p>"}
	if custom.IsRerunnable() {
		t.Error("un artefacto custom nunca es re-ejecutable")
	}
}

func TestFromLegacyArtifact_ConservaLosDatosComoInstantanea(t *testing.T) {
	legacy := &LegacyChatArtifact{
		Type:      ChatArtifactTable,
		Title:     "Ventas",
		ChartSpec: &ChartSpec{Data: []map[string]interface{}{{"region": "Caribe"}}},
	}
	art := FromLegacyArtifact(legacy)

	if art.Title != "Ventas" || art.Type != ChatArtifactTable {
		t.Errorf("no conservó tipo/título: %+v", art)
	}
	if len(art.ChartSpec.Data) != 1 {
		t.Error("debe conservar los datos guardados")
	}
	if art.IsRerunnable() {
		t.Error("un artefacto legacy nunca es re-ejecutable")
	}
	if !art.ID.IsZero() {
		t.Error("un artefacto legacy no tiene id propio")
	}
}

func TestFromLegacyArtifact_NilDevuelveNil(t *testing.T) {
	if FromLegacyArtifact(nil) != nil {
		t.Error("nil debe devolver nil")
	}
}
```

- [ ] **Step 2: Correr el test y verificar que falla**

Run: `cd backend && go test ./internal/models/ -run 'TestIsRerunnable|TestFromLegacyArtifact' -v`
Expected: FAIL — no compila, faltan los tipos.

- [ ] **Step 3: Implementar**

En `backend/internal/models/chat.go`:

Agregar a `ChartSpec` los dos campos de presentación:

```go
type ChartSpec struct {
	ChartType ChatChartType            `bson:"chart_type,omitempty" json:"chartType,omitempty"`
	Data      []map[string]interface{} `bson:"data,omitempty" json:"data,omitempty"`
	XKey      string                   `bson:"x_key,omitempty" json:"xKey,omitempty"`
	YKeys     []string                 `bson:"y_keys,omitempty" json:"yKeys,omitempty"`
	// Columns: proyección — qué columnas mostrar y en qué orden. Vacío =
	// todas. Sin esto, una tabla sobre filas crudas vuelca el documento
	// entero de Mongo, con su _id incluido.
	Columns []string `bson:"columns,omitempty" json:"columns,omitempty"`
	// Labels: nombre de presentación por columna. Se aplica al renderizar
	// y NUNCA toca los datos ni las claves de XKey/YKeys — mantenerlos
	// separados evita que renombrar algo rompa la referencia. Sin esto,
	// un gráfico sobre una agregación sale con los ejes rotulados "_id" y
	// "aggValue", que son nombres internos del pipeline.
	Labels map[string]string `bson:"labels,omitempty" json:"labels,omitempty"`
}
```

> `Data` pasa a llevar `omitempty`: en el camino nuevo viene vacío porque los datos salen de `sources`.

Agregar `ArtifactSource` y renombrar el artefacto embebido:

```go
// ArtifactSource es una consulta que alimenta a un artefacto. El artefacto
// declara de DÓNDE salen sus datos, no cuáles son: así invocarlo seis
// meses después trae datos de hoy, y el render inicial y la exportación
// recorren exactamente el mismo camino.
type ArtifactSource struct {
	// Monitor es el alias, resuelto contra ChatArtifact.MonitorIDs.
	Monitor string         `bson:"monitor" json:"monitor"`
	Query   ChatQueryInput `bson:"query" json:"query"`
	// Label nombra la serie/origen cuando hay más de una fuente.
	Label string `bson:"label,omitempty" json:"label,omitempty"`
}

// LegacyChatArtifact es el artefacto embebido en ChatMessage tal como se
// guardaba antes de que los artefactos fueran entidades con id. Nunca se
// escribe; solo se lee, y FromLegacyArtifact lo convierte. Mismo criterio
// que LegacyMonitorID en ChatConversation.
type LegacyChatArtifact struct {
	Type      ChatArtifactType `bson:"type" json:"type"`
	Title     string           `bson:"title" json:"title"`
	ChartSpec *ChartSpec       `bson:"chart_spec,omitempty" json:"chartSpec,omitempty"`
	Code      string           `bson:"code,omitempty" json:"code,omitempty"`
}

// ChatArtifact es la entidad persistida en la colección chat_artifacts.
type ChatArtifact struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID         primitive.ObjectID `bson:"user_id" json:"userId"`
	ConversationID primitive.ObjectID `bson:"conversation_id" json:"conversationId"`
	MessageID      primitive.ObjectID `bson:"message_id,omitempty" json:"messageId,omitempty"`
	// MonitorIDs es la allowlist del artefacto: se fija al crearlo a
	// partir de los monitores de su conversación, y es contra esto que se
	// resuelven los alias de las sources al re-ejecutar. Un artefacto
	// nunca puede ganar acceso a un monitor que su conversación no tenía.
	MonitorIDs []primitive.ObjectID `bson:"monitor_ids,omitempty" json:"monitorIds"`

	Type      ChatArtifactType `bson:"type" json:"type"`
	Title     string           `bson:"title" json:"title"`
	ChartSpec *ChartSpec       `bson:"chart_spec,omitempty" json:"chartSpec,omitempty"`
	Code      string           `bson:"code,omitempty" json:"code,omitempty"`
	Sources   []ArtifactSource `bson:"sources,omitempty" json:"sources,omitempty"`

	// Saved/SavedName: un artefacto nace efímero (vive en su mensaje) y se
	// vuelve permanente cuando el usuario lo guarda con un nombre.
	Saved     bool   `bson:"saved" json:"saved"`
	SavedName string `bson:"saved_name,omitempty" json:"savedName,omitempty"`

	// CachedData/RanAt: resultado de la última ejecución. Solo POST /run
	// los escribe; exportar re-ejecuta pero no muta el artefacto.
	CachedData []map[string]interface{} `bson:"cached_data,omitempty" json:"cachedData,omitempty"`
	RanAt      time.Time                `bson:"ran_at,omitempty" json:"ranAt,omitempty"`

	CreatedAt time.Time `bson:"created_at" json:"createdAt"`
}

// IsRerunnable: la regla del spec es una sola — sources vacío significa
// instantánea. Cubre por igual a los artefactos legacy (guardados antes
// de este cambio) y a los custom (HTML/JS con sus datos embebidos, sin
// consulta que re-ejecutar).
func (a *ChatArtifact) IsRerunnable() bool {
	return len(a.Sources) > 0
}

// FromLegacyArtifact adapta un artefacto embebido viejo a la entidad
// nueva. Queda sin ID y sin Sources: es una instantánea, no se puede
// guardar ni re-ejecutar.
func FromLegacyArtifact(l *LegacyChatArtifact) *ChatArtifact {
	if l == nil {
		return nil
	}
	return &ChatArtifact{
		Type:      l.Type,
		Title:     l.Title,
		ChartSpec: l.ChartSpec,
		Code:      l.Code,
	}
}
```

En `ChatMessage`, reemplazar el campo embebido:

```go
type ChatMessage struct {
	ID             primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	ConversationID primitive.ObjectID  `bson:"conversation_id" json:"conversationId"`
	Role           ChatRole            `bson:"role" json:"role"`
	Content        string              `bson:"content" json:"content"`
	ArtifactID     *primitive.ObjectID `bson:"artifact_id,omitempty" json:"artifactId,omitempty"`
	// LegacyArtifact: mensajes guardados antes de que los artefactos
	// tuvieran id. Solo lectura; el repositorio lo convierte con
	// FromLegacyArtifact y lo expone en Artifact.
	LegacyArtifact *LegacyChatArtifact `bson:"artifact,omitempty" json:"-"`
	// Artifact es el artefacto ya resuelto que se le manda al frontend.
	// No se persiste: es un campo de salida.
	Artifact  *ChatArtifact `bson:"-" json:"artifact,omitempty"`
	CreatedAt time.Time     `bson:"created_at" json:"createdAt"`
}
```

- [ ] **Step 4: Correr los tests y verificar que pasan**

Run: `cd backend && go test ./internal/models/ -v`
Expected: PASS.

`go build ./...` va a fallar en `chat_service.go` (usa `models.ChatArtifact` con la forma vieja) y en `chat_llm_parsing.go`. Se cierra en las tareas 4 y 8.

- [ ] **Step 5: No commitear todavía**

Seguir a la Task 3.

---

## Task 3: Repositorio de artefactos

**Files:**
- Create: `backend/internal/repository/chat_artifact_repo.go`
- Modify: `backend/internal/repository/chat_repo.go` (`DeleteConversation`, lectura de mensajes)

**Interfaces:**
- Produces:
  - `func NewChatArtifactRepository(db *database.MongoDB) *ChatArtifactRepository`
  - `Create(ctx, art *models.ChatArtifact) error`
  - `GetByID(ctx, id primitive.ObjectID) (*models.ChatArtifact, error)`
  - `ListSavedByUser(ctx, userID primitive.ObjectID) ([]models.ChatArtifact, error)`
  - `SetSaved(ctx, id primitive.ObjectID, saved bool, name string) error`
  - `SetCache(ctx, id primitive.ObjectID, data []map[string]interface{}, ranAt time.Time) error`
  - `Delete(ctx, id primitive.ObjectID) error`
  - `DeleteUnsavedByConversation(ctx, conversationID primitive.ObjectID) error`

- [ ] **Step 1: Crear el repositorio**

```go
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/thureos/compliance/internal/database"
	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ChatArtifactRepository struct {
	artifacts *mongo.Collection
}

func NewChatArtifactRepository(db *database.MongoDB) *ChatArtifactRepository {
	r := &ChatArtifactRepository{artifacts: db.Collection("chat_artifacts")}
	r.ensureIndexes()
	return r
}

func (r *ChatArtifactRepository) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _ = r.artifacts.Indexes().CreateMany(ctx, []mongo.IndexModel{
		// La biblioteca: los guardados del usuario, más recientes primero.
		{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "saved", Value: 1}, {Key: "created_at", Value: -1}}},
		// El borrado en cascada al eliminar una conversación.
		{Keys: bson.D{{Key: "conversation_id", Value: 1}}},
	})
}

func (r *ChatArtifactRepository) Create(ctx context.Context, art *models.ChatArtifact) error {
	art.CreatedAt = time.Now()
	res, err := r.artifacts.InsertOne(ctx, art)
	if err != nil {
		return fmt.Errorf("creando artefacto: %w", err)
	}
	art.ID = res.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *ChatArtifactRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.ChatArtifact, error) {
	var art models.ChatArtifact
	if err := r.artifacts.FindOne(ctx, bson.M{"_id": id}).Decode(&art); err != nil {
		return nil, fmt.Errorf("artefacto %s no encontrado: %w", id.Hex(), err)
	}
	return &art, nil
}

// ListSavedByUser devuelve la biblioteca del usuario. Solo los guardados:
// los efímeros viven en su mensaje y no tienen por qué ensuciar la lista.
func (r *ChatArtifactRepository) ListSavedByUser(ctx context.Context, userID primitive.ObjectID) ([]models.ChatArtifact, error) {
	cursor, err := r.artifacts.Find(ctx,
		bson.M{"user_id": userID, "saved": true},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("listando artefactos: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	results := []models.ChatArtifact{}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("decodificando artefactos: %w", err)
	}
	return results, nil
}

func (r *ChatArtifactRepository) SetSaved(ctx context.Context, id primitive.ObjectID, saved bool, name string) error {
	_, err := r.artifacts.UpdateByID(ctx, id, bson.M{"$set": bson.M{"saved": saved, "saved_name": name}})
	if err != nil {
		return fmt.Errorf("guardando artefacto %s: %w", id.Hex(), err)
	}
	return nil
}

// SetCache escribe el resultado de la última ejecución. Lo llama SOLO el
// endpoint /run: exportar re-ejecuta pero no muta el artefacto, para que
// bajar un Excel no le cambie la fecha a quien lo esté mirando.
func (r *ChatArtifactRepository) SetCache(ctx context.Context, id primitive.ObjectID, data []map[string]interface{}, ranAt time.Time) error {
	_, err := r.artifacts.UpdateByID(ctx, id, bson.M{"$set": bson.M{"cached_data": data, "ran_at": ranAt}})
	if err != nil {
		return fmt.Errorf("cacheando artefacto %s: %w", id.Hex(), err)
	}
	return nil
}

func (r *ChatArtifactRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	if _, err := r.artifacts.DeleteOne(ctx, bson.M{"_id": id}); err != nil {
		return fmt.Errorf("borrando artefacto %s: %w", id.Hex(), err)
	}
	return nil
}

// DeleteUnsavedByConversation limpia los efímeros al borrar una
// conversación. Los guardados sobreviven: la biblioteca es del usuario,
// no de la conversación, y el valor de un artefacto guardado está en sus
// sources, no en el hilo que lo originó.
func (r *ChatArtifactRepository) DeleteUnsavedByConversation(ctx context.Context, conversationID primitive.ObjectID) error {
	_, err := r.artifacts.DeleteMany(ctx, bson.M{"conversation_id": conversationID, "saved": false})
	if err != nil {
		return fmt.Errorf("borrando artefactos de la conversación %s: %w", conversationID.Hex(), err)
	}
	return nil
}
```

- [ ] **Step 2: Resolver el artefacto al leer mensajes**

En `chat_repo.go`, `ChatRepository` recibe el repositorio de artefactos para poder resolver `ArtifactID` y `LegacyArtifact` en un solo lugar — así ningún llamador tiene que saber que existen dos formas.

```go
type ChatRepository struct {
	conversations *mongo.Collection
	messages      *mongo.Collection
	artifacts     *ChatArtifactRepository
}

func NewChatRepository(db *database.MongoDB, artifacts *ChatArtifactRepository) *ChatRepository {
	r := &ChatRepository{
		conversations: db.Collection("chat_conversations"),
		messages:      db.Collection("chat_messages"),
		artifacts:     artifacts,
	}
	r.ensureIndexes()
	return r
}
```

Al final de `ListMessagesByConversation`, antes del `return`:

```go
	// Resolver el artefacto de cada mensaje: los nuevos apuntan por
	// artifact_id, los viejos lo llevan embebido. El frontend recibe
	// siempre la misma forma y no tiene que distinguir.
	for i := range results {
		switch {
		case results[i].ArtifactID != nil:
			art, err := r.artifacts.GetByID(ctx, *results[i].ArtifactID)
			if err == nil {
				results[i].Artifact = art
			}
			// Un artefacto borrado deja el mensaje sin artefacto, no rompe
			// la carga del hilo.
		case results[i].LegacyArtifact != nil:
			results[i].Artifact = models.FromLegacyArtifact(results[i].LegacyArtifact)
		}
		results[i].LegacyArtifact = nil
	}
	return results, nil
```

En `DeleteConversation`, antes de borrar los mensajes:

```go
	if err := r.artifacts.DeleteUnsavedByConversation(ctx, id); err != nil {
		return err
	}
```

- [ ] **Step 3: Cablear la construcción**

Buscar dónde se construye el repositorio: `grep -rn "NewChatRepository" backend/`. Crear el de artefactos antes y pasárselo.

- [ ] **Step 4: Compilar**

Run: `cd backend && go build ./internal/repository/ ./internal/models/`
Expected: compila.

- [ ] **Step 5: Commit**

```bash
cd backend && gofmt -w internal/models/ internal/repository/
git add backend/internal/models/chat.go backend/internal/models/chat_test.go backend/internal/repository/
git commit -m "feat(artefactos): entidad de primera clase con colección propia"
```

---

## Task 4: `ParseArtifact` con `sources`

Esta tarea corrige una regresión que descartaría **todos** los artefactos nuevos: `ParseArtifact` hoy exige `chartSpec` con `len(Data) > 0`, y en el camino nuevo `Data` viene vacío porque los datos salen de `sources`.

**Files:**
- Modify: `backend/internal/services/chat_llm_parsing.go:43-75`
- Test: `backend/internal/services/chat_llm_parsing_test.go`

- [ ] **Step 1: Escribir el test que falla**

```go
// REGRESIÓN: bajo el contrato nuevo los datos vienen de sources y Data
// llega vacío. La validación vieja (len(Data) > 0) rechazaría todos los
// artefactos nuevos, en silencio.
func TestParseArtifact_ChartConSourcesYSinData(t *testing.T) {
	raw := []byte(`{"type":"chart","title":"Ventas",
	  "sources":[{"monitor":"transacciones","query":{"monitor":"transacciones","aggregate":{"field":"monto","function":"sum","groupBy":"region"}}}],
	  "chartSpec":{"chartType":"bar","xKey":"_id","yKeys":["aggValue"]}}`)

	art, err := ParseArtifact(raw)
	if err != nil {
		t.Fatalf("un chart con sources y sin data debe ser válido: %v", err)
	}
	if len(art.Sources) != 1 {
		t.Fatalf("quiero 1 source, tengo %d", len(art.Sources))
	}
}

func TestParseArtifact_TableConDataYSinSourcesSigueSiendoValido(t *testing.T) {
	raw := []byte(`{"type":"table","title":"T","chartSpec":{"data":[{"a":1}]}}`)
	art, err := ParseArtifact(raw)
	if err != nil {
		t.Fatalf("una instantánea debe seguir siendo válida: %v", err)
	}
	if art.IsRerunnable() {
		t.Error("sin sources no es re-ejecutable")
	}
}

func TestParseArtifact_SinSourcesNiDataEsInvalido(t *testing.T) {
	raw := []byte(`{"type":"table","title":"T","chartSpec":{}}`)
	if _, err := ParseArtifact(raw); err == nil {
		t.Fatal("un artefacto sin datos ni sources debe ser rechazado")
	}
}

func TestParseArtifact_SourceSinMonitorEsInvalido(t *testing.T) {
	raw := []byte(`{"type":"table","title":"T",
	  "sources":[{"query":{"aggregate":{"field":"monto","function":"sum"}}}]}`)
	if _, err := ParseArtifact(raw); err == nil {
		t.Fatal("una source sin monitor debe ser rechazada")
	}
}

// Un chart con varias fuentes se pivotea sobre xKey; sin xKey no hay
// sobre qué pivotear.
func TestParseArtifact_ChartMultiFuenteSinXKeyEsInvalido(t *testing.T) {
	raw := []byte(`{"type":"chart","title":"T",
	  "sources":[
	    {"monitor":"a","query":{"monitor":"a","aggregate":{"field":"m","function":"sum"}}},
	    {"monitor":"b","query":{"monitor":"b","aggregate":{"field":"m","function":"sum"}}}],
	  "chartSpec":{"chartType":"bar","yKeys":["aggValue"]}}`)
	if _, err := ParseArtifact(raw); err == nil {
		t.Fatal("un chart multi-fuente sin xKey debe ser rechazado")
	}
}
```

- [ ] **Step 2: Correr el test y verificar que falla**

Run: `cd backend && go test ./internal/services/ -run TestParseArtifact -v`
Expected: FAIL.

- [ ] **Step 3: Implementar**

Reemplazar el cuerpo de `ParseArtifact`:

```go
func ParseArtifact(raw json.RawMessage) (*models.ChatArtifact, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var artifact models.ChatArtifact
	if err := json.Unmarshal(raw, &artifact); err != nil {
		return nil, fmt.Errorf("artefacto inválido: %w", err)
	}

	switch artifact.Type {
	case models.ChatArtifactChart, models.ChatArtifactTable:
		// Válido por cualquiera de los dos caminos: sources (se ejecuta) o
		// data inline (instantánea/legacy). Exigir data como antes
		// rechazaría todos los artefactos del contrato nuevo.
		hasData := artifact.ChartSpec != nil && len(artifact.ChartSpec.Data) > 0
		if len(artifact.Sources) == 0 && !hasData {
			return nil, fmt.Errorf("un artefacto %s requiere 'sources' o 'chartSpec.data'", artifact.Type)
		}
		for i, src := range artifact.Sources {
			if strings.TrimSpace(src.Monitor) == "" {
				return nil, fmt.Errorf("la fuente %d no indica 'monitor'", i)
			}
			// La query se valida con el mismo parser que el tool call: un
			// solo criterio de qué es una consulta bien formada.
			encoded, err := json.Marshal(src.Query)
			if err != nil {
				return nil, fmt.Errorf("la fuente %d tiene una query inválida: %w", i, err)
			}
			if _, err := ParseQueryToolInput(encoded); err != nil {
				return nil, fmt.Errorf("la fuente %d tiene una query inválida: %w", i, err)
			}
		}
		if artifact.Type == models.ChatArtifactChart {
			if artifact.ChartSpec == nil {
				return nil, fmt.Errorf("artefacto chart requiere chartSpec")
			}
			switch artifact.ChartSpec.ChartType {
			case models.ChatChartBar, models.ChatChartLine, models.ChatChartPie:
			default:
				return nil, fmt.Errorf("chartType inválido: %q", artifact.ChartSpec.ChartType)
			}
			// Varias fuentes se pivotean sobre XKey (una serie por
			// fuente); sin XKey no hay sobre qué pivotear.
			if len(artifact.Sources) > 1 && artifact.ChartSpec.XKey == "" {
				return nil, fmt.Errorf("un chart con varias fuentes requiere chartSpec.xKey")
			}
		}
	case models.ChatArtifactCustom:
		if artifact.Code == "" {
			return nil, fmt.Errorf("artefacto custom requiere code")
		}
	default:
		return nil, fmt.Errorf("tipo de artefacto inválido: %q", artifact.Type)
	}
	return &artifact, nil
}
```

Si `src.Query.Monitor` viene vacío pero `src.Monitor` no, copiarlo antes de validar:

```go
			if artifact.Sources[i].Query.Monitor == "" {
				artifact.Sources[i].Query.Monitor = src.Monitor
			}
```

(Va antes del `json.Marshal`, para que el LLM no tenga que repetir el alias en los dos lados.)

- [ ] **Step 4: Correr los tests y verificar que pasan**

Run: `cd backend && go test ./internal/services/ -run TestParseArtifact -v`
Expected: PASS, los 5 tests.

- [ ] **Step 5: Commit**

```bash
cd backend && gofmt -w internal/services/
git add backend/internal/services/chat_llm_parsing.go backend/internal/services/chat_llm_parsing_test.go
git commit -m "feat(artefactos): validar sources en ParseArtifact sin romper las instantáneas"
```

---

## Task 5: Proyección, etiquetas, concatenación y pivote

Funciones puras, sin Mongo. Es donde vive la diferencia entre `table` (concatena) y `chart` (pivotea).

**Files:**
- Create: `backend/internal/services/artifact_shape.go`
- Create: `backend/internal/services/artifact_shape_test.go`

**Interfaces:**
- Produces:
  - `func projectRows(rows []map[string]interface{}, columns []string) []map[string]interface{}`
  - `func concatSources(results [][]map[string]interface{}, labels []string) []map[string]interface{}`
  - `func pivotSources(results [][]map[string]interface{}, labels []string, xKey, valueKey string) []map[string]interface{}`

- [ ] **Step 1: Escribir el test que falla**

```go
package services

import (
	"testing"
)

func TestProjectRows_RecortaYOrdena(t *testing.T) {
	rows := []map[string]interface{}{
		{"_id": "x", "monto": 100, "region": "Caribe", "extra": "ruido"},
	}
	got := projectRows(rows, []string{"region", "monto"})

	if len(got) != 1 || len(got[0]) != 2 {
		t.Fatalf("quiero 1 fila de 2 columnas, tengo %v", got)
	}
	if got[0]["region"] != "Caribe" || got[0]["monto"] != 100 {
		t.Errorf("valores incorrectos: %v", got[0])
	}
	if _, hay := got[0]["_id"]; hay {
		t.Error("_id no fue proyectado y no debería aparecer")
	}
}

func TestProjectRows_SinColumnasDevuelveTodo(t *testing.T) {
	rows := []map[string]interface{}{{"a": 1, "b": 2}}
	got := projectRows(rows, nil)
	if len(got[0]) != 2 {
		t.Errorf("sin proyección deben venir todas las columnas: %v", got[0])
	}
}

func TestProjectRows_ColumnaInexistenteSeOmite(t *testing.T) {
	rows := []map[string]interface{}{{"a": 1}}
	got := projectRows(rows, []string{"a", "no_existe"})
	if _, hay := got[0]["no_existe"]; hay {
		t.Error("una columna que no está en la fila no debe inventarse")
	}
}

func TestConcatSources_ApilaConDiscriminador(t *testing.T) {
	results := [][]map[string]interface{}{
		{{"_id": "Caribe", "aggValue": 10}},
		{{"_id": "Andina", "aggValue": 20}},
	}
	got := concatSources(results, []string{"Bancolombia", "Davivienda"})

	if len(got) != 2 {
		t.Fatalf("quiero 2 filas, tengo %d", len(got))
	}
	if got[0]["_monitor"] != "Bancolombia" || got[1]["_monitor"] != "Davivienda" {
		t.Errorf("falta o está mal el discriminador: %v", got)
	}
}

// Concatenar un chart multi-fuente daría UNA serie con las x repetidas,
// que Recharts apila mal. Una serie por monitor exige pivotear.
func TestPivotSources_UnaColumnaPorFuente(t *testing.T) {
	results := [][]map[string]interface{}{
		{{"_id": "Caribe", "aggValue": 100}, {"_id": "Andina", "aggValue": 80}},
		{{"_id": "Caribe", "aggValue": 60}},
	}
	got := pivotSources(results, []string{"Bancolombia", "Davivienda"}, "_id", "aggValue")

	if len(got) != 2 {
		t.Fatalf("quiero 2 filas (una por valor de x), tengo %d: %v", len(got), got)
	}

	byX := map[interface{}]map[string]interface{}{}
	for _, row := range got {
		byX[row["_id"]] = row
	}
	if byX["Caribe"]["Bancolombia"] != 100 || byX["Caribe"]["Davivienda"] != 60 {
		t.Errorf("fila Caribe incorrecta: %v", byX["Caribe"])
	}
	if byX["Andina"]["Bancolombia"] != 80 {
		t.Errorf("fila Andina incorrecta: %v", byX["Andina"])
	}
	// Una fuente sin dato para esa x deja nil: Recharts corta la línea,
	// que es lo correcto — no había dato, no es un cero.
	if v, hay := byX["Andina"]["Davivienda"]; hay && v != nil {
		t.Errorf("Davivienda no tiene dato para Andina, debería ser nil: %v", v)
	}
}

func TestPivotSources_ConservaElOrdenDeAparicion(t *testing.T) {
	results := [][]map[string]interface{}{
		{{"_id": "z", "aggValue": 1}, {"_id": "a", "aggValue": 2}},
	}
	got := pivotSources(results, []string{"S"}, "_id", "aggValue")
	if got[0]["_id"] != "z" || got[1]["_id"] != "a" {
		t.Errorf("el pivote debe conservar el orden en que llegaron las x (el pipeline ya ordenó): %v", got)
	}
}
```

- [ ] **Step 2: Correr el test y verificar que falla**

Run: `cd backend && go test ./internal/services/ -run 'TestProjectRows|TestConcatSources|TestPivotSources' -v`
Expected: FAIL — funciones no definidas.

- [ ] **Step 3: Implementar**

Crear `backend/internal/services/artifact_shape.go`:

```go
package services

// Moldeado de los resultados de las sources de un artefacto antes de
// renderizarlos. Todo lo de acá es puro: entra data, sale data.

// artifactMonitorColumn es el nombre de la columna discriminadora que
// concatSources agrega cuando hay más de una fuente.
const artifactMonitorColumn = "_monitor"

// projectRows recorta cada fila a las columnas pedidas. Sin columnas,
// devuelve las filas como vinieron.
//
// Hace falta porque QueryData devuelve el documento COMPLETO de Mongo:
// sin proyección, una tabla sobre filas crudas vuelca todos los campos,
// incluido el _id.
func projectRows(rows []map[string]interface{}, columns []string) []map[string]interface{} {
	if len(columns) == 0 {
		return rows
	}
	out := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		projected := make(map[string]interface{}, len(columns))
		for _, col := range columns {
			// Una columna que no está en la fila se omite, no se inventa
			// como nil: así la tabla no muestra una columna fantasma.
			if v, ok := row[col]; ok {
				projected[col] = v
			}
		}
		out = append(out, projected)
	}
	return out
}

// concatSources apila los resultados de varias fuentes agregando una
// columna que dice de cuál vino cada fila. Es la forma correcta para una
// TABLA comparativa.
func concatSources(results [][]map[string]interface{}, labels []string) []map[string]interface{} {
	out := []map[string]interface{}{}
	for i, rows := range results {
		label := ""
		if i < len(labels) {
			label = labels[i]
		}
		for _, row := range rows {
			merged := make(map[string]interface{}, len(row)+1)
			for k, v := range row {
				merged[k] = v
			}
			merged[artifactMonitorColumn] = label
			out = append(out, merged)
		}
	}
	return out
}

// pivotSources arma una fila por valor de xKey y una columna por fuente.
// Es la forma correcta para un GRÁFICO con una serie por monitor:
// concatenar daría una sola serie con las x repetidas, que Recharts apila
// mal en vez de dibujar dos series.
//
// Una fuente que no tiene dato para una x deja la celda ausente (nil al
// serializar): Recharts corta ahí la línea, que es lo correcto — no había
// dato, y un cero mentiría.
//
// Conserva el orden en que aparecen las x: el pipeline de agregación ya
// las ordenó por valor, y reordenar acá tiraría ese trabajo.
func pivotSources(results [][]map[string]interface{}, labels []string, xKey, valueKey string) []map[string]interface{} {
	order := []interface{}{}
	byX := map[interface{}]map[string]interface{}{}

	for i, rows := range results {
		label := ""
		if i < len(labels) {
			label = labels[i]
		}
		for _, row := range rows {
			x, ok := row[xKey]
			if !ok {
				continue
			}
			pivoted, seen := byX[x]
			if !seen {
				pivoted = map[string]interface{}{xKey: x}
				byX[x] = pivoted
				order = append(order, x)
			}
			pivoted[label] = row[valueKey]
		}
	}

	out := make([]map[string]interface{}, 0, len(order))
	for _, x := range order {
		out = append(out, byX[x])
	}
	return out
}
```

- [ ] **Step 4: Correr los tests y verificar que pasan**

Run: `cd backend && go test ./internal/services/ -run 'TestProjectRows|TestConcatSources|TestPivotSources' -v`
Expected: PASS, los 6 tests.

- [ ] **Step 5: Commit**

```bash
cd backend && gofmt -w internal/services/artifact_shape.go internal/services/artifact_shape_test.go
git add backend/internal/services/artifact_shape.go backend/internal/services/artifact_shape_test.go
git commit -m "feat(artefactos): proyección, concatenación y pivote de las fuentes"
```

---

## Task 6: `RunArtifact` y la autorización

El corazón del plan. Los tests de autorización no son opcionales.

**Files:**
- Create: `backend/internal/services/artifact_service.go`
- Create: `backend/internal/services/artifact_service_test.go`

**Interfaces:**
- Consumes: `buildMonitorAliases`, `resolveQueryMonitor` (plan multi-monitor), `projectRows`, `concatSources`, `pivotSources` (Task 5)
- Produces:
  - `type ArtifactRun struct { Data []map[string]interface{}; RanAt time.Time }`
  - `var ErrArtifactNotFound error`
  - `func (s *ArtifactService) RunArtifact(ctx context.Context, art *models.ChatArtifact, userID primitive.ObjectID) (ArtifactRun, error)`
  - `func resolveArtifactMonitors(aliases map[string]*models.Monitor, sources []models.ArtifactSource) ([]*models.Monitor, error)`

- [ ] **Step 1: Escribir el test que falla**

`resolveArtifactMonitors` es la parte pura y testeable sin Mongo; ahí van los tests de autorización.

```go
package services

import (
	"testing"

	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestResolveArtifactMonitors_AliasValidos(t *testing.T) {
	monitors := []models.Monitor{
		{ID: primitive.NewObjectID(), Name: "Transacciones"},
		{ID: primitive.NewObjectID(), Name: "Alertas SWIFT"},
	}
	aliases := buildMonitorAliases(monitors)
	sources := []models.ArtifactSource{
		{Monitor: "transacciones"},
		{Monitor: "alertas_swift"},
	}

	got, err := resolveArtifactMonitors(aliases, sources)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if len(got) != 2 || got[0].ID != monitors[0].ID || got[1].ID != monitors[1].ID {
		t.Errorf("resolución incorrecta")
	}
}

// AUTORIZACIÓN: la allowlist de un artefacto es art.MonitorIDs. Un alias
// fuera de ella no resuelve, y por lo tanto no se ejecuta ninguna
// consulta. Ver Global Constraints.
func TestResolveArtifactMonitors_AliasFueraDeLaAllowlist(t *testing.T) {
	aliases := buildMonitorAliases([]models.Monitor{
		{ID: primitive.NewObjectID(), Name: "Transacciones"},
	})
	sources := []models.ArtifactSource{{Monitor: "clientes_secretos"}}

	if _, err := resolveArtifactMonitors(aliases, sources); err == nil {
		t.Fatal("un alias fuera de la allowlist debe fallar")
	}
}

// AUTORIZACIÓN: un ObjectID crudo nunca es una forma válida de nombrar un
// monitor, ni siquiera el del propio artefacto.
func TestResolveArtifactMonitors_ObjectIDCrudoNoResuelve(t *testing.T) {
	id := primitive.NewObjectID()
	aliases := buildMonitorAliases([]models.Monitor{{ID: id, Name: "Transacciones"}})
	sources := []models.ArtifactSource{{Monitor: id.Hex()}}

	if _, err := resolveArtifactMonitors(aliases, sources); err == nil {
		t.Fatal("un ObjectID crudo no debe resolver")
	}
}

// Renombrar un monitor cambia su alias y rompería un artefacto guardado.
// Con una sola fuente y un solo monitor, el respaldo posicional resuelve.
func TestResolveArtifactMonitors_RespaldoPosicionalTrasRenombrar(t *testing.T) {
	id := primitive.NewObjectID()
	// El monitor se llamaba "Transacciones" cuando se guardó el artefacto;
	// ahora se llama distinto, así que su alias cambió.
	aliases := buildMonitorAliases([]models.Monitor{{ID: id, Name: "Movimientos 2026"}})
	sources := []models.ArtifactSource{{Monitor: "transacciones"}}

	got, err := resolveArtifactMonitors(aliases, sources)
	if err != nil {
		t.Fatalf("con una sola fuente y un solo monitor debería resolver por posición: %v", err)
	}
	if got[0].ID != id {
		t.Error("resolvió al monitor equivocado")
	}
}

// Un chart multi-fuente pivotea, y entonces las series dejan de ser las
// YKeys del LLM para pasar a ser los labels de las fuentes. Sin esto el
// gráfico se renderiza vacío: yKeys apuntaría a "aggValue", una clave que
// el pivote ya no dejó en las filas.
func TestShape_ChartMultiFuenteDevuelveLosLabelsComoSeries(t *testing.T) {
	art := &models.ChatArtifact{
		Type: models.ChatArtifactChart,
		ChartSpec: &models.ChartSpec{
			ChartType: models.ChatChartBar,
			XKey:      "_id",
			YKeys:     []string{"aggValue"},
		},
	}
	results := [][]map[string]interface{}{
		{{"_id": "Caribe", "aggValue": 100}},
		{{"_id": "Caribe", "aggValue": 60}},
	}
	svc := &ArtifactService{}

	rows, series := svc.shape(art, results, []string{"Bancolombia", "Davivienda"})

	if len(series) != 2 || series[0] != "Bancolombia" || series[1] != "Davivienda" {
		t.Fatalf("series = %v, quiero los labels de las fuentes", series)
	}
	if rows[0]["Bancolombia"] != 100 || rows[0]["Davivienda"] != 60 {
		t.Errorf("la fila pivoteada no tiene una columna por fuente: %v", rows[0])
	}
	for _, key := range series {
		if _, hay := rows[0][key]; !hay {
			t.Errorf("la serie %q no existe en los datos — el gráfico saldría vacío", key)
		}
	}
}

// Con una sola fuente no hay pivote y las series siguen siendo las YKeys.
func TestShape_UnaSolaFuenteConservaLasYKeys(t *testing.T) {
	art := &models.ChatArtifact{
		Type:      models.ChatArtifactChart,
		ChartSpec: &models.ChartSpec{ChartType: models.ChatChartBar, XKey: "_id", YKeys: []string{"aggValue"}},
	}
	results := [][]map[string]interface{}{{{"_id": "Caribe", "aggValue": 100}}}
	svc := &ArtifactService{}

	_, series := svc.shape(art, results, []string{"Bancolombia"})
	if len(series) != 1 || series[0] != "aggValue" {
		t.Errorf("series = %v, quiero [aggValue]", series)
	}
}

// El respaldo posicional solo aplica cuando no hay ambigüedad. Con varios
// monitores, adivinar cuál es cuál podría consultar el equivocado.
func TestResolveArtifactMonitors_SinRespaldoPosicionalSiHayAmbiguedad(t *testing.T) {
	aliases := buildMonitorAliases([]models.Monitor{
		{ID: primitive.NewObjectID(), Name: "Movimientos 2026"},
		{ID: primitive.NewObjectID(), Name: "Alertas 2026"},
	})
	sources := []models.ArtifactSource{{Monitor: "transacciones"}}

	if _, err := resolveArtifactMonitors(aliases, sources); err == nil {
		t.Fatal("con varios monitores no se debe adivinar cuál era")
	}
}
```

- [ ] **Step 2: Correr el test y verificar que falla**

Run: `cd backend && go test ./internal/services/ -run TestResolveArtifactMonitors -v`
Expected: FAIL — función no definida.

- [ ] **Step 3: Implementar**

Crear `backend/internal/services/artifact_service.go`:

```go
package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ErrArtifactNotFound cubre tanto "no existe" como "no es tuyo" — nunca
// se distingue cuál de las dos, mismo criterio que
// ErrConversationNotFound (ver Global Constraints).
var ErrArtifactNotFound = errors.New("artefacto no encontrado")

// ErrArtifactNotRerunnable: el artefacto es una instantánea (legacy o
// custom) y no tiene consulta que volver a correr.
var ErrArtifactNotRerunnable = errors.New("este artefacto no se puede actualizar")

// ArtifactRun es el resultado de ejecutar un artefacto: los datos ya
// proyectados y (para un chart multi-fuente) pivoteados, más el momento
// de la corrida.
type ArtifactRun struct {
	Data []map[string]interface{}
	// Series son las claves que hay que graficar en las filas de Data.
	// Existe porque el pivote RENOMBRA las columnas: con varias fuentes
	// las filas quedan {_id: "Caribe", "Bancolombia": 100, ...}, y el
	// ChartSpec.YKeys que escribió el LLM sigue diciendo ["aggValue"] —
	// una clave que ya no está en los datos. Sin este campo, todo gráfico
	// multi-fuente se renderiza vacío.
	//
	// Con una sola fuente vale ChartSpec.YKeys tal cual; con varias, los
	// labels de las fuentes. Quien renderiza usa SIEMPRE Series y nunca
	// YKeys directo.
	Series []string
	RanAt  time.Time
}

type ArtifactService struct {
	artifactRepo *repository.ChatArtifactRepository
	monitorRepo  *repository.MonitorRepository
}

func NewArtifactService(artifactRepo *repository.ChatArtifactRepository, monitorRepo *repository.MonitorRepository) *ArtifactService {
	return &ArtifactService{artifactRepo: artifactRepo, monitorRepo: monitorRepo}
}

// resolveArtifactMonitors traduce el alias de cada source al monitor real.
//
// ESTA ES LA FRONTERA DE AUTORIZACIÓN de un artefacto: `aliases` se
// construye solo a partir de art.MonitorIDs, así que un alias que no está
// ahí no puede convertirse en una consulta. Nunca se acepta un ObjectID
// crudo.
//
// Respaldo posicional: el alias se deriva del NOMBRE del monitor, así que
// renombrarlo rompe un artefacto guardado. Cuando hay exactamente un
// monitor y una fuente, no hay ambigüedad posible y se resuelve por
// posición. Con varios, se falla: adivinar podría consultar el monitor
// equivocado, y eso es peor que no mostrar nada.
func resolveArtifactMonitors(aliases map[string]*models.Monitor, sources []models.ArtifactSource) ([]*models.Monitor, error) {
	out := make([]*models.Monitor, 0, len(sources))
	for i, src := range sources {
		monitor, err := resolveQueryMonitor(aliases, src.Monitor)
		if err != nil {
			if len(aliases) == 1 && len(sources) == 1 {
				for _, only := range aliases {
					out = append(out, only)
				}
				continue
			}
			return nil, fmt.Errorf("fuente %d: %w", i, err)
		}
		out = append(out, monitor)
	}
	return out, nil
}

// RunArtifact ejecuta las sources del artefacto y devuelve la tabla
// resultante. Es el ÚNICO camino por el que se obtienen datos de un
// artefacto: lo usan el render, la invocación y las exportaciones.
//
// NO persiste nada: solo el endpoint /run escribe la caché. Exportar
// re-ejecuta pero no muta el artefacto, para que bajar un Excel no le
// cambie la fecha a quien lo esté mirando en pantalla.
func (s *ArtifactService) RunArtifact(ctx context.Context, art *models.ChatArtifact, userID primitive.ObjectID) (ArtifactRun, error) {
	if art.UserID != userID {
		return ArtifactRun{}, ErrArtifactNotFound
	}
	if !art.IsRerunnable() {
		return ArtifactRun{}, ErrArtifactNotRerunnable
	}

	monitors := make([]models.Monitor, 0, len(art.MonitorIDs))
	for _, id := range art.MonitorIDs {
		monitor, err := s.monitorRepo.FindByID(ctx, id)
		if err != nil {
			continue // monitor borrado: se excluye de la allowlist
		}
		monitors = append(monitors, *monitor)
	}
	aliases := buildMonitorAliases(monitors)

	resolved, err := resolveArtifactMonitors(aliases, art.Sources)
	if err != nil {
		return ArtifactRun{}, err
	}

	results := make([][]map[string]interface{}, 0, len(art.Sources))
	labels := make([]string, 0, len(art.Sources))
	for i, src := range art.Sources {
		rows, err := s.runSource(ctx, resolved[i], src.Query)
		if err != nil {
			return ArtifactRun{}, fmt.Errorf("ejecutando la fuente %d: %w", i, err)
		}
		results = append(results, rows)
		label := src.Label
		if label == "" {
			label = src.Monitor
		}
		labels = append(labels, label)
	}

	rows, series := s.shape(art, results, labels)
	return ArtifactRun{Data: rows, Series: series, RanAt: time.Now()}, nil
}

// shape aplica la presentación y devuelve, junto a las filas, las claves
// que hay que graficar.
//
// Con una sola fuente los datos van tal cual y las series son las YKeys
// que escribió el LLM. Con varias, un chart PIVOTEA —una serie por
// fuente, porque concatenar daría una sola serie con las x repetidas— y
// entonces las series pasan a ser los labels de las fuentes: el pivote
// renombró las columnas y YKeys ya no apunta a nada.
//
// Una tabla con varias fuentes CONCATENA, con su columna discriminadora.
func (s *ArtifactService) shape(
	art *models.ChatArtifact,
	results [][]map[string]interface{},
	labels []string,
) ([]map[string]interface{}, []string) {
	var yKeys []string
	if art.ChartSpec != nil {
		yKeys = art.ChartSpec.YKeys
	}

	if len(results) == 1 {
		rows := results[0]
		if art.ChartSpec != nil {
			rows = projectRows(rows, art.ChartSpec.Columns)
		}
		return rows, yKeys
	}

	if art.Type == models.ChatArtifactChart && art.ChartSpec != nil {
		valueKey := ""
		if len(yKeys) > 0 {
			valueKey = yKeys[0]
		}
		// NO se proyecta después de pivotear: las columnas que el pivote
		// acaba de crear se llaman como los labels de las fuentes, y una
		// proyección escrita para las columnas originales las borraría
		// justo después de crearlas. Columns es solo para tablas.
		return pivotSources(results, labels, art.ChartSpec.XKey, valueKey), labels
	}

	rows := concatSources(results, labels)
	if art.ChartSpec != nil {
		rows = projectRows(rows, art.ChartSpec.Columns)
	}
	return rows, yKeys
}

// runSource ejecuta una consulta contra el monitor YA RESUELTO. No vuelve
// a mirar src.Query.Monitor: la autorización pasó en
// resolveArtifactMonitors y este método no debe poder saltearla.
func (s *ArtifactService) runSource(ctx context.Context, monitor *models.Monitor, query models.ChatQueryInput) ([]map[string]interface{}, error) {
	if query.Aggregate != nil {
		pipeline := buildDataAggregatePipeline(*query.Aggregate)
		results, err := s.monitorRepo.AggregateData(ctx, monitor.CollectionID, pipeline)
		if err != nil {
			return nil, fmt.Errorf("ejecutando agregación: %w", err)
		}
		return bsonToMaps(results), nil
	}

	filter := bson.M{}
	if query.ConditionGroup != nil {
		filter = BuildMongoFilter(*query.ConditionGroup)
	}
	limit := query.Limit
	if limit <= 0 || limit > chatQueryMaxLimit {
		limit = chatQueryMaxLimit
	}
	results, err := s.monitorRepo.QueryData(ctx, monitor.CollectionID, filter, int64(limit))
	if err != nil {
		return nil, fmt.Errorf("ejecutando consulta: %w", err)
	}
	return bsonToMaps(results), nil
}

func bsonToMaps(docs []bson.M) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(docs))
	for _, doc := range docs {
		out = append(out, map[string]interface{}(doc))
	}
	return out
}
```

- [ ] **Step 4: Correr los tests y verificar que pasan**

Run: `cd backend && go test ./internal/services/ -run TestResolveArtifactMonitors -v`
Expected: PASS, los 5 tests.

- [ ] **Step 5: Commit**

```bash
cd backend && gofmt -w internal/services/
git add backend/internal/services/artifact_service.go backend/internal/services/artifact_service_test.go
git commit -m "feat(artefactos): ejecutar las fuentes con la allowlist del artefacto"
```

---

## Task 7: Exportación a Excel

**Files:**
- Create: `backend/internal/services/artifact_xlsx.go`
- Create: `backend/internal/services/artifact_xlsx_test.go`

**Interfaces:**
- Produces: `func BuildArtifactXLSX(art *models.ChatArtifact, run ArtifactRun) ([]byte, error)`

- [ ] **Step 1: Escribir el test que falla**

```go
package services

import (
	"bytes"
	"testing"
	"time"

	"github.com/thureos/compliance/internal/models"
	"github.com/xuri/excelize/v2"
)

func artifactWithRows() (*models.ChatArtifact, ArtifactRun) {
	art := &models.ChatArtifact{
		Type:  models.ChatArtifactTable,
		Title: "Ventas por región",
		ChartSpec: &models.ChartSpec{
			Columns: []string{"region", "monto"},
			Labels:  map[string]string{"region": "Región", "monto": "Monto total"},
		},
	}
	run := ArtifactRun{
		RanAt: time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC),
		Data: []map[string]interface{}{
			{"region": "Caribe", "monto": 1500.5},
			{"region": "Andina", "monto": 2300.0},
		},
	}
	return art, run
}

func TestBuildArtifactXLSX_UsaLosRotulosComoCabecera(t *testing.T) {
	art, run := artifactWithRows()
	data, err := BuildArtifactXLSX(art, run)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("el archivo generado no es un xlsx válido: %v", err)
	}
	sheet := f.GetSheetName(0)

	// La cabecera muestra el rótulo, no la clave interna.
	if got, _ := f.GetCellValue(sheet, "A1"); got != "Región" {
		t.Errorf("A1 = %q, quiero %q", got, "Región")
	}
	if got, _ := f.GetCellValue(sheet, "B1"); got != "Monto total" {
		t.Errorf("B1 = %q, quiero %q", got, "Monto total")
	}
}

// Los montos tienen que entrar como NÚMEROS: si van como texto, Excel no
// los suma ni los ordena, que es lo primero que hace quien baja el
// archivo.
func TestBuildArtifactXLSX_LosNumerosSonNumeros(t *testing.T) {
	art, run := artifactWithRows()
	data, _ := BuildArtifactXLSX(art, run)
	f, _ := excelize.OpenReader(bytes.NewReader(data))
	sheet := f.GetSheetName(0)

	cellType, err := f.GetCellType(sheet, "B2")
	if err != nil {
		t.Fatalf("no pude leer el tipo de B2: %v", err)
	}
	if cellType == excelize.CellTypeSharedString || cellType == excelize.CellTypeInlineString {
		t.Errorf("B2 quedó como texto (%v); un monto debe ser numérico", cellType)
	}
}

func TestBuildArtifactXLSX_SinFilasNoFalla(t *testing.T) {
	art := &models.ChatArtifact{Type: models.ChatArtifactTable, Title: "Vacío"}
	if _, err := BuildArtifactXLSX(art, ArtifactRun{RanAt: time.Now()}); err != nil {
		t.Fatalf("un artefacto sin filas debe generar un archivo vacío, no un error: %v", err)
	}
}

func TestBuildArtifactXLSX_SinProyeccionUsaLasClavesDeLasFilas(t *testing.T) {
	art := &models.ChatArtifact{Type: models.ChatArtifactTable, Title: "T"}
	run := ArtifactRun{
		RanAt: time.Now(),
		Data:  []map[string]interface{}{{"b": 2, "a": 1}},
	}
	data, err := BuildArtifactXLSX(art, run)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	f, _ := excelize.OpenReader(bytes.NewReader(data))
	sheet := f.GetSheetName(0)
	// Las claves se ordenan para que el archivo sea reproducible: el
	// recorrido de un mapa en Go es aleatorio.
	if got, _ := f.GetCellValue(sheet, "A1"); got != "a" {
		t.Errorf("A1 = %q, quiero %q (columnas ordenadas)", got, "a")
	}
}
```

- [ ] **Step 2: Correr el test y verificar que falla**

Run: `cd backend && go test ./internal/services/ -run TestBuildArtifactXLSX -v`
Expected: FAIL — `undefined: BuildArtifactXLSX`.

- [ ] **Step 3: Implementar**

```go
package services

import (
	"bytes"
	"fmt"
	"sort"
	"time"

	"github.com/thureos/compliance/internal/models"
	"github.com/xuri/excelize/v2"
)

// artifactSheetName: Excel topea el nombre de hoja en 31 caracteres y
// prohíbe varios símbolos, así que se usa uno fijo en vez del título.
const artifactSheetName = "Datos"

// BuildArtifactXLSX arma el Excel de un artefacto ya ejecutado.
//
// Los números van como números (no como texto) porque lo primero que hace
// quien baja el archivo es sumarlos u ordenarlos, y una columna de texto
// no se deja. Las fechas van con formato de fecha, por lo mismo.
func BuildArtifactXLSX(art *models.ChatArtifact, run ArtifactRun) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	index, err := f.NewSheet(artifactSheetName)
	if err != nil {
		return nil, fmt.Errorf("creando la hoja: %w", err)
	}
	f.SetActiveSheet(index)
	_ = f.DeleteSheet("Sheet1")

	columns := artifactColumns(art, run.Data)

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	})
	if err != nil {
		return nil, fmt.Errorf("creando el estilo de cabecera: %w", err)
	}
	dateStyle, err := f.NewStyle(&excelize.Style{NumFmt: 22}) // d/m/yyyy h:mm
	if err != nil {
		return nil, fmt.Errorf("creando el estilo de fecha: %w", err)
	}

	// Cabecera: el rótulo de presentación si lo hay, si no la clave.
	for i, col := range columns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(artifactSheetName, cell, artifactColumnLabel(art, col))
		_ = f.SetCellStyle(artifactSheetName, cell, cell, headerStyle)
		_ = f.SetColWidth(artifactSheetName, colLetter(i), colLetter(i), 18)
	}

	for r, row := range run.Data {
		for c, col := range columns {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			value := row[col]
			if t, ok := value.(time.Time); ok {
				_ = f.SetCellValue(artifactSheetName, cell, t)
				_ = f.SetCellStyle(artifactSheetName, cell, cell, dateStyle)
				continue
			}
			// excelize infiere el tipo del valor de Go: un float64 entra
			// como número, un string como texto. Por eso NO se convierte
			// nada a string antes de escribir.
			_ = f.SetCellValue(artifactSheetName, cell, value)
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("serializando el xlsx: %w", err)
	}
	return buf.Bytes(), nil
}

// artifactColumns decide qué columnas van y en qué orden: la proyección
// si el artefacto la declara, si no la unión de las claves de todas las
// filas, ORDENADA — el recorrido de un mapa en Go es aleatorio y sin
// ordenar el archivo saldría distinto en cada descarga.
func artifactColumns(art *models.ChatArtifact, rows []map[string]interface{}) []string {
	if art.ChartSpec != nil && len(art.ChartSpec.Columns) > 0 {
		return art.ChartSpec.Columns
	}
	seen := map[string]bool{}
	columns := []string{}
	for _, row := range rows {
		for k := range row {
			if !seen[k] {
				seen[k] = true
				columns = append(columns, k)
			}
		}
	}
	sort.Strings(columns)
	return columns
}

func artifactColumnLabel(art *models.ChatArtifact, column string) string {
	if art.ChartSpec != nil {
		if label, ok := art.ChartSpec.Labels[column]; ok && label != "" {
			return label
		}
	}
	return column
}

func colLetter(index int) string {
	name, _ := excelize.ColumnNumberToName(index + 1)
	return name
}
```

- [ ] **Step 4: Correr los tests y verificar que pasan**

Run: `cd backend && go test ./internal/services/ -run TestBuildArtifactXLSX -v`
Expected: PASS, los 4 tests.

- [ ] **Step 5: Commit**

```bash
cd backend && gofmt -w internal/services/artifact_xlsx.go internal/services/artifact_xlsx_test.go
git add backend/internal/services/artifact_xlsx.go backend/internal/services/artifact_xlsx_test.go
git commit -m "feat(artefactos): exportar a Excel con excelize"
```

---

## Task 8: Persistir el artefacto al responder y actualizar el prompt

**Files:**
- Modify: `backend/internal/services/chat_service.go` (`Ask`, `chatSystemPromptTemplate`)

**Interfaces:**
- Consumes: `ChatArtifactRepository.Create` (Task 3), `buildMonitorAliases`

- [ ] **Step 1: Actualizar el bloque de artefacto del prompt**

En `chatSystemPromptTemplate`, reemplazar la sección del `<artifact>`:

```
Si la respuesta se presta a visualizarse (series de tiempo, comparaciones,
rankings, distribuciones), agregá al FINAL de tu respuesta un bloque
delimitado exactamente así, con un JSON válido dentro:

<artifact>
{"type": "chart", "title": "Monto por región",
 "sources": [{"monitor": "transacciones", "label": "Bancolombia",
              "query": {"aggregate": {"field": "monto", "function": "sum", "groupBy": "region"}}}],
 "chartSpec": {"chartType": "bar", "xKey": "_id", "yKeys": ["aggValue"],
               "labels": {"_id": "Región", "aggValue": "Monto total"}}}
</artifact>

IMPORTANTE sobre "sources": el artefacto declara las CONSULTAS que lo
alimentan, no sus datos. Poné en "sources" las mismas consultas que
acabás de ejecutar con la herramienta, para que el artefacto muestre lo
que describiste. Así se puede volver a abrir más adelante con datos
actualizados.

Los nombres de columna que devuelve una agregación son SIEMPRE "_id" (el
valor por el que se agrupó), "aggValue" (el resultado de la función) y
"count". Usá esos nombres en "xKey" y "yKeys", y poné en "labels" el
nombre legible de cada uno — sin "labels" los ejes salen rotulados "_id"
y "aggValue".

Para una tabla sobre filas crudas (conditionGroup), declará en
"chartSpec.columns" qué columnas mostrar: sin eso se vuelca el documento
entero, con campos internos incluidos.

<artifact>
{"type": "table", "title": "Operaciones sobre 50M",
 "sources": [{"monitor": "transacciones",
              "query": {"conditionGroup": {"logic": "AND", "conditions": [
                  {"field": "monto", "operator": "gt", "value": 50000000}]}, "limit": 200}}],
 "chartSpec": {"columns": ["fecha", "monto", "beneficiario"],
               "labels": {"fecha": "Fecha", "monto": "Monto", "beneficiario": "Beneficiario"}}}
</artifact>

Si un gráfico compara VARIOS monitores, poné una source por monitor con
su "label", y un "xKey" común: se dibuja una serie por monitor.

Si necesitás algo que un gráfico de barra/línea/torta no puede expresar,
usá "type": "custom" con "code": HTML/JS autocontenido que renderice la
visualización — ese código corre aislado en un iframe sin acceso a la
sesión ni a la red, así que tiene que traer sus propios datos embebidos
en el propio "code" (no puede hacer fetch a nada). Un artefacto custom no
se puede actualizar después, así que usalo solo cuando haga falta.

Si no hace falta artefacto, no incluyas el bloque <artifact>. Respondé
siempre en español.
```

- [ ] **Step 2: Persistir el artefacto en `Ask`**

`ChatService` recibe el repositorio de artefactos. Agregar el campo y el parámetro al constructor, y actualizar la llamada (`grep -rn "NewChatService" backend/`).

Reemplazar el bloque final de `Ask`, donde hoy se arma `assistantMsg`:

```go
	assistantMsg := models.ChatMessage{
		ConversationID: conversationID,
		Role:           models.ChatRoleAssistant,
		Content:        text,
	}
	if err := s.chatRepo.CreateMessage(ctx, &assistantMsg); err != nil {
		return models.ChatMessage{}, fmt.Errorf("guardando respuesta: %w", err)
	}

	// El artefacto se persiste como entidad propia y el mensaje lo
	// referencia por id. MonitorIDs se fija acá, a partir de los
	// monitores de ESTA conversación: es la allowlist con la que el
	// artefacto se va a re-ejecutar después, quizá fuera de todo hilo.
	if artifact != nil {
		artifact.UserID = userID
		artifact.ConversationID = conversationID
		artifact.MessageID = assistantMsg.ID
		artifact.MonitorIDs = artifactMonitorIDs(aliases, artifact.Sources)
		if err := s.artifactRepo.Create(ctx, artifact); err != nil {
			// Un artefacto que no se pudo guardar no debe hacer perder la
			// respuesta de texto, que es lo que el usuario está esperando.
			artifact = nil
		} else {
			assistantMsg.ArtifactID = &artifact.ID
			_ = s.chatRepo.SetMessageArtifact(ctx, assistantMsg.ID, artifact.ID)
		}
	}
	assistantMsg.Artifact = artifact
	_ = s.chatRepo.TouchConversation(ctx, conversationID)
	return assistantMsg, nil
```

Agregar en `artifact_service.go`:

```go
// artifactMonitorIDs traduce los alias de las sources a los ObjectID de
// los monitores, para fijar la allowlist del artefacto al crearlo.
// Un alias que no resuelve se omite: no se puede fabricar acceso a un
// monitor que la conversación no tenía.
func artifactMonitorIDs(aliases map[string]*models.Monitor, sources []models.ArtifactSource) []primitive.ObjectID {
	seen := map[primitive.ObjectID]bool{}
	ids := []primitive.ObjectID{}
	for _, src := range sources {
		monitor, err := resolveQueryMonitor(aliases, src.Monitor)
		if err != nil {
			continue
		}
		if !seen[monitor.ID] {
			seen[monitor.ID] = true
			ids = append(ids, monitor.ID)
		}
	}
	return ids
}
```

Y en `chat_repo.go`:

```go
// SetMessageArtifact enlaza un mensaje ya persistido con su artefacto.
func (r *ChatRepository) SetMessageArtifact(ctx context.Context, messageID, artifactID primitive.ObjectID) error {
	_, err := r.messages.UpdateByID(ctx, messageID, bson.M{"$set": bson.M{"artifact_id": artifactID}})
	if err != nil {
		return fmt.Errorf("enlazando artefacto al mensaje %s: %w", messageID.Hex(), err)
	}
	return nil
}
```

- [ ] **Step 3: Compilar y correr los tests**

Run: `cd backend && go build ./... && go test ./internal/... 2>&1 | tail -20`
Expected: compila; los tests pasan.

- [ ] **Step 4: Commit**

```bash
cd backend && gofmt -w internal/
git add backend/internal/services/chat_service.go backend/internal/services/artifact_service.go backend/internal/repository/chat_repo.go
git commit -m "feat(artefactos): persistir el artefacto con la allowlist de su conversación"
```

---

## Task 9: Handlers y rutas

**Files:**
- Create: `backend/internal/handlers/artifact_handler.go`
- Modify: `backend/internal/router/router.go`

**Interfaces:**
- Produces: `Save`, `Unsave`, `List`, `Run`, `ExportXLSX`, `Delete`

- [ ] **Step 1: Crear el handler**

```go
package handlers

import (
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/thureos/compliance/internal/repository"
	"github.com/thureos/compliance/internal/services"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ArtifactHandler struct {
	artifactRepo    *repository.ChatArtifactRepository
	artifactService *services.ArtifactService
}

func NewArtifactHandler(artifactRepo *repository.ChatArtifactRepository, artifactService *services.ArtifactService) *ArtifactHandler {
	return &ArtifactHandler{artifactRepo: artifactRepo, artifactService: artifactService}
}

// load resuelve el artefacto del path y verifica propiedad. Devuelve
// siempre 404 cuando no es del usuario: nunca se distingue "no existe" de
// "no es tuyo" (ver Global Constraints).
//
// El tercer valor es un OK, no un error, y es importante que así sea:
// fiber.Ctx.JSON() devuelve nil cuando serializa bien — escribe la
// respuesta, no produce un error. Devolver su resultado como "el error"
// daría nil en todos los caminos de fallo, el llamador seguiría de largo
// y desreferenciaría un artefacto nil. Cuando ok es false la respuesta de
// error YA fue escrita: el handler solo tiene que devolver nil.
func (h *ArtifactHandler) load(c *fiber.Ctx) (*models.ChatArtifact, primitive.ObjectID, bool) {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
		return nil, primitive.NilObjectID, false
	}
	userID, err := primitive.ObjectIDFromHex(c.Locals("userId").(string))
	if err != nil {
		_ = c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "usuario inválido"})
		return nil, primitive.NilObjectID, false
	}
	art, err := h.artifactRepo.GetByID(c.Context(), id)
	if err != nil || art.UserID != userID {
		_ = c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "artefacto no encontrado"})
		return nil, primitive.NilObjectID, false
	}
	return art, userID, true
}

func (h *ArtifactHandler) Save(c *fiber.Ctx) error {
	art, _, ok := h.load(c)
	if !ok {
		return nil // load ya escribió la respuesta de error
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := c.BodyParser(&body); err != nil || body.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name es obligatorio"})
	}
	if err := h.artifactRepo.SetSaved(c.Context(), art.ID, true, body.Name); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	art.Saved, art.SavedName = true, body.Name
	return c.JSON(art)
}

func (h *ArtifactHandler) Unsave(c *fiber.Ctx) error {
	art, _, ok := h.load(c)
	if !ok {
		return nil // load ya escribió la respuesta de error
	}
	if err := h.artifactRepo.SetSaved(c.Context(), art.ID, false, ""); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "artefacto quitado de la biblioteca"})
}

func (h *ArtifactHandler) List(c *fiber.Ctx) error {
	userID, err := primitive.ObjectIDFromHex(c.Locals("userId").(string))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "usuario inválido"})
	}
	arts, err := h.artifactRepo.ListSavedByUser(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(arts)
}

// Run re-ejecuta el artefacto y persiste la caché. Es el ÚNICO endpoint
// que escribe cached_data/ran_at: exportar re-ejecuta pero no muta.
func (h *ArtifactHandler) Run(c *fiber.Ctx) error {
	art, userID, ok := h.load(c)
	if !ok {
		return nil // load ya escribió la respuesta de error
	}
	run, err := h.artifactService.RunArtifact(c.Context(), art, userID)
	if err != nil {
		if errors.Is(err, services.ErrArtifactNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "artefacto no encontrado"})
		}
		if errors.Is(err, services.ErrArtifactNotRerunnable) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	_ = h.artifactRepo.SetCache(c.Context(), art.ID, run.Data, run.RanAt)
	// series viaja al frontend porque el pivote renombra las columnas:
	// sin ella, un gráfico multi-fuente no sabría qué claves dibujar.
	return c.JSON(fiber.Map{"data": run.Data, "series": run.Series, "ranAt": run.RanAt})
}

func (h *ArtifactHandler) ExportXLSX(c *fiber.Ctx) error {
	art, userID, ok := h.load(c)
	if !ok {
		return nil // load ya escribió la respuesta de error
	}
	run, err := h.artifactService.RunArtifact(c.Context(), art, userID)
	if err != nil {
		// Una instantánea no se re-ejecuta, pero sí se puede exportar con
		// los datos que ya tiene guardados.
		if errors.Is(err, services.ErrArtifactNotRerunnable) && len(art.CachedData) > 0 {
			run = services.ArtifactRun{Data: art.CachedData, RanAt: art.RanAt}
		} else {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}
	data, err := services.BuildArtifactXLSX(art, run)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.xlsx"`, art.Title))
	return c.Send(data)
}

func (h *ArtifactHandler) Delete(c *fiber.Ctx) error {
	art, _, ok := h.load(c)
	if !ok {
		return nil // load ya escribió la respuesta de error
	}
	if err := h.artifactRepo.Delete(c.Context(), art.ID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "artefacto borrado"})
}
```

Agregar el import de `models`.

> `load` centraliza las tres validaciones (id, usuario, propiedad) y escribe la respuesta de error él mismo. Los cinco handlers que lo usan comparten el idioma `if !ok { return nil }`.

- [ ] **Step 2: Registrar las rutas**

En `router.go`, después del grupo `chat`:

```go
	// Artefactos del chatbot: privados del usuario que los generó, misma
	// vara que las conversaciones.
	artifacts := protected.Group("/chat/artifacts")
	artifacts.Get("/", h.Artifact.List)
	artifacts.Post("/:id/save", h.Artifact.Save)
	artifacts.Delete("/:id/save", h.Artifact.Unsave)
	artifacts.Post("/:id/run", h.Artifact.Run)
	artifacts.Get("/:id/export.xlsx", h.Artifact.ExportXLSX)
	artifacts.Delete("/:id", h.Artifact.Delete)
```

Agregar `Artifact *ArtifactHandler` al struct de handlers y construirlo donde se arman los demás.

- [ ] **Step 3: Compilar y correr los tests**

Run: `cd backend && go build ./... && go test ./... 2>&1 | tail -20`
Expected: todo verde.

- [ ] **Step 4: Commit**

```bash
cd backend && gofmt -w internal/ && golangci-lint run
git add backend/internal/handlers/artifact_handler.go backend/internal/router/router.go backend/cmd/server/main.go
git commit -m "feat(artefactos): endpoints de biblioteca, ejecución y exportación"
```

---

## Task 10: Frontend — cliente de API

**Files:**
- Create: `frontend/src/lib/api/artifacts.ts`
- Modify: `frontend/src/lib/api/chat.ts` (tipos)

- [ ] **Step 1: Actualizar los tipos en `chat.ts`**

```ts
export interface ChartSpec {
  chartType?: "bar" | "line" | "pie";
  data?: Record<string, unknown>[];
  xKey?: string;
  yKeys?: string[];
  /** Proyección: qué columnas mostrar y en qué orden. Vacío = todas. */
  columns?: string[];
  /** Nombre de presentación por columna. No toca los datos. */
  labels?: Record<string, string>;
}

export interface ArtifactSource {
  monitor: string;
  label?: string;
}

export interface ChatArtifact {
  id?: string;
  type: ChatArtifactType;
  title: string;
  chartSpec?: ChartSpec;
  code?: string;
  sources?: ArtifactSource[];
  monitorIds?: string[];
  saved?: boolean;
  savedName?: string;
  cachedData?: Record<string, unknown>[];
  ranAt?: string;
}
```

- [ ] **Step 2: Crear el cliente de artefactos**

```ts
import { api } from "./client";
import type { ChatArtifact } from "./chat";

export interface ArtifactRun {
  data: Record<string, unknown>[];
  /**
   * Claves a graficar en `data`. Con varias fuentes el backend pivotea y
   * renombra las columnas con el label de cada fuente, así que
   * chartSpec.yKeys deja de apuntar a nada: hay que usar SIEMPRE esto.
   */
  series: string[];
  ranAt: string;
}

/**
 * Un artefacto es re-ejecutable si declara las consultas que lo
 * alimentan. Sin sources es una instantánea: legacy o custom.
 */
export function isRerunnable(artifact: ChatArtifact): boolean {
  return (artifact.sources?.length ?? 0) > 0;
}

export const artifactsApi = {
  list: () => api.get<ChatArtifact[]>("/chat/artifacts"),

  save: (id: string, name: string) =>
    api.post<ChatArtifact>(`/chat/artifacts/${id}/save`, { name }),

  unsave: (id: string) =>
    api.delete<{ message: string }>(`/chat/artifacts/${id}/save`),

  run: (id: string) => api.post<ArtifactRun>(`/chat/artifacts/${id}/run`, {}),

  remove: (id: string) =>
    api.delete<{ message: string }>(`/chat/artifacts/${id}`),

  /** URL del Excel — se abre directo, no pasa por fetch. */
  xlsxUrl: (id: string) =>
    `${process.env.NEXT_PUBLIC_API_URL}/chat/artifacts/${id}/export.xlsx`,
};
```

> Si `api.get` inyecta el JWT en un header, `xlsxUrl` abierto con `window.open` no lo lleva. Verificar cómo autentica `lib/api/client.ts`: si usa header, la descarga tiene que hacerse con `fetch` + `blob` en vez de `window.open`. La Task 12 lo resuelve con `fetch`, que funciona en los dos casos.

- [ ] **Step 3: Verificar tipos**

Run: `cd frontend && npx tsc --noEmit`
Expected: errores solo en los componentes que se actualizan en las tareas siguientes.

- [ ] **Step 4: No commitear todavía**

---

## Task 11: Frontend — documento HTML e informe PDF

**Files:**
- Create: `frontend/src/lib/artifact-report.ts`
- Modify: `frontend/src/lib/chat-export.ts`

**Interfaces:**
- Produces:
  - `export function buildArtifactHTML(artifact, rows, ranAt, monitorNames): string`
  - `export function downloadArtifactHTML(...)`, `export function openArtifactHTML(...)`
  - `export async function exportArtifactPDF(...)`

- [ ] **Step 1: Crear el módulo del documento**

`buildArtifactHTML` arma un `.html` autónomo —estilos embebidos, sin recursos externos— con cabecera de marca, título, fecha de corrida, chips de los monitores consultados y la tabla formateada. Si el artefacto es un `chart`, embebe el PNG del SVG como data URI usando el `exportChartAsPNG` que ya existe (extraído a una función que devuelve el data URI en vez de descargarlo).

Puntos que el implementador debe respetar:

- **Colores por token, resueltos en tiempo de generación.** El documento sale del navegador y no tiene acceso a las custom properties de la app: hay que resolverlas con `getComputedStyle` como ya hace `resolveChartColors` en `chat-export.ts` y `readReportColors` en `pdf-tokens.ts`.
- **Escapado de HTML obligatorio.** Los valores vienen de datos subidos por usuarios. Toda interpolación pasa por una función `escapeHTML` que reemplaza `&`, `<`, `>`, `"` y `'`. Este documento SÍ es HTML —no hay alternativa, es el formato pedido— pero se genera concatenando texto escapado, nunca insertando marcado que venga de los datos.
- **Tipografía de marca:** Inter para lectura, JetBrains Mono para montos, identificadores y timestamps. Se declaran como `font-family` con fallback real, sin `@import` de fuentes externas.
- **Formato de celdas:** reutiliza `formatValue`/`classifyValue` de `@/lib/format-value` si el plan de formato rico ya se ejecutó; si no, se crea ahí ese módulo primero.

- [ ] **Step 2: Extraer el data URI del gráfico**

En `chat-export.ts`, refactorizar `exportChartAsPNG` para que se apoye en una función nueva que devuelve el PNG como data URI, y que la exportación a archivo la use:

```ts
/** Serializa el primer <svg> de containerId a un data URI PNG @2x. */
export function chartToPNGDataURL(containerId: string): Promise<string | null>
```

`exportChartAsPNG` pasa a ser `chartToPNGDataURL` + descarga. Así el PDF y el HTML pueden embeber la imagen sin duplicar la lógica de serialización y resolución de colores.

- [ ] **Step 3: Crear el informe PDF**

`exportArtifactPDF` sigue el patrón de `red-flag-report.ts`: `readReportColors` con los tokens de marca, cabecera, título, fecha de corrida, chips de monitores, `autoTable` con las columnas proyectadas y sus rótulos, y —si es un `chart`— la imagen embebida antes de la tabla.

Límites a respetar, tomados de `red-flag-report.ts`: máximo 8 columnas (más es ilegible en A4 vertical) y margen de 14.

- [ ] **Step 4: Verificar tipos, lint y build**

Run: `cd frontend && npx tsc --noEmit && npm run lint && npm run build`
Expected: sin errores.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/artifact-report.ts frontend/src/lib/chat-export.ts frontend/src/lib/api/artifacts.ts frontend/src/lib/api/chat.ts
git commit -m "feat(artefactos): documento HTML e informe PDF con marca"
```

---

## Task 12: Frontend — menú de exportación

**Files:**
- Create: `frontend/src/components/chat/export-menu.tsx`

- [ ] **Step 1: Crear el componente**

Un `DropdownMenu` (ya está `@radix-ui/react-dropdown-menu`) con las opciones aplicables **según el tipo**:

| Tipo | Opciones |
|---|---|
| `chart` | PNG, CSV, Excel, PDF, HTML |
| `table` | CSV, Excel, PDF, HTML |
| `custom` | HTML, Imprimir |

La regla se implementa como una función `availableFormats(artifact)` para que la tabla de arriba viva en un solo lugar y no repartida por el JSX.

**El caso `custom` se dice, no se esconde:** el menú muestra por qué no están PDF ni Excel («un artefacto interactivo no se puede convertir a PDF ni a Excel»), en vez de simplemente omitir las opciones y dejar al usuario buscándolas.

La descarga del Excel va por `fetch` con las mismas credenciales que usa `lib/api/client.ts`, y después `URL.createObjectURL` + click en un `<a>` — no con `window.open`, que no llevaría el JWT si la API autentica por header.

- [ ] **Step 2: Verificar tipos, lint y build**

Run: `cd frontend && npx tsc --noEmit && npm run lint && npm run build`

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/chat/export-menu.tsx
git commit -m "feat(artefactos): menú de exportación por tipo de artefacto"
```

---

## Task 13: Frontend — canvas con actualizar y exportar

**Files:**
- Modify: `frontend/src/components/chat/artifact-canvas.tsx`

- [ ] **Step 1: Estado de ejecución**

`ArtifactCanvas` pasa a manejar los datos que muestra, en vez de leer `artifact.chartSpec.data` directo:

```tsx
const [rows, setRows] = useState(artifact.cachedData ?? artifact.chartSpec?.data ?? []);
const [ranAt, setRanAt] = useState(artifact.ranAt);
const [running, setRunning] = useState(false);
const [staleReason, setStaleReason] = useState<string | null>(null);
```

Al montar un artefacto re-ejecutable, se dispara `artifactsApi.run(id)`: se pinta la caché de inmediato y se reemplaza al llegar. Si falla, **queda la caché con un aviso explícito de que son datos de tal fecha** — nunca una pantalla vacía.

- [ ] **Step 2: Cabecera**

- Botón «Actualizar» con el ícono `RefreshCw`, **deshabilitado con el motivo visible** cuando `!isRerunnable(artifact)`: «este artefacto guarda una instantánea y no se puede actualizar».
- Fecha de la última corrida junto al título, en JetBrains Mono.
- Botón «Guardar» que pide nombre y llama `artifactsApi.save`; si ya está guardado, muestra el nombre y permite quitarlo.
- `<ExportMenu>` en lugar del botón de descarga único actual.

**Las series a graficar salen de `run.series`, nunca de `spec.yKeys`.** Con varias fuentes el backend pivotea y renombra las columnas con el label de cada fuente; `yKeys` sigue diciendo `["aggValue"]`, que ya no existe en las filas. El estado arranca en `artifact.chartSpec?.yKeys ?? []` y se reemplaza por `run.series` en cuanto llega la ejecución.

- [ ] **Step 3: Aplicar rótulos al renderizar**

`ChartArtifact` y `TableArtifact` reciben `labels` y lo usan para los nombres de eje, la leyenda y las cabeceras — **sin tocar `xKey`/`yKeys`**, que siguen refiriéndose a las claves reales:

```tsx
const label = (key: string) => artifact.chartSpec?.labels?.[key] ?? key;
```

En Recharts, `<XAxis dataKey={spec.xKey} />` no cambia; lo que cambia es el `name` de cada `<Bar>`/`<Line>` y el `<Tooltip formatter>`.

- [ ] **Step 4: Verificar tipos, lint y build**

Run: `cd frontend && npx tsc --noEmit && npm run lint && npm run build`

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/chat/artifact-canvas.tsx
git commit -m "feat(artefactos): actualizar, guardar y exportar desde el canvas"
```

---

## Task 14: Frontend — biblioteca de artefactos

**Files:**
- Create: `frontend/src/components/chat/artifact-library.tsx`
- Modify: `frontend/src/app/(dashboard)/chatbot/page.tsx`

- [ ] **Step 1: Crear el panel**

Lista de `artifactsApi.list()` con, por fila: el nombre guardado, un ícono por tipo, chips de los monitores que consulta y la fecha de la última corrida en mono. Al hacer clic, el artefacto se abre en el canvas (que lo ejecuta contra datos actuales). Cada fila tiene borrar, con confirmación en línea siguiendo el patrón que ya usa la lista de conversaciones (`deletingId` + Sí/No).

Estado vacío con texto: «Todavía no guardaste ningún artefacto. Cuando el asistente genere un gráfico o una tabla, guardalo para volver a abrirlo cuando quieras con datos actualizados.»

- [ ] **Step 2: Integrar en la página**

Pestañas en el sidebar: «Conversaciones» / «Artefactos». Abrir un artefacto de la biblioteca lo muestra en el canvas sin necesidad de una conversación activa — es el punto del pedido «invocar directamente».

- [ ] **Step 3: Verificar tipos, lint y build**

Run: `cd frontend && npx tsc --noEmit && npm run lint && npm run build`

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/chat/artifact-library.tsx "frontend/src/app/(dashboard)/chatbot/page.tsx"
git commit -m "feat(artefactos): biblioteca de artefactos guardados"
```

---

## Task 15: Verificación de extremo a extremo

- [ ] **Step 1: Levantar todo**

Run: `docker compose up -d && (cd backend && go run cmd/server/main.go &) && cd frontend && npm run dev`

- [ ] **Step 2: Compatibilidad con artefactos viejos**

Abrir una conversación anterior a este cambio que tenga un artefacto. Debe renderizarse con sus datos guardados, y «Actualizar» debe estar deshabilitado con el motivo visible.

- [ ] **Step 3: Ciclo completo**

Pedir un gráfico. Guardarlo con nombre. Ir a la pestaña Artefactos, abrirlo: debe re-ejecutar y mostrar la fecha de corrida.

- [ ] **Step 4: Datos frescos de verdad**

Subir datos nuevos al monitor. Volver a abrir el artefacto guardado: los números **deben cambiar**. Esta es la prueba de que el contrato de `sources` funciona; si no cambian, el artefacto está sirviendo la caché y hay un bug.

- [ ] **Step 5: Multi-fuente**

En una conversación con dos monitores, pedir un gráfico comparativo. Debe salir **una serie por monitor** (dos barras por categoría), no una sola serie con las x repetidas.

- [ ] **Step 6: Exportaciones**

Bajar el mismo artefacto en los cinco formatos. Verificar: en el Excel, que los montos se puedan **sumar** con una fórmula (si no, entraron como texto); en el PDF y el HTML, que se vean los rótulos legibles y no `_id`/`aggValue`.

- [ ] **Step 7: Exportar no muta**

Con el artefacto abierto en una pestaña mostrando su `ranAt`, bajar el Excel desde otra. Recargar la primera: **`ranAt` no debe haber cambiado**.

- [ ] **Step 8: Autorización**

Con el token de un usuario A, pedir `POST /chat/artifacts/<id de B>/run`. Expected: `404`, indistinguible de un id inexistente.

- [ ] **Step 9: Suite completa**

Run: `cd backend && go test ./... && golangci-lint run && cd ../frontend && npm run test:run && npm run lint && npm run build`
Expected: todo verde.

---

## Auto-repaso del plan

**Cobertura del spec:**

| Sección del spec | Tarea |
|---|---|
| Prerrequisito: mover `ChatQueryInput` a `models` | 1 |
| `ArtifactSource`, entidad `ChatArtifact`, `LegacyChatArtifact` | 2 |
| Proyección y rótulos (`Columns`, `Labels`) | 2 (modelo), 5 (proyección), 7/11/13 (rótulos) |
| Varias fuentes: `table` concatena, `chart` pivotea | 5 |
| `sources` vacío ⇒ instantánea | 2 (`IsRerunnable`) |
| Colección `chat_artifacts`, índices, borrado en cascada | 3 |
| `RunArtifact` + autorización | 6 |
| Cuándo se escribe la caché | 6 (no persiste), 9 (`/run` sí) |
| Monitor renombrado → respaldo posicional | 6 |
| API (6 endpoints) | 9 |
| Exportación XLSX | 7, 9 |
| Exportación PDF / HTML / CSV / PNG | 11, 12 |
| Cambio en `ParseArtifact` | 4 |
| Prompt con `sources`, nombres de columna, multi-fuente | 8 |
| Biblioteca e invocación | 13, 14 |
| Manejo de errores | 6, 9, 13 |
| Formato rico (markdown, tabla) | **plan aparte**: `2026-09-10-chat-formato-rico.md` |

**Consistencia de tipos:** `ArtifactRun` se define en la Task 6 y se consume con esa forma en 7 (`BuildArtifactXLSX`) y 9 (handlers). `models.ChatQueryInput` se define en la Task 1 y lo embebe `ArtifactSource` en la 2. `IsRerunnable()` (Go, Task 2) tiene su espejo `isRerunnable()` (TS, Task 10) con la misma regla.

**Nota sobre las tareas 11-14:** son las únicas descritas por requisitos en vez de por código completo. El motivo es que dependen de detalles de `lib/api/client.ts` (cómo inyecta el JWT) y de si el plan de formato rico ya corrió — dos cosas que el implementador tiene delante y yo no puedo fijar sin arriesgar código que no compile. Cada una lista los puntos no negociables (escapado de HTML, tokens de marca, `fetch` en vez de `window.open`, motivo visible en vez de opción escondida).
