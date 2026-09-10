# Chat multi-monitor — Plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Que una conversación del Analista IA consulte varios monitores a la vez y pueda correlacionar entre ellos en una misma respuesta.

**Architecture:** `ChatConversation.MonitorID` pasa a `MonitorIDs []ObjectID`. La herramienta `query_monitor_data` gana un campo `monitor` que el LLM llena con un **alias corto** derivado del nombre del monitor; `executeQuery` resuelve ese alias contra una allowlist construida a partir de `conv.MonitorIDs` y nunca ejecuta una consulta con un alias desconocido. El system prompt emite un bloque de schema por monitor. La lista de conversaciones pasa de estar scopeada por monitor a estarlo por usuario.

**Tech Stack:** Go 1.26, Fiber v2, MongoDB (driver oficial), `golang.org/x/text` (ya en el módulo como indirecta), Next.js 16 + React 19, TypeScript.

**Spec:** `docs/superpowers/specs/2026-09-10-chat-multimonitor-design.md`

## Global Constraints

- **Autorización, invariante central:** `executeQuery` resuelve el alias del monitor **exclusivamente** contra el mapa construido a partir de `conv.MonitorIDs`. Un alias desconocido devuelve error de herramienta **sin ejecutar consulta**. Nunca se acepta un ObjectID crudo en el campo `monitor`. Motivo: los datos que el LLM lee son archivos subidos por usuarios y pueden contener inyecciones de prompt.
- **Privacidad de conversaciones:** `conv.UserID != userID` se trata como `ErrConversationNotFound`, indistinguible de "no existe". Nunca se filtra cuál de las dos ocurrió.
- **Cota de monitores por conversación:** máximo **6**.
- **Cotas escaladas:** iteraciones de herramienta `5 + 3×len(MonitorIDs)`, tope `20`. Timeout de `Ask`: `60s + 30s×len(MonitorIDs)`, tope `240s`.
- **Sin migración de datos.** Las conversaciones existentes traen `monitor_id` escalar y se normalizan al leer.
- **Errores envueltos con contexto:** `fmt.Errorf("operación: %w", err)`.
- **Frontend:** ningún componente escribe un color literal; se usan utilidades de token o `@/lib/semantic-colors`.
- **Idioma:** comentarios y mensajes de error orientados al usuario, en español.

---

## Estructura de archivos

**Backend — nuevos:**
- `backend/internal/services/chat_monitors.go` — derivación de alias y construcción de la allowlist. Función pura, sin dependencias de Mongo.
- `backend/internal/services/chat_monitors_test.go`

**Backend — modificados:**
- `backend/internal/models/chat.go` — `MonitorIDs`, `LegacyMonitorID`, `Normalize()`
- `backend/internal/repository/chat_repo.go` — índices, `ListConversationsByUser`, `Normalize()` al leer
- `backend/internal/services/chat_llm_parsing.go` — campo `Monitor` en `ChatQueryInput`
- `backend/internal/services/chat_service.go` — prompt multi-schema, `executeQuery` con allowlist, cotas escaladas, historial con referencia a artefacto
- `backend/internal/handlers/chat_handler.go` — `monitorIds`, listado por usuario, endpoint de agregar monitor
- `backend/internal/router/router.go` — ruta nueva

**Frontend — modificados:**
- `frontend/src/lib/api/chat.ts` — tipos y llamadas
- `frontend/src/app/(dashboard)/chatbot/page.tsx` — inversión del flujo

**Frontend — nuevo:**
- `frontend/src/components/chat/monitor-multi-select.tsx` — multi-select con búsqueda y chips

---

## Task 1: Alias de monitor

Función pura, sin dependencias. Es la base de la allowlist de seguridad, así que va primera y con tests exhaustivos.

**Files:**
- Create: `backend/internal/services/chat_monitors.go`
- Test: `backend/internal/services/chat_monitors_test.go`

**Interfaces:**
- Consumes: `models.Monitor` (ya existe: campos `ID`, `Name`, `Schema`, `CollectionID`)
- Produces:
  - `func monitorAlias(name string) string`
  - `func buildMonitorAliases(monitors []models.Monitor) map[string]*models.Monitor`

- [ ] **Step 1: Escribir el test que falla**

Crear `backend/internal/services/chat_monitors_test.go`:

```go
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
```

- [ ] **Step 2: Correr el test y verificar que falla**

Run: `cd backend && go test ./internal/services/ -run 'TestMonitorAlias|TestBuildMonitorAliases' -v`
Expected: FAIL, no compila — `undefined: monitorAlias`, `undefined: buildMonitorAliases`.

- [ ] **Step 3: Implementar**

Crear `backend/internal/services/chat_monitors.go`:

```go
package services

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/thureos/compliance/internal/models"
	"golang.org/x/text/unicode/norm"
)

// monitorAliasMaxLen topea el alias — el LLM lo escribe en cada tool
// call, y un nombre de monitor largo no aporta legibilidad extra.
const monitorAliasMaxLen = 40

// stripDiacritics descompone en NFD y descarta las marcas combinantes
// (categoría Mn), que es como se quitan tildes y diéresis sin tabla
// manual: "Región" -> "Region".
func stripDiacritics(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// monitorAlias deriva el identificador corto que el LLM usa para nombrar
// un monitor en query_monitor_data. Determinista para un mismo nombre.
// Devuelve "" si el nombre no tiene ningún carácter alfanumérico — el
// llamador (buildMonitorAliases) se encarga del respaldo.
func monitorAlias(name string) string {
	var b strings.Builder
	prevUnderscore := false
	for _, r := range strings.ToLower(stripDiacritics(name)) {
		switch {
		case r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			prevUnderscore = false
		case !prevUnderscore && b.Len() > 0:
			// Colapsa cualquier corrida de separadores en un solo "_", y
			// nunca abre el alias con uno.
			b.WriteRune('_')
			prevUnderscore = true
		}
	}
	alias := strings.Trim(b.String(), "_")
	if len(alias) > monitorAliasMaxLen {
		alias = strings.Trim(alias[:monitorAliasMaxLen], "_")
	}
	return alias
}

// buildMonitorAliases arma el mapa alias -> monitor de una conversación.
// Es la ÚNICA fuente de verdad de qué monitores puede tocar el LLM en un
// turno: se construye a partir de los monitores de la conversación y de
// nada más. Ver Global Constraints (autorización).
//
// El orden de `monitors` define qué monitor se queda con el alias base
// cuando hay colisión, así que debe ser estable (el de conv.MonitorIDs).
func buildMonitorAliases(monitors []models.Monitor) map[string]*models.Monitor {
	aliases := make(map[string]*models.Monitor, len(monitors))
	for i := range monitors {
		base := monitorAlias(monitors[i].Name)
		if base == "" {
			base = fmt.Sprintf("monitor_%d", i+1)
		}
		alias := base
		for n := 2; ; n++ {
			if _, taken := aliases[alias]; !taken {
				break
			}
			alias = fmt.Sprintf("%s_%d", base, n)
		}
		aliases[alias] = &monitors[i]
	}
	return aliases
}
```

- [ ] **Step 4: Correr los tests y verificar que pasan**

Run: `cd backend && go test ./internal/services/ -run 'TestMonitorAlias|TestBuildMonitorAliases' -v`
Expected: PASS, los 5 tests.

Si `golang.org/x/text` figura como `// indirect` en `go.mod`, correr `go mod tidy` para promoverla a directa. No descarga nada nuevo: la versión v0.41.0 ya está en el módulo.

- [ ] **Step 5: Commit**

```bash
cd backend && go mod tidy && gofmt -w internal/services/chat_monitors.go internal/services/chat_monitors_test.go
git add backend/internal/services/chat_monitors.go backend/internal/services/chat_monitors_test.go backend/go.mod
git commit -m "feat(chat): derivar alias de monitor para el contrato multi-monitor"
```

---

## Task 2: Modelo — `MonitorIDs` y normalización de documentos legacy

**Files:**
- Modify: `backend/internal/models/chat.go:66-75` (struct `ChatConversation`)
- Test: `backend/internal/models/chat_test.go` (crear)

**Interfaces:**
- Produces:
  - `ChatConversation.MonitorIDs []primitive.ObjectID` (bson `monitor_ids`, json `monitorIds`)
  - `ChatConversation.LegacyMonitorID *primitive.ObjectID` (bson `monitor_id`, json `-`)
  - `func (c *ChatConversation) Normalize()`
  - `const MaxMonitorsPerConversation = 6`

- [ ] **Step 1: Escribir el test que falla**

Crear `backend/internal/models/chat_test.go`:

```go
package models

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestNormalize_DocumentoLegacySeConvierteAMonitorIDs(t *testing.T) {
	id := primitive.NewObjectID()
	conv := ChatConversation{LegacyMonitorID: &id}
	conv.Normalize()

	if len(conv.MonitorIDs) != 1 || conv.MonitorIDs[0] != id {
		t.Fatalf("quiero MonitorIDs=[%s], tengo %v", id.Hex(), conv.MonitorIDs)
	}
	if conv.LegacyMonitorID != nil {
		t.Error("LegacyMonitorID debe quedar en nil después de normalizar")
	}
}

func TestNormalize_DocumentoNuevoNoSeToca(t *testing.T) {
	ids := []primitive.ObjectID{primitive.NewObjectID(), primitive.NewObjectID()}
	conv := ChatConversation{MonitorIDs: ids}
	conv.Normalize()

	if len(conv.MonitorIDs) != 2 || conv.MonitorIDs[0] != ids[0] || conv.MonitorIDs[1] != ids[1] {
		t.Fatalf("MonitorIDs no debe cambiar; tengo %v", conv.MonitorIDs)
	}
}

// Un documento a medio migrar (ambos campos presentes) debe quedarse con
// monitor_ids: es el campo que se escribe hoy, el legacy es residuo.
func TestNormalize_AmbosCamposGanaMonitorIDs(t *testing.T) {
	nuevo := primitive.NewObjectID()
	viejo := primitive.NewObjectID()
	conv := ChatConversation{MonitorIDs: []primitive.ObjectID{nuevo}, LegacyMonitorID: &viejo}
	conv.Normalize()

	if len(conv.MonitorIDs) != 1 || conv.MonitorIDs[0] != nuevo {
		t.Fatalf("quiero que gane monitor_ids (%s), tengo %v", nuevo.Hex(), conv.MonitorIDs)
	}
}

func TestNormalize_SinNingunCampoQuedaVacio(t *testing.T) {
	conv := ChatConversation{}
	conv.Normalize()
	if len(conv.MonitorIDs) != 0 {
		t.Fatalf("quiero MonitorIDs vacío, tengo %v", conv.MonitorIDs)
	}
}
```

- [ ] **Step 2: Correr el test y verificar que falla**

Run: `cd backend && go test ./internal/models/ -run TestNormalize -v`
Expected: FAIL, no compila — `conv.LegacyMonitorID undefined`, `conv.Normalize undefined`.

- [ ] **Step 3: Implementar**

En `backend/internal/models/chat.go`, reemplazar el struct `ChatConversation` (líneas 66-75) por:

```go
// MaxMonitorsPerConversation topea cuántos monitores puede abarcar una
// conversación. Cada monitor agrega un bloque de schema al system prompt
// y al menos una iteración de herramienta; más allá de esto el prompt se
// vuelve caro y la respuesta lenta sin que el caso de uso lo pida.
const MaxMonitorsPerConversation = 6

// ChatConversation es privada de UserID — ver Global Constraints.
type ChatConversation struct {
	ID         primitive.ObjectID   `bson:"_id,omitempty" json:"id"`
	MonitorIDs []primitive.ObjectID `bson:"monitor_ids,omitempty" json:"monitorIds"`
	// LegacyMonitorID: conversaciones creadas antes del multi-monitor
	// guardaban un único "monitor_id" escalar. Nunca se escribe; solo se
	// lee para que Normalize lo colapse en MonitorIDs. No se expone en
	// JSON: fuera del repositorio nadie debería saber que existe.
	LegacyMonitorID *primitive.ObjectID `bson:"monitor_id,omitempty" json:"-"`
	UserID          primitive.ObjectID  `bson:"user_id" json:"userId"`
	Title           string              `bson:"title" json:"title"`
	CreatedAt       time.Time           `bson:"created_at" json:"createdAt"`
	UpdatedAt       time.Time           `bson:"updated_at" json:"updatedAt"`
}

// Normalize colapsa el campo legacy en MonitorIDs. Todo lector de
// conversaciones debe llamarla justo después de decodificar; el
// repositorio la centraliza para que ningún llamador pueda olvidarla.
func (c *ChatConversation) Normalize() {
	if len(c.MonitorIDs) == 0 && c.LegacyMonitorID != nil {
		c.MonitorIDs = []primitive.ObjectID{*c.LegacyMonitorID}
	}
	c.LegacyMonitorID = nil
}
```

- [ ] **Step 4: Correr los tests y verificar que pasan**

Run: `cd backend && go test ./internal/models/ -run TestNormalize -v`
Expected: PASS, los 4 tests.

`go build ./...` va a fallar en `chat_handler.go`, `chat_repo.go` y `chat_service.go` porque usan `conv.MonitorID`. Es esperado: se arreglan en las tareas 3, 6 y 8. Para no dejar el árbol roto, esta tarea se commitea junto con la 3.

- [ ] **Step 5: No commitear todavía**

Esta tarea deja el build roto a propósito. Seguir directo a la Task 3, que lo cierra.

---

## Task 3: Repositorio — listado por usuario e índices

**Files:**
- Modify: `backend/internal/repository/chat_repo.go:29-37` (índices), `:52-59` (`GetConversation`), `:61-76` (listado)

**Interfaces:**
- Consumes: `models.ChatConversation.Normalize()` (Task 2)
- Produces: `func (r *ChatRepository) ListConversationsByUser(ctx context.Context, userID primitive.ObjectID, monitorFilter *primitive.ObjectID) ([]models.ChatConversation, error)`
- Elimina: `ListConversationsByMonitorAndUser`

- [ ] **Step 1: Reemplazar los índices**

En `backend/internal/repository/chat_repo.go`, dentro de `ensureIndexes`, reemplazar el índice de `conversations`:

```go
func (r *ChatRepository) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// {user_id, updated_at}: la consulta principal del sidebar, que ya no
	// se scopea por monitor. {user_id, monitor_ids}: el filtro opcional
	// "solo las que incluyen X" — Mongo indexa arrays elemento a elemento,
	// así que un multikey sobre monitor_ids alcanza.
	_, _ = r.conversations.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "updated_at", Value: -1}}},
		{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "monitor_ids", Value: 1}}},
	})
	_, _ = r.messages.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "conversation_id", Value: 1}, {Key: "created_at", Value: 1}},
	})
}
```

> El índice viejo `{monitor_id, user_id, updated_at}` no se borra automáticamente. Queda huérfano y no molesta; se puede eliminar a mano con `db.chat_conversations.dropIndex("monitor_id_1_user_id_1_updated_at_-1")`.

- [ ] **Step 2: Normalizar al leer**

Reemplazar `GetConversation` y `ListConversationsByMonitorAndUser`:

```go
func (r *ChatRepository) GetConversation(ctx context.Context, id primitive.ObjectID) (*models.ChatConversation, error) {
	var conv models.ChatConversation
	if err := r.conversations.FindOne(ctx, bson.M{"_id": id}).Decode(&conv); err != nil {
		return nil, fmt.Errorf("conversación %s no encontrada: %w", id.Hex(), err)
	}
	conv.Normalize()
	return &conv, nil
}

// ListConversationsByUser devuelve las conversaciones del usuario
// ordenadas por actividad. monitorFilter es opcional: si viene, acota a
// las conversaciones que incluyen ese monitor.
//
// A diferencia de la versión anterior, el scoping por monitor dejó de ser
// obligatorio: una conversación que abarca varios monitores no pertenece
// a la lista de ninguno en particular.
func (r *ChatRepository) ListConversationsByUser(
	ctx context.Context,
	userID primitive.ObjectID,
	monitorFilter *primitive.ObjectID,
) ([]models.ChatConversation, error) {
	filter := bson.M{"user_id": userID}
	if monitorFilter != nil {
		// Matchea tanto monitor_ids (array) como el monitor_id escalar de
		// documentos legacy: en Mongo, igualdad contra un campo array
		// matchea si algún elemento coincide.
		filter["$or"] = []bson.M{
			{"monitor_ids": *monitorFilter},
			{"monitor_id": *monitorFilter},
		}
	}
	cursor, err := r.conversations.Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("listando conversaciones: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	results := []models.ChatConversation{}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("decodificando conversaciones: %w", err)
	}
	for i := range results {
		results[i].Normalize()
	}
	return results, nil
}

// AddMonitor agrega un monitor a la conversación sin duplicarlo.
// $addToSet sobre monitor_ids es idempotente: repetir la llamada con el
// mismo monitor no cambia nada.
func (r *ChatRepository) AddMonitor(ctx context.Context, id, monitorID primitive.ObjectID) error {
	_, err := r.conversations.UpdateByID(ctx, id, bson.M{"$addToSet": bson.M{"monitor_ids": monitorID}})
	if err != nil {
		return fmt.Errorf("agregando monitor a la conversación %s: %w", id.Hex(), err)
	}
	return nil
}
```

- [ ] **Step 3: Compilar el paquete**

Run: `cd backend && go build ./internal/repository/ ./internal/models/`
Expected: compila sin errores.

- [ ] **Step 4: Correr los tests de modelos**

Run: `cd backend && go test ./internal/models/ -v`
Expected: PASS (los 4 tests de la Task 2).

- [ ] **Step 5: Commit**

```bash
cd backend && gofmt -w internal/models/chat.go internal/models/chat_test.go internal/repository/chat_repo.go
git add backend/internal/models/chat.go backend/internal/models/chat_test.go backend/internal/repository/chat_repo.go
git commit -m "feat(chat): conversaciones con varios monitores y listado por usuario"
```

---

## Task 4: Contrato de la herramienta — campo `monitor`

**Files:**
- Modify: `backend/internal/services/chat_llm_parsing.go:17-22` (`ChatQueryInput`), `:26-41` (`ParseQueryToolInput`)
- Test: `backend/internal/services/chat_llm_parsing_test.go` (existe; agregar casos)

**Interfaces:**
- Produces: `ChatQueryInput.Monitor string` (json `monitor`)

- [ ] **Step 1: Escribir el test que falla**

Agregar al final de `backend/internal/services/chat_llm_parsing_test.go`:

```go
func TestParseQueryToolInput_ExigeMonitor(t *testing.T) {
	raw := []byte(`{"aggregate": {"field": "monto", "function": "sum"}}`)
	if _, err := ParseQueryToolInput(raw); err == nil {
		t.Fatal("un input sin 'monitor' debe ser rechazado")
	}
}

func TestParseQueryToolInput_RechazaMonitorVacio(t *testing.T) {
	raw := []byte(`{"monitor": "   ", "aggregate": {"field": "monto", "function": "sum"}}`)
	if _, err := ParseQueryToolInput(raw); err == nil {
		t.Fatal("un 'monitor' en blanco debe ser rechazado")
	}
}

func TestParseQueryToolInput_ConservaYNormalizaElMonitor(t *testing.T) {
	raw := []byte(`{"monitor": "  transacciones  ", "aggregate": {"field": "monto", "function": "sum"}}`)
	input, err := ParseQueryToolInput(raw)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if input.Monitor != "transacciones" {
		t.Errorf("Monitor = %q, quiero %q", input.Monitor, "transacciones")
	}
}
```

- [ ] **Step 2: Correr el test y verificar que falla**

Run: `cd backend && go test ./internal/services/ -run TestParseQueryToolInput -v`
Expected: FAIL — `input.Monitor undefined`, y los dos primeros fallan porque hoy no se valida.

- [ ] **Step 3: Implementar**

En `chat_llm_parsing.go`, agregar el campo y la validación:

```go
type ChatQueryInput struct {
	// Monitor es el ALIAS del monitor a consultar, no su ObjectID. Lo
	// resuelve executeQuery contra la allowlist de la conversación — ver
	// Global Constraints (autorización).
	Monitor        string                 `json:"monitor"`
	ConditionGroup *models.ConditionGroup `json:"conditionGroup,omitempty"`
	Aggregate      *ChatAggregateSpec     `json:"aggregate,omitempty"`
	Limit          int                    `json:"limit,omitempty"`
}
```

Y en `ParseQueryToolInput`, justo después del `json.Unmarshal`:

```go
	input.Monitor = strings.TrimSpace(input.Monitor)
	if input.Monitor == "" {
		return ChatQueryInput{}, fmt.Errorf("query_monitor_data requiere 'monitor' (el alias del monitor a consultar)")
	}
```

Agregar `"strings"` a los imports del archivo.

- [ ] **Step 4: Correr los tests y verificar que pasan**

Run: `cd backend && go test ./internal/services/ -run TestParseQueryToolInput -v`
Expected: PASS.

> Otros tests del paquete pueden fallar si construyen un input sin `monitor`. Corregirlos agregando `"monitor": "m1"` al JSON de prueba: el contrato cambió y los tests deben reflejarlo.

- [ ] **Step 5: Commit**

```bash
cd backend && gofmt -w internal/services/chat_llm_parsing.go internal/services/chat_llm_parsing_test.go
git add backend/internal/services/chat_llm_parsing.go backend/internal/services/chat_llm_parsing_test.go
git commit -m "feat(chat): exigir alias de monitor en el input de query_monitor_data"
```

---

## Task 5: System prompt multi-schema

**Files:**
- Modify: `backend/internal/services/chat_service.go:105-131` (`chatSystemPromptTemplate`, `buildSystemPrompt`), `:44-88` (descripción y schema de la herramienta)
- Test: `backend/internal/services/chat_monitors_test.go` (agregar)

**Interfaces:**
- Consumes: `buildMonitorAliases` (Task 1)
- Produces: `func buildSystemPrompt(aliases map[string]*models.Monitor) string`

- [ ] **Step 1: Escribir el test que falla**

Agregar a `chat_monitors_test.go`:

```go
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
```

Agregar `"strings"` a los imports del archivo de test.

- [ ] **Step 2: Correr el test y verificar que falla**

Run: `cd backend && go test ./internal/services/ -run TestBuildSystemPrompt -v`
Expected: FAIL — `buildSystemPrompt` hoy recibe `*models.Monitor`, no un mapa.

- [ ] **Step 3: Implementar**

En `chat_service.go`, reemplazar `chatSystemPromptTemplate` y `buildSystemPrompt`:

```go
const chatSystemPromptTemplate = `Sos un analista de datos que responde preguntas sobre los monitores listados abajo.

%s
Tenés una herramienta, query_monitor_data, para traer datos REALES — nunca
inventes números. Usala todas las veces que necesites para responder bien.
Cada llamada consulta UN monitor: indicá cuál en el campo "monitor" usando
su alias exacto de la lista. Para comparar entre monitores, llamá una vez
por cada uno y correlacioná los resultados al responder.

Si la respuesta se presta a visualizarse (series de tiempo, comparaciones,
rankings, distribuciones), agregá al FINAL de tu respuesta un bloque
delimitado exactamente así, con un JSON válido dentro:

<artifact>
{"type": "chart", "title": "...", "chartSpec": {"chartType": "bar", "data": [{"x": "...", "y": 1}], "xKey": "x", "yKeys": ["y"]}}
</artifact>

Para una tabla simple: "type": "table" con el mismo "chartSpec" (sin
chartType). Si necesitás algo que un gráfico de barra/línea/torta no puede
expresar, usá "type": "custom" con "code": HTML/JS autocontenido que
renderice la visualización — ese código corre aislado en un iframe sin
acceso a la sesión ni a la red, así que tiene que traer sus propios datos
embebidos en el propio "code" (no puede hacer fetch a nada).

Si no hace falta artefacto, no incluyas el bloque <artifact>. Respondé
siempre en español.`

// buildSystemPrompt emite un bloque de schema por monitor, encabezado por
// el alias con el que el LLM debe nombrarlo.
//
// Recorre los alias ORDENADOS: el orden de recorrido de un mapa en Go es
// aleatorio, y un prompt que cambia de texto en cada turno invalida el
// caché de prompt del proveedor y hace irreproducible cualquier bug.
func buildSystemPrompt(aliases map[string]*models.Monitor) string {
	names := make([]string, 0, len(aliases))
	for alias := range aliases {
		names = append(names, alias)
	}
	sort.Strings(names)

	var blocks strings.Builder
	blocks.WriteString("Monitores disponibles (usá el alias en el campo \"monitor\"):\n\n")
	for _, alias := range names {
		monitor := aliases[alias]
		fmt.Fprintf(&blocks, "### %s — %q\n", alias, monitor.Name)
		for _, f := range monitor.Schema {
			fmt.Fprintf(&blocks, "- %s: %s\n", f.Name, f.Type)
		}
		blocks.WriteString("\n")
	}
	return fmt.Sprintf(chatSystemPromptTemplate, blocks.String())
}
```

Agregar `"sort"` a los imports de `chat_service.go`.

En el mismo archivo, agregar la propiedad `monitor` a `queryToolJSONSchema` (dentro de `"properties"`, antes de `"conditionGroup"`) y marcarla obligatoria:

```go
var queryToolJSONSchema = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"monitor": map[string]interface{}{
			"type":        "string",
			"description": "Alias del monitor a consultar. Debe ser uno de los alias listados en el prompt.",
		},
		// ... conditionGroup, aggregate, limit sin cambios ...
	},
	"required": []string{"monitor"},
}
```

Y agregar al final de `queryToolDescription`:

```
Cada llamada consulta UN monitor: indicá cuál en "monitor" con su alias exacto. Para comparar entre monitores, llamá una vez por cada uno.
```

**Pasar `required` a Anthropic.** En `askAnthropic`, el `ToolInputSchemaParam` se arma hoy solo con `queryToolJSONSchema["properties"]`, así que la clave `required` se descarta para el proveedor por defecto. Agregarla:

```go
		InputSchema: anthropic.ToolInputSchemaParam{
			Type:       "object",
			Properties: queryToolJSONSchema["properties"],
			Required:   queryToolJSONSchema["required"].([]string),
		},
```

Sin esto el modelo puede omitir `monitor`; `ParseQueryToolInput` igual lo rechaza y el LLM se corrige solo, así que el costo es una iteración perdida, no un error. Si el SDK no expone `Required` en esa struct, dejarlo como está y anotarlo: la validación del servidor cubre el caso.

- [ ] **Step 4: Correr los tests y verificar que pasan**

Run: `cd backend && go test ./internal/services/ -run 'TestBuildSystemPrompt|TestMonitorAlias|TestBuildMonitorAliases' -v`
Expected: PASS. `go build ./...` sigue roto por `chat_service.go` (Task 6) — esperado.

- [ ] **Step 5: No commitear todavía**

Sigue directo a la Task 6, que cierra el build.

---

## Task 6: `executeQuery` con allowlist y cotas escaladas

Esta tarea implementa la invariante de seguridad central del plan. Los tests de autorización no son opcionales.

**Files:**
- Modify: `backend/internal/services/chat_service.go:171-197` (`executeQuery`), `:199-280` (`Ask`), `:26` (`chatMaxToolIterations`)
- Test: `backend/internal/services/chat_monitors_test.go` (agregar)

**Interfaces:**
- Consumes: `buildMonitorAliases` (Task 1), `ChatQueryInput.Monitor` (Task 4)
- Produces:
  - `func resolveQueryMonitor(aliases map[string]*models.Monitor, alias string) (*models.Monitor, error)`
  - `func chatToolIterations(monitorCount int) int`
  - `func chatAskTimeout(monitorCount int) time.Duration`

- [ ] **Step 1: Escribir el test que falla**

Agregar a `chat_monitors_test.go`:

```go
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
```

Agregar `"time"` a los imports del archivo de test.

- [ ] **Step 2: Correr el test y verificar que falla**

Run: `cd backend && go test ./internal/services/ -run 'TestResolveQueryMonitor|TestChatToolIterations|TestChatAskTimeout' -v`
Expected: FAIL — las tres funciones no existen.

- [ ] **Step 3: Implementar las funciones puras**

Agregar a `backend/internal/services/chat_monitors.go`:

```go
// resolveQueryMonitor traduce el alias que mandó el LLM al monitor real.
//
// ESTA ES LA FRONTERA DE AUTORIZACIÓN del chatbot multi-monitor: el mapa
// `aliases` se construye solo a partir de conv.MonitorIDs, así que un
// alias que no está ahí no tiene forma de convertirse en una consulta.
// Nunca se acepta un ObjectID crudo: si el LLM manda un hex, cae acá como
// alias desconocido igual que cualquier otro texto.
//
// Importa porque los datos que el LLM lee son archivos subidos por
// usuarios: una celda con una inyección de prompt puede pedirle que
// consulte otro monitor, y este chequeo es lo que hace que ese pedido no
// llegue a ningún lado.
func resolveQueryMonitor(aliases map[string]*models.Monitor, alias string) (*models.Monitor, error) {
	if monitor, ok := aliases[alias]; ok {
		return monitor, nil
	}
	valid := make([]string, 0, len(aliases))
	for a := range aliases {
		valid = append(valid, a)
	}
	sort.Strings(valid)
	return nil, fmt.Errorf(
		"el monitor %q no existe en esta conversación; los disponibles son: %s",
		alias, strings.Join(valid, ", "),
	)
}

// chatToolIterations y chatAskTimeout escalan con la cantidad de
// monitores: una pregunta cruzada necesita al menos una llamada por
// monitor antes de poder empezar a razonar, así que las cotas fijas
// pensadas para un solo monitor se agotaban sin haber respondido.
const (
	chatBaseToolIterations = 5
	chatToolIterationsPerMonitor = 3
	chatMaxToolIterations = 20

	chatBaseAskTimeout = 60 * time.Second
	chatAskTimeoutPerMonitor = 30 * time.Second
	chatMaxAskTimeout = 240 * time.Second
)

func chatToolIterations(monitorCount int) int {
	n := chatBaseToolIterations + chatToolIterationsPerMonitor*monitorCount
	if n > chatMaxToolIterations {
		return chatMaxToolIterations
	}
	return n
}

func chatAskTimeout(monitorCount int) time.Duration {
	d := chatBaseAskTimeout + chatAskTimeoutPerMonitor*time.Duration(monitorCount)
	if d > chatMaxAskTimeout {
		return chatMaxAskTimeout
	}
	return d
}
```

Agregar `"sort"` y `"time"` a los imports de `chat_monitors.go`.

Borrar la constante `chatMaxToolIterations` vieja de `chat_service.go:26` (línea `const chatMaxToolIterations = 5` y su comentario): ahora vive acá con otro significado (el tope, no el valor fijo).

- [ ] **Step 4: Correr los tests y verificar que pasan**

Run: `cd backend && go test ./internal/services/ -run 'TestResolveQueryMonitor|TestChatToolIterations|TestChatAskTimeout' -v`
Expected: PASS, los 6 tests.

- [ ] **Step 5: Cablear `executeQuery` y `Ask`**

En `chat_service.go`, cambiar la firma de `executeQuery` para que reciba la allowlist en vez de un monitor fijo:

```go
// executeQuery ejecuta lo que el LLM pidió vía query_monitor_data contra
// los datos reales del monitor que nombró, y devuelve el resultado
// serializado.
//
// El alias se resuelve contra `aliases` — la allowlist de la conversación
// — ANTES de tocar Mongo. Ver resolveQueryMonitor y Global Constraints.
func (s *ChatService) executeQuery(ctx context.Context, aliases map[string]*models.Monitor, raw json.RawMessage) (string, error) {
	input, err := ParseQueryToolInput(raw)
	if err != nil {
		return "", err
	}

	monitor, err := resolveQueryMonitor(aliases, input.Monitor)
	if err != nil {
		return "", err
	}

	if input.Aggregate != nil {
		pipeline := buildDataAggregatePipeline(*input.Aggregate)
		results, err := s.monitorRepo.AggregateData(ctx, monitor.CollectionID, pipeline)
		if err != nil {
			return "", fmt.Errorf("ejecutando agregación: %w", err)
		}
		out, _ := json.Marshal(results)
		return string(out), nil
	}

	filter := bson.M{}
	if input.ConditionGroup != nil {
		filter = BuildMongoFilter(*input.ConditionGroup)
	}
	results, err := s.monitorRepo.QueryData(ctx, monitor.CollectionID, filter, int64(input.Limit))
	if err != nil {
		return "", fmt.Errorf("ejecutando consulta: %w", err)
	}
	out, _ := json.Marshal(results)
	return string(out), nil
}
```

En `Ask`, reemplazar la carga del monitor único y el timeout fijo:

```go
func (s *ChatService) Ask(ctx context.Context, conversationID, userID primitive.ObjectID, userMessage string) (models.ChatMessage, error) {
	// La conversación se lee ANTES de fijar el timeout: cuántos monitores
	// abarca es lo que define cuánto puede tardar.
	conv, err := s.chatRepo.GetConversation(ctx, conversationID)
	if err != nil {
		return models.ChatMessage{}, ErrConversationNotFound
	}
	if conv.UserID != userID {
		return models.ChatMessage{}, ErrConversationNotFound
	}

	ctx, cancel := context.WithTimeout(ctx, chatAskTimeout(len(conv.MonitorIDs)))
	defer cancel()

	// Un monitor borrado mientras la conversación seguía apuntando a él se
	// excluye en silencio: la conversación sigue sirviendo con el resto.
	// Solo si no queda ninguno es un error.
	monitors := make([]models.Monitor, 0, len(conv.MonitorIDs))
	for _, id := range conv.MonitorIDs {
		monitor, err := s.monitorRepo.FindByID(ctx, id)
		if err != nil {
			continue
		}
		monitors = append(monitors, *monitor)
	}
	if len(monitors) == 0 {
		return models.ChatMessage{}, ErrMonitorNotFound
	}
	aliases := buildMonitorAliases(monitors)

	// ... el resto de Ask sin cambios hasta la llamada al proveedor ...
```

y más abajo, donde hoy dice `s.askDeepSeek(ctx, cfg.AI, monitor, ...)` / `s.askAnthropic(ctx, cfg.AI, monitor, ...)`, pasar `aliases` en lugar de `monitor`.

Cambiar las firmas de `askAnthropic` y `askDeepSeek` de `monitor *models.Monitor` a `aliases map[string]*models.Monitor`, y dentro de cada una:

- `systemPrompt := buildSystemPrompt(aliases)` (en vez de `buildSystemPrompt(monitor)`)
- `for i := 0; i < chatToolIterations(len(aliases)); i++` (en vez de `chatMaxToolIterations`)
- `s.executeQuery(ctx, aliases, block.Input)` / `s.executeQuery(ctx, aliases, json.RawMessage(call.Function.Arguments))`
- En los mensajes de error del fallback final, reemplazar `chatMaxToolIterations` por `chatToolIterations(len(aliases))`.

- [ ] **Step 6: Compilar y correr todo el paquete**

Run: `cd backend && go build ./... && go test ./internal/services/ -v`
Expected: compila; los tests del paquete pasan.

- [ ] **Step 7: Commit**

```bash
cd backend && gofmt -w internal/services/
git add backend/internal/services/chat_service.go backend/internal/services/chat_monitors.go backend/internal/services/chat_monitors_test.go
git commit -m "feat(chat): resolver el monitor por alias contra la allowlist de la conversación"
```

---

## Task 7: Historial con referencia al artefacto

**Files:**
- Modify: `backend/internal/services/chat_service.go:288-300` (`historyToAnthropic`), `:395-402` (`historyToDeepSeek`)
- Test: `backend/internal/services/chat_monitors_test.go` (agregar)

**Interfaces:**
- Produces: `func historyContent(m models.ChatMessage) string`

- [ ] **Step 1: Escribir el test que falla**

Agregar a `chat_monitors_test.go`:

```go
func TestHistoryContent_MensajeSinArtefactoNoCambia(t *testing.T) {
	m := models.ChatMessage{Role: models.ChatRoleAssistant, Content: "El total es 42."}
	if got := historyContent(m); got != "El total es 42." {
		t.Errorf("historyContent = %q, quiero el contenido sin tocar", got)
	}
}

// Sin esto el asistente no tiene forma de saber que en un turno anterior
// generó un artefacto, y "volvé a mostrarme la tabla de antes" no puede
// funcionar.
func TestHistoryContent_MensajeConArtefactoAgregaLaReferencia(t *testing.T) {
	m := models.ChatMessage{
		Role:     models.ChatRoleAssistant,
		Content:  "Acá va el desglose.",
		Artifact: &models.ChatArtifact{Type: models.ChatArtifactTable, Title: "Ventas por región"},
	}
	got := historyContent(m)

	if !strings.Contains(got, "Acá va el desglose.") {
		t.Errorf("debe conservar el texto original; tengo %q", got)
	}
	for _, want := range []string{"table", "Ventas por región"} {
		if !strings.Contains(got, want) {
			t.Errorf("la referencia debería mencionar %q; tengo %q", want, got)
		}
	}
}

// Los datos del artefacto son miles de tokens por turno y el LLM ya los
// describió en su texto: solo va la referencia.
func TestHistoryContent_NoIncluyeLosDatosDelArtefacto(t *testing.T) {
	m := models.ChatMessage{
		Role:    models.ChatRoleAssistant,
		Content: "Listo.",
		Artifact: &models.ChatArtifact{
			Type:  models.ChatArtifactTable,
			Title: "T",
			ChartSpec: &models.ChartSpec{
				Data: []map[string]interface{}{{"secreto": "no_debe_aparecer"}},
			},
		},
	}
	if strings.Contains(historyContent(m), "no_debe_aparecer") {
		t.Error("los datos del artefacto no deben ir al historial")
	}
}
```

- [ ] **Step 2: Correr el test y verificar que falla**

Run: `cd backend && go test ./internal/services/ -run TestHistoryContent -v`
Expected: FAIL — `undefined: historyContent`.

- [ ] **Step 3: Implementar**

Agregar a `chat_service.go`, junto a las funciones de historial:

```go
// historyContent arma el texto con el que un mensaje viaja de vuelta al
// LLM en el historial. Si el mensaje llevaba artefacto, agrega una línea
// de referencia: sin ella el asistente no tiene forma de saber que ya
// generó uno, y no puede responder a "volvé a mostrarme la tabla de
// antes". Van el tipo y el título, nunca los datos — son miles de tokens
// por turno y el LLM ya los describió en su propio texto.
func historyContent(m models.ChatMessage) string {
	if m.Artifact == nil {
		return m.Content
	}
	return fmt.Sprintf("%s\n\n[Generaste un artefacto: tipo=%s, título=%q]",
		m.Content, m.Artifact.Type, m.Artifact.Title)
}
```

En `historyToAnthropic`, reemplazar `anthropic.NewTextBlock(m.Content)` por `anthropic.NewTextBlock(historyContent(m))`.

En `historyToDeepSeek`, reemplazar `Content: m.Content` por `Content: historyContent(m)`.

- [ ] **Step 4: Correr los tests y verificar que pasan**

Run: `cd backend && go test ./internal/services/ -run TestHistoryContent -v`
Expected: PASS, los 3 tests.

- [ ] **Step 5: Commit**

```bash
cd backend && gofmt -w internal/services/chat_service.go internal/services/chat_monitors_test.go
git add backend/internal/services/chat_service.go backend/internal/services/chat_monitors_test.go
git commit -m "feat(chat): referenciar el artefacto generado en el historial del LLM"
```

---

## Task 8: Handlers y rutas

**Files:**
- Modify: `backend/internal/handlers/chat_handler.go:22-45` (`CreateConversation`), `:47-64` (`ListConversations`)
- Modify: `backend/internal/router/router.go:127-132`

**Interfaces:**
- Consumes: `ListConversationsByUser`, `AddMonitor` (Task 3), `models.MaxMonitorsPerConversation` (Task 2)
- Produces: `func (h *ChatHandler) AddMonitor(c *fiber.Ctx) error`

- [ ] **Step 1: Reescribir `CreateConversation`**

```go
// CreateConversation arranca una conversación nueva sobre uno o varios
// monitores — privada del usuario autenticado (ver Global Constraints).
// El conjunto de monitores fijado acá es la allowlist que después usa
// executeQuery: por eso se valida que cada uno exista antes de guardar.
func (h *ChatHandler) CreateConversation(c *fiber.Ctx) error {
	var body struct {
		MonitorIDs []string `json:"monitorIds"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if len(body.MonitorIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "hay que elegir al menos un monitor"})
	}
	if len(body.MonitorIDs) > models.MaxMonitorsPerConversation {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("una conversación puede abarcar hasta %d monitores", models.MaxMonitorsPerConversation),
		})
	}

	monitorIDs := make([]primitive.ObjectID, 0, len(body.MonitorIDs))
	for _, raw := range body.MonitorIDs {
		id, err := primitive.ObjectIDFromHex(raw)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "monitorId inválido"})
		}
		if _, err := h.monitorRepo.FindByID(c.Context(), id); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "monitor no encontrado"})
		}
		monitorIDs = append(monitorIDs, id)
	}

	userID, err := primitive.ObjectIDFromHex(c.Locals("userId").(string))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "usuario inválido"})
	}

	conv := models.ChatConversation{MonitorIDs: monitorIDs, UserID: userID, Title: "Nueva conversación"}
	if err := h.chatRepo.CreateConversation(c.Context(), &conv); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(conv)
}
```

`ChatHandler` necesita `monitorRepo`. Agregar el campo al struct y al constructor:

```go
type ChatHandler struct {
	chatRepo    *repository.ChatRepository
	monitorRepo *repository.MonitorRepository
	chatService *services.ChatService
}

func NewChatHandler(chatRepo *repository.ChatRepository, monitorRepo *repository.MonitorRepository, chatService *services.ChatService) *ChatHandler {
	return &ChatHandler{chatRepo: chatRepo, monitorRepo: monitorRepo, chatService: chatService}
}
```

Actualizar la llamada a `NewChatHandler` donde se construyen los handlers (buscar con `grep -rn "NewChatHandler" backend/`) para pasarle el `monitorRepo` que ya existe ahí.

Agregar `"fmt"` a los imports de `chat_handler.go`.

- [ ] **Step 2: Reescribir `ListConversations` y agregar `AddMonitor`**

```go
// ListConversations devuelve las conversaciones del usuario autenticado,
// ordenadas por actividad. monitorId es un filtro OPCIONAL: una
// conversación que abarca varios monitores no pertenece a la lista de
// ninguno en particular, así que el scoping principal es por usuario.
func (h *ChatHandler) ListConversations(c *fiber.Ctx) error {
	userID, err := primitive.ObjectIDFromHex(c.Locals("userId").(string))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "usuario inválido"})
	}

	var monitorFilter *primitive.ObjectID
	if raw := c.Query("monitorId"); raw != "" {
		id, err := primitive.ObjectIDFromHex(raw)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "monitorId inválido"})
		}
		monitorFilter = &id
	}

	convs, err := h.chatRepo.ListConversationsByUser(c.Context(), userID, monitorFilter)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(convs)
}

// AddMonitor suma un monitor a una conversación existente. No hay
// operación inversa a propósito: el historial ya referenció ese monitor y
// sus datos, y quitarlo dejaría mensajes previos hablando de algo que el
// asistente ya no puede ver (ver el spec, "Fuera de alcance").
func (h *ChatHandler) AddMonitor(c *fiber.Ctx) error {
	convID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	var body struct {
		MonitorID string `json:"monitorId"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	monitorID, err := primitive.ObjectIDFromHex(body.MonitorID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "monitorId inválido"})
	}
	userID, err := primitive.ObjectIDFromHex(c.Locals("userId").(string))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "usuario inválido"})
	}

	conv, err := h.chatRepo.GetConversation(c.Context(), convID)
	if err != nil || conv.UserID != userID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "conversación no encontrada"})
	}
	for _, existing := range conv.MonitorIDs {
		if existing == monitorID {
			return c.JSON(conv) // idempotente: ya estaba
		}
	}
	if len(conv.MonitorIDs) >= models.MaxMonitorsPerConversation {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("una conversación puede abarcar hasta %d monitores", models.MaxMonitorsPerConversation),
		})
	}
	if _, err := h.monitorRepo.FindByID(c.Context(), monitorID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "monitor no encontrado"})
	}

	if err := h.chatRepo.AddMonitor(c.Context(), convID, monitorID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	conv.MonitorIDs = append(conv.MonitorIDs, monitorID)
	return c.JSON(conv)
}
```

- [ ] **Step 3: Registrar la ruta**

En `backend/internal/router/router.go`, agregar después de la línea 131:

```go
	chat.Post("/conversations/:id/monitors", h.Chat.AddMonitor)
```

- [ ] **Step 4: Compilar y correr los tests**

Run: `cd backend && go build ./... && go test ./... 2>&1 | tail -20`
Expected: compila; los tests de `handlers`, `middleware`, `router` y `services` pasan.

- [ ] **Step 5: Commit**

```bash
cd backend && gofmt -w internal/handlers/chat_handler.go internal/router/router.go
git add backend/internal/handlers/chat_handler.go backend/internal/router/router.go backend/cmd/server/main.go
git commit -m "feat(chat): endpoints de conversación multi-monitor"
```

---

## Task 9: Frontend — cliente de API

**Files:**
- Modify: `frontend/src/lib/api/chat.ts:20-58`

**Interfaces:**
- Produces: `ChatConversation.monitorIds: string[]`, `chatApi.createConversation(monitorIds)`, `chatApi.listConversations(monitorId?)`, `chatApi.addMonitor(conversationId, monitorId)`

- [ ] **Step 1: Actualizar tipos y llamadas**

En `frontend/src/lib/api/chat.ts`, reemplazar `ChatConversation` y el objeto `chatApi`:

```ts
export interface ChatConversation {
  id: string;
  monitorIds: string[];
  userId: string;
  title: string;
  createdAt: string;
  updatedAt: string;
}

export const chatApi = {
  createConversation: (monitorIds: string[]) =>
    api.post<ChatConversation>("/chat/conversations", { monitorIds }),

  /** monitorId es un filtro opcional: sin él trae todas las del usuario. */
  listConversations: (monitorId?: string) =>
    api.get<ChatConversation[]>(
      monitorId
        ? `/chat/conversations?monitorId=${monitorId}`
        : "/chat/conversations",
    ),

  addMonitor: (conversationId: string, monitorId: string) =>
    api.post<ChatConversation>(
      `/chat/conversations/${conversationId}/monitors`,
      { monitorId },
    ),

  listMessages: (conversationId: string) =>
    api.get<ChatMessage[]>(`/chat/conversations/${conversationId}/messages`),

  ask: (conversationId: string, content: string) =>
    api.post<ChatMessage>(`/chat/conversations/${conversationId}/messages`, {
      content,
    }),

  deleteConversation: (conversationId: string) =>
    api.delete<{ message: string }>(`/chat/conversations/${conversationId}`),
};
```

- [ ] **Step 2: Verificar tipos**

Run: `cd frontend && npx tsc --noEmit`
Expected: errores **solo** en `src/app/(dashboard)/chatbot/page.tsx` (usa `createConversation(monitorId)` con un string). Se arreglan en la Task 10.

- [ ] **Step 3: No commitear todavía**

Seguir a la Task 10, que cierra la compilación.

---

## Task 10: Frontend — multi-select de monitores

**Files:**
- Create: `frontend/src/components/chat/monitor-multi-select.tsx`

**Interfaces:**
- Produces: `export function MonitorMultiSelect({ monitors, selected, onChange, max }: MonitorMultiSelectProps)`

- [ ] **Step 1: Crear el componente**

```tsx
"use client";

import { useState } from "react";
import { Check, Search, X } from "lucide-react";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import type { Monitor } from "@/lib/types";

interface MonitorMultiSelectProps {
  monitors: Monitor[];
  selected: string[];
  onChange: (ids: string[]) => void;
  /** Tope del backend (models.MaxMonitorsPerConversation). */
  max: number;
}

/**
 * Selector de los monitores que va a abarcar una conversación. El
 * conjunto elegido acá es la allowlist que el backend usa para resolver
 * los alias del LLM, así que el tope se respeta también del lado del
 * servidor: esto es comodidad, no seguridad.
 */
export function MonitorMultiSelect({
  monitors,
  selected,
  onChange,
  max,
}: MonitorMultiSelectProps) {
  const [query, setQuery] = useState("");

  const visible = monitors.filter((m) =>
    m.name.toLowerCase().includes(query.trim().toLowerCase()),
  );
  const atLimit = selected.length >= max;

  function toggle(id: string) {
    if (selected.includes(id)) {
      onChange(selected.filter((s) => s !== id));
    } else if (!atLimit) {
      onChange([...selected, id]);
    }
  }

  return (
    <div className="space-y-2">
      {selected.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {selected.map((id) => {
            const monitor = monitors.find((m) => m.id === id);
            return (
              <span
                key={id}
                className="inline-flex items-center gap-1 rounded-full bg-accent px-2 py-0.5 text-xs text-accent-foreground"
              >
                {monitor?.name ?? id}
                <button
                  type="button"
                  onClick={() => toggle(id)}
                  aria-label={`Quitar ${monitor?.name ?? id}`}
                  className="rounded-full hover:bg-background/50"
                >
                  <X className="h-3 w-3" />
                </button>
              </span>
            );
          })}
        </div>
      )}

      <div className="relative">
        <Search className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Buscar monitor…"
          className="pl-8"
        />
      </div>

      <div className="max-h-56 overflow-y-auto rounded-md border">
        {visible.length === 0 && (
          <p className="p-3 text-sm text-muted-foreground">
            Ningún monitor coincide con la búsqueda.
          </p>
        )}
        {visible.map((m) => {
          const isSelected = selected.includes(m.id);
          const disabled = !isSelected && atLimit;
          return (
            <button
              key={m.id}
              type="button"
              disabled={disabled}
              onClick={() => toggle(m.id)}
              className={cn(
                "flex w-full items-center gap-2 px-3 py-2 text-left text-sm",
                isSelected ? "bg-accent/50" : "hover:bg-accent/30",
                disabled && "cursor-not-allowed opacity-50",
              )}
            >
              <span
                className={cn(
                  "flex h-4 w-4 shrink-0 items-center justify-center rounded-sm border",
                  isSelected && "bg-primary text-primary-foreground",
                )}
              >
                {isSelected && <Check className="h-3 w-3" />}
              </span>
              <span className="truncate">{m.name}</span>
            </button>
          );
        })}
      </div>

      <p className="text-xs text-muted-foreground">
        {selected.length} de {max} monitores
        {atLimit && " — llegaste al tope"}
      </p>
    </div>
  );
}
```

- [ ] **Step 2: Verificar tipos**

Run: `cd frontend && npx tsc --noEmit`
Expected: sigue el error de `page.tsx` de la Task 9, ninguno nuevo en este archivo.

- [ ] **Step 3: No commitear todavía**

Seguir a la Task 11.

---

## Task 11: Frontend — inversión del flujo en la página

**Files:**
- Modify: `frontend/src/app/(dashboard)/chatbot/page.tsx` (reescritura del encabezado de estado y el layout)

**Interfaces:**
- Consumes: `chatApi` (Task 9), `MonitorMultiSelect` (Task 10)

- [ ] **Step 1: Quitar el selector de monitor y el reset por cambio**

Borrar de `page.tsx`:

- El estado `const [monitorId, setMonitorId] = useState("")`.
- El bloque completo `const [prevMonitorId, setPrevMonitorId] = useState(monitorId)` con su `if (monitorId !== prevMonitorId) { ... }` (líneas ~38-45). Ese patrón existía para resetear el hilo al cambiar de monitor; con conversaciones que abarcan varios, no hay un evento "cambió el monitor".
- El `<div className="max-w-xs space-y-1.5">` con el `<Label>Monitor</Label>` y su `<select>`.
- El envoltorio condicional `{monitorId && ( ... )}` alrededor de la grilla: la grilla se muestra siempre.

Cambiar el efecto que carga conversaciones para que no dependa de `monitorId`:

```tsx
  useEffect(() => {
    chatApi
      .listConversations()
      .then(setConversations)
      .catch(() => toastError("Error al cargar conversaciones"));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
```

- [ ] **Step 2: Agregar el estado del diálogo de creación**

```tsx
  const [creating, setCreating] = useState(false);
  const [draftMonitorIds, setDraftMonitorIds] = useState<string[]>([]);

  /** Espejo del backend: models.MaxMonitorsPerConversation. */
  const MAX_MONITORS = 6;

  const activeConversation = conversations.find((c) => c.id === conversationId);

  function monitorNames(ids: string[]): string[] {
    return ids.map((id) => monitors.find((m) => m.id === id)?.name ?? id);
  }

  async function newConversation() {
    if (draftMonitorIds.length === 0) return;
    try {
      const conv = await chatApi.createConversation(draftMonitorIds);
      setConversations((prev) => [conv, ...prev]);
      setConversationId(conv.id);
      setMessages([]);
      setActiveArtifact(null);
      setCreating(false);
      setDraftMonitorIds([]);
    } catch {
      toastError("Error al crear la conversación");
    }
  }
```

Reemplazar también la creación implícita dentro de `send`: sin conversación activa ya no se puede inferir el monitor, así que `send` deja de crear una.

```tsx
  async function send(e: React.FormEvent) {
    e.preventDefault();
    if (!input.trim() || !conversationId) return;

    const question = input;
    setInput("");
    setMessages((prev) => [
      ...prev,
      {
        id: `local-${Date.now()}`,
        conversationId,
        role: "user",
        content: question,
        createdAt: new Date().toISOString(),
      },
    ]);
    setSending(true);
    try {
      const answer = await chatApi.ask(conversationId, question);
      setMessages((prev) => [...prev, answer]);
      if (answer.artifact) setActiveArtifact(answer.artifact);
    } catch {
      toastError("Error al consultar al chatbot");
    } finally {
      setSending(false);
    }
  }
```

- [ ] **Step 3: Renderizar el panel de creación, los chips y el estado vacío**

Reemplazar el botón «Nueva conversación» del sidebar por:

```tsx
              <Button
                variant="outline"
                size="sm"
                className="w-full justify-start gap-2"
                onClick={() => setCreating((v) => !v)}
              >
                <Plus className="h-4 w-4" /> Nueva conversación
              </Button>

              {creating && (
                <div className="space-y-2 rounded-md border p-2">
                  <MonitorMultiSelect
                    monitors={monitors}
                    selected={draftMonitorIds}
                    onChange={setDraftMonitorIds}
                    max={MAX_MONITORS}
                  />
                  <Button
                    size="sm"
                    className="w-full"
                    disabled={draftMonitorIds.length === 0}
                    onClick={newConversation}
                  >
                    Empezar
                  </Button>
                </div>
              )}
```

En cada fila de la lista de conversaciones, debajo del título, agregar los chips:

```tsx
                        <span className="mt-0.5 block truncate text-[11px] text-muted-foreground">
                          {monitorNames(c.monitorIds).join(" · ")}
                        </span>
```

Y en la cabecera del panel de chat, antes del `<CardContent>`:

```tsx
              {activeConversation && (
                <div className="flex flex-wrap gap-1.5 border-b px-4 py-2">
                  {monitorNames(activeConversation.monitorIds).map((name) => (
                    <span
                      key={name}
                      className="rounded-full bg-accent px-2 py-0.5 text-xs text-accent-foreground"
                    >
                      {name}
                    </span>
                  ))}
                </div>
              )}
```

Reemplazar el estado vacío del hilo para que refleje el flujo nuevo:

```tsx
                {messages.length === 0 && (
                  <p className="text-sm text-muted-foreground flex items-center gap-2">
                    <Bot className="h-4 w-4" />
                    {conversationId
                      ? "Preguntame algo sobre estos monitores."
                      : "Creá una conversación y elegí sobre qué monitores querés preguntar."}
                  </p>
                )}
```

Y deshabilitar el input sin conversación activa:

```tsx
                <Input
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                  placeholder="Preguntá algo sobre los datos de estos monitores"
                  disabled={sending || !conversationId}
                />
                <Button type="submit" disabled={sending || !input.trim() || !conversationId}>
```

Agregar el import: `import { MonitorMultiSelect } from "@/components/chat/monitor-multi-select";`

- [ ] **Step 4: Verificar tipos y lint**

Run: `cd frontend && npx tsc --noEmit && npm run lint`
Expected: sin errores.

- [ ] **Step 5: Verificar el build**

Run: `cd frontend && npm run build`
Expected: build exitoso.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/api/chat.ts frontend/src/components/chat/monitor-multi-select.tsx "frontend/src/app/(dashboard)/chatbot/page.tsx"
git commit -m "feat(chat): elegir varios monitores al crear una conversación"
```

---

## Task 12: Verificación de extremo a extremo

**Files:** ninguno (verificación manual)

- [ ] **Step 1: Levantar la infraestructura**

Run: `docker compose up -d && cd backend && go run cmd/server/main.go`
Expected: el servidor arranca en `:8080` sin errores de índices.

- [ ] **Step 2: Levantar el frontend**

Run: `cd frontend && npm run dev`

- [ ] **Step 3: Verificar la compatibilidad con conversaciones viejas**

Abrir `/chatbot`. Las conversaciones creadas antes de este cambio deben aparecer en la lista con **un** chip de monitor (el `monitor_id` legacy normalizado), y deben poder responder preguntas.

- [ ] **Step 4: Verificar el cruce**

Crear una conversación con dos monitores y preguntar algo que exija los dos (ej. «cuántos registros tiene cada monitor»). Verificar en los logs que hubo al menos una llamada a la herramienta por monitor, y que la respuesta menciona ambos.

- [ ] **Step 5: Verificar el tope**

Intentar crear una conversación con 7 monitores vía API directa:

```bash
curl -X POST localhost:8080/api/v1/chat/conversations \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"monitorIds":["...","...","...","...","...","...","..."]}'
```

Expected: `400` con «una conversación puede abarcar hasta 6 monitores».

- [ ] **Step 6: Correr la suite completa**

Run: `cd backend && go test ./... && golangci-lint run && cd ../frontend && npm run test:run && npm run lint`
Expected: todo verde.

---

## Auto-repaso del plan

**Cobertura del spec** — cada sección del spec tiene tarea asignada:

| Sección del spec | Tarea |
|---|---|
| Modelo de datos (`MonitorIDs`, `Normalize`, cota 6) | 2 |
| Índices | 3 |
| Alias de monitor | 1 |
| Contrato de la herramienta (campo `monitor`) | 4 |
| System prompt | 5 |
| **Autorización** | 1 (`resolveQueryMonitor`) + 6 (cableado en `executeQuery`) |
| Cotas de iteración y timeout | 6 |
| Historial y artefactos | 7 |
| API (3 endpoints) | 8 |
| Frontend (inversión del flujo) | 9, 10, 11 |
| Manejo de errores | 6 (monitor borrado), 8 (validaciones), 1 (alias desconocido) |
| Testing | integrado en cada tarea |

**Consistencia de tipos** — verificada de punta a punta: `buildMonitorAliases` devuelve `map[string]*models.Monitor`, y ese es el tipo que consumen `resolveQueryMonitor`, `buildSystemPrompt`, `executeQuery`, `askAnthropic` y `askDeepSeek`. `ChatQueryInput.Monitor` es `string` en la Task 4 y se consume como `string` en la Task 6.

**Orden de commits** — las tareas 2+3, 5+6 y 9+10+11 dejan el build roto entre medio y se commitean juntas. Está anotado en el paso final de cada una.
