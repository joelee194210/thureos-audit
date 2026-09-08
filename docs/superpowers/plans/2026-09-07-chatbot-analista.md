# Chatbot "Analista IA" Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Chatbot conversacional por monitor que responde con datos reales (nunca inventados) vía tool-calling contra el motor de reglas existente, genera artefactos visuales en un panel canvas separado, y los exporta.

**Architecture:** Subsistema nuevo. Reutiliza `AIConfig` (Anthropic/DeepSeek, ya en `system_config`) para el proveedor LLM, y `BuildMongoFilter`/`buildAggExpr`/`parseTimeWindow` del motor de reglas para ejecutar las consultas reales que el LLM pide vía una herramienta `query_monitor_data`. El LLM puede encadenar hasta 5 llamadas a la herramienta por pregunta antes de responder. La respuesta final puede incluir un artefacto (gráfico/tabla estructurados, o código HTML/JS que se renderiza aislado en un iframe sandboxeado para casos que un chart-spec fijo no puede expresar).

**Tech Stack:** Go/Fiber/MongoDB (backend, ya en el repo), Next.js/React/Recharts (frontend, ya en el repo). Sin dependencias nuevas — el export de PNG usa serialización de SVG nativa del navegador, sin librería adicional.

**Spec:** `docs/superpowers/specs/2026-09-07-chatbot-analista-design.md`

## Global Constraints

- Las respuestas del chatbot se basan SIEMPRE en datos reales devueltos por `query_monitor_data` — el LLM nunca debe inventar números; el system prompt lo exige explícitamente.
- Cada consulta tiene tope duro de filas (1000 para consultas simples, 200 grupos para agregaciones) y usa el `context.Context` con su propio timeout — nunca depende de lo que pida el LLM.
- Máximo 5 iteraciones de tool-calling por pregunta; al llegar al tope se le pide al LLM una respuesta final sin más herramientas.
- Las conversaciones son privadas por usuario — un usuario nunca puede leer ni escribirle a una conversación de otro usuario, incluso conociendo el ID.
- Código `"custom"` generado por el LLM se renderiza EXCLUSIVAMENTE en un `<iframe sandbox="allow-scripts">` sin `allow-same-origin`, sin `allow-top-navigation`, sin `allow-popups` — sin acceso a cookies, `localStorage`, el token de sesión, ni la red de la app.
- Todos los roles (admin, compliance, viewer) pueden usar el chatbot — es de solo lectura.
- Un artefacto mal formado (JSON roto, código vacío) nunca aborta la respuesta — se descarta el artefacto y se conserva el texto.
- Sin API key de IA configurada, el chatbot responde con un mensaje de chat explicando el problema — nunca un 500.

---

## Task 1: Modelo de datos

**Files:**
- Create: `backend/internal/models/chat.go`

**Interfaces:**
- Produces: `models.ChatRole` (const: `ChatRoleUser`, `ChatRoleAssistant`), `models.ChatArtifactType` (const: `ChatArtifactChart`, `ChatArtifactTable`, `ChatArtifactCustom`), `models.ChatChartType` (const: `ChatChartBar`, `ChatChartLine`, `ChatChartPie`), `models.ChartSpec{ChartType, Data, XKey, YKeys}`, `models.ChatArtifact{Type, Title, ChartSpec, Code}`, `models.ChatMessage{ID, ConversationID, Role, Content, Artifact, CreatedAt}`, `models.ChatConversation{ID, MonitorID, UserID, Title, CreatedAt, UpdatedAt}`.

- [ ] **Step 1: Crear `backend/internal/models/chat.go`**

```go
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ChatRole distingue quién escribió un ChatMessage.
type ChatRole string

const (
	ChatRoleUser      ChatRole = "user"
	ChatRoleAssistant ChatRole = "assistant"
)

// ChatArtifactType — qué clase de artefacto trae una respuesta del chatbot.
type ChatArtifactType string

const (
	ChatArtifactChart  ChatArtifactType = "chart"
	ChatArtifactTable  ChatArtifactType = "table"
	ChatArtifactCustom ChatArtifactType = "custom"
)

// ChatChartType — tipos de gráfico que el chart-spec estructurado soporta
// sin necesidad de código custom.
type ChatChartType string

const (
	ChatChartBar  ChatChartType = "bar"
	ChatChartLine ChatChartType = "line"
	ChatChartPie  ChatChartType = "pie"
)

// ChartSpec es la forma estructurada y segura de un artefacto — nunca
// ejecuta código, el frontend la renderiza directo con Recharts o como
// tabla. ChartType solo aplica cuando ChatArtifact.Type == "chart"; una
// tabla usa Data sin ChartType/XKey/YKeys.
type ChartSpec struct {
	ChartType ChatChartType            `bson:"chart_type,omitempty" json:"chartType,omitempty"`
	Data      []map[string]interface{} `bson:"data" json:"data"`
	XKey      string                   `bson:"x_key,omitempty" json:"xKey,omitempty"`
	YKeys     []string                 `bson:"y_keys,omitempty" json:"yKeys,omitempty"`
}

// ChatArtifact acompaña opcionalmente una respuesta del asistente. Para
// Type == "custom", Code es HTML/JS autocontenido que corre aislado en un
// iframe sandboxeado (ver Global Constraints) — nunca en el contexto de la
// app.
type ChatArtifact struct {
	Type      ChatArtifactType `bson:"type" json:"type"`
	Title     string           `bson:"title" json:"title"`
	ChartSpec *ChartSpec       `bson:"chart_spec,omitempty" json:"chartSpec,omitempty"`
	Code      string           `bson:"code,omitempty" json:"code,omitempty"`
}

type ChatMessage struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ConversationID primitive.ObjectID `bson:"conversation_id" json:"conversationId"`
	Role           ChatRole           `bson:"role" json:"role"`
	Content        string             `bson:"content" json:"content"`
	Artifact       *ChatArtifact      `bson:"artifact,omitempty" json:"artifact,omitempty"`
	CreatedAt      time.Time          `bson:"created_at" json:"createdAt"`
}

// ChatConversation es privada de UserID — ver Global Constraints.
type ChatConversation struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	MonitorID primitive.ObjectID `bson:"monitor_id" json:"monitorId"`
	UserID    primitive.ObjectID `bson:"user_id" json:"userId"`
	Title     string             `bson:"title" json:"title"`
	CreatedAt time.Time          `bson:"created_at" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updatedAt"`
}
```

- [ ] **Step 2: Verificar que compila**

Run: `cd backend && go build ./...`
Expected: sin errores.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/models/chat.go
git commit -m "feat(chatbot): modelo de datos de conversaciones y artefactos"
```

---

## Task 2: Constructor de pipeline de agregación sin threshold

**Files:**
- Create: `backend/internal/services/chat_query.go`
- Test: `backend/internal/services/chat_query_test.go`

**Interfaces:**
- Consumes: `models.AggFunction`, `models.Condition`, `models.ConditionGroup`, `models.LogicAND` (Tarea 1 no las toca, ya existen en `rule.go`); `BuildMongoFilter`, `buildAggExpr`, `parseTimeWindow` (funciones de paquete ya existentes en `rule_engine.go`, mismo paquete `services`).
- Produces: `ChatAggregateSpec{Field, Function, GroupBy, TimeField, TimeWindow, Filter}`, `buildDataAggregatePipeline(spec ChatAggregateSpec) mongo.Pipeline`, constante `chatAggregateResultLimit = 200`.

**Por qué un constructor nuevo y no reusar `buildAggregatePipeline`:** esa función (en `rule_engine.go`) hornea un `$match` de threshold después del `$group` — está pensada para el motor de reglas, que solo le interesan los grupos que superan un umbral. El chatbot quiere TODOS los grupos de una agregación (ej. "total por categoría"), así que el pipeline nuevo reusa las mismas piezas puras (`BuildMongoFilter`, `buildAggExpr`, `parseTimeWindow`) pero sin el paso de threshold.

- [ ] **Step 1: Escribir los tests (fallan primero)**

Crear `backend/internal/services/chat_query_test.go`:

```go
package services

import (
	"testing"

	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson"
)

func baseChatAggregateSpec() ChatAggregateSpec {
	return ChatAggregateSpec{
		Field:    "amount",
		Function: models.AggFuncSum,
		GroupBy:  "account",
	}
}

func TestBuildDataAggregatePipeline_SinFiltroNiVentana(t *testing.T) {
	pipeline := buildDataAggregatePipeline(baseChatAggregateSpec())
	assertStages(t, pipelineStageKeys(pipeline), []string{"$group", "$sort", "$limit"})
}

// A diferencia de buildAggregatePipeline, nunca hay un $match después del
// $group — no existe el concepto de threshold acá.
func TestBuildDataAggregatePipeline_NuncaFiltraPorThreshold(t *testing.T) {
	pipeline := buildDataAggregatePipeline(baseChatAggregateSpec())
	for i, stage := range pipeline {
		if i == 0 {
			continue // el primer stage puede ser el $match del Filter opcional
		}
		if stage[0].Key == "$match" {
			t.Fatalf("no debería haber un $match después del primer stage, encontrado en la posición %d: %v", i, stage)
		}
	}
}

func TestBuildDataAggregatePipeline_ConFiltroAgregaMatchAntesDelGroup(t *testing.T) {
	spec := baseChatAggregateSpec()
	spec.Filter = []models.Condition{
		{Field: "amount", Operator: models.OpGreaterEqual, Value: 1000.0},
		{Field: "amount", Operator: models.OpLessThan, Value: 10000.0},
	}
	pipeline := buildDataAggregatePipeline(spec)
	assertStages(t, pipelineStageKeys(pipeline), []string{"$match", "$group", "$sort", "$limit"})

	wantFilter := BuildMongoFilter(models.ConditionGroup{Logic: models.LogicAND, Conditions: spec.Filter})
	gotFilter := stageOperand(t, pipeline[0], "$match")
	wantBytes, _ := bson.Marshal(wantFilter)
	gotBytes, _ := bson.Marshal(gotFilter)
	if string(wantBytes) != string(gotBytes) {
		t.Errorf("contenido del $match no coincide con BuildMongoFilter:\n got=%v\nwant=%v", gotFilter, wantFilter)
	}
}

func TestBuildDataAggregatePipeline_ConVentanaDeTiempoAgregaMatchDeFecha(t *testing.T) {
	spec := baseChatAggregateSpec()
	spec.TimeField = "fecha"
	spec.TimeWindow = "30d"
	pipeline := buildDataAggregatePipeline(spec)
	assertStages(t, pipelineStageKeys(pipeline), []string{"$match", "$group", "$sort", "$limit"})

	matchValue := stageOperand(t, pipeline[0], "$match")
	if _, ok := matchValue["fecha"]; !ok {
		t.Errorf("el $match de ventana de tiempo debería filtrar por 'fecha', got %+v", matchValue)
	}
}

func TestBuildDataAggregatePipeline_SinGroupByAgrupaTodoJunto(t *testing.T) {
	spec := baseChatAggregateSpec()
	spec.GroupBy = ""
	pipeline := buildDataAggregatePipeline(spec)
	groupValue := stageOperand(t, pipeline[0], "$group")
	if groupValue["_id"] != nil {
		t.Errorf("_id del $group sin GroupBy debería ser nil, got %v", groupValue["_id"])
	}
}

func TestBuildDataAggregatePipeline_RespetaTopeDeGrupos(t *testing.T) {
	pipeline := buildDataAggregatePipeline(baseChatAggregateSpec())
	last := pipeline[len(pipeline)-1]
	if last[0].Key != "$limit" {
		t.Fatalf("el último stage debería ser $limit, got %q", last[0].Key)
	}
	if last[0].Value != chatAggregateResultLimit {
		t.Errorf("$limit = %v, want %d", last[0].Value, chatAggregateResultLimit)
	}
}
```

- [ ] **Step 2: Correr los tests, verificar que fallan**

Run: `cd backend && go test ./internal/services/... -run TestBuildDataAggregatePipeline -v`
Expected: FAIL — `buildDataAggregatePipeline`/`ChatAggregateSpec`/`chatAggregateResultLimit` no existen todavía.

- [ ] **Step 3: Crear `backend/internal/services/chat_query.go`**

```go
package services

import (
	"time"

	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// ChatAggregateSpec agrupa y agrega sin umbral — a diferencia de
// models.AggregateCondition (pensada para el motor de reglas, que corta
// por threshold), esta trae TODOS los grupos: el chatbot quiere el dato,
// no una decisión de match/no-match.
type ChatAggregateSpec struct {
	Field      string             `json:"field"`
	Function   models.AggFunction `json:"function"`
	GroupBy    string             `json:"groupBy,omitempty"`
	TimeField  string             `json:"timeField,omitempty"`
	TimeWindow string             `json:"timeWindow,omitempty"`
	Filter     []models.Condition `json:"filter,omitempty"`
}

// chatAggregateResultLimit topea cuántos grupos puede devolver una
// agregación del chatbot — evita que una pregunta sobre un campo de alta
// cardinalidad (ej. agrupar por ID de transacción) tire miles de grupos.
const chatAggregateResultLimit = 200

// buildDataAggregatePipeline arma el pipeline de agregación para el chat:
// mismos pasos que buildAggregatePipeline (ventana de tiempo, filtro
// previo, group) pero SIN el $match de threshold — el chatbot quiere
// todos los grupos, no solo los que superan un umbral.
func buildDataAggregatePipeline(spec ChatAggregateSpec) mongo.Pipeline {
	pipeline := mongo.Pipeline{}

	if spec.TimeField != "" && spec.TimeWindow != "" {
		dur := parseTimeWindow(spec.TimeWindow)
		if dur > 0 {
			cutoff := time.Now().Add(-dur)
			pipeline = append(pipeline, bson.D{
				{Key: "$match", Value: bson.D{
					{Key: spec.TimeField, Value: bson.D{
						{Key: "$gte", Value: cutoff.Format("01/02/2006 03:04 PM")},
					}},
				}},
			})
		}
	}

	if len(spec.Filter) > 0 {
		pipeline = append(pipeline, bson.D{
			{Key: "$match", Value: BuildMongoFilter(models.ConditionGroup{
				Logic: models.LogicAND, Conditions: spec.Filter,
			})},
		})
	}

	aggExpr := buildAggExpr(spec.Function, spec.Field)
	var groupID interface{}
	if spec.GroupBy != "" {
		groupID = "$" + spec.GroupBy
	}

	pipeline = append(pipeline, bson.D{
		{Key: "$group", Value: bson.D{
			{Key: "_id", Value: groupID},
			{Key: "aggValue", Value: aggExpr},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}},
	})

	pipeline = append(pipeline, bson.D{
		{Key: "$sort", Value: bson.D{{Key: "aggValue", Value: -1}}},
	})
	pipeline = append(pipeline, bson.D{
		{Key: "$limit", Value: chatAggregateResultLimit},
	})

	return pipeline
}
```

- [ ] **Step 4: Correr los tests, verificar que pasan**

Run: `cd backend && go test ./internal/services/... -run TestBuildDataAggregatePipeline -v`
Expected: PASS (6/6).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/chat_query.go backend/internal/services/chat_query_test.go
git commit -m "feat(chatbot): pipeline de agregación sin threshold para el chat"
```

---

## Task 3: Parseo y validación de la salida del LLM

**Files:**
- Create: `backend/internal/services/chat_llm_parsing.go`
- Test: `backend/internal/services/chat_llm_parsing_test.go`

**Interfaces:**
- Consumes: `models.ConditionGroup`, `models.ChatArtifact`, `models.ChatArtifactChart/Table/Custom`, `models.ChatChartBar/Line/Pie` (Tarea 1); `ChatAggregateSpec` (Tarea 2).
- Produces: `chatQueryMaxLimit = 1000`, `ChatQueryInput{ConditionGroup, Aggregate, Limit}`, `ParseQueryToolInput(raw json.RawMessage) (ChatQueryInput, error)`, `ParseArtifact(raw json.RawMessage) (*models.ChatArtifact, error)`.

- [ ] **Step 1: Escribir los tests (fallan primero)**

Crear `backend/internal/services/chat_llm_parsing_test.go`:

```go
package services

import (
	"encoding/json"
	"testing"

	"github.com/thureos/compliance/internal/models"
)

func TestParseQueryToolInput_ConConditionGroupValido(t *testing.T) {
	raw := json.RawMessage(`{"conditionGroup":{"logic":"AND","conditions":[{"field":"amount","operator":"gt","value":1000}]}}`)
	input, err := ParseQueryToolInput(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if input.ConditionGroup == nil || len(input.ConditionGroup.Conditions) != 1 {
		t.Fatalf("conditionGroup no se parseó correctamente: %+v", input)
	}
	if input.Limit != chatQueryMaxLimit {
		t.Errorf("Limit sin especificar debería quedar en el tope %d, got %d", chatQueryMaxLimit, input.Limit)
	}
}

func TestParseQueryToolInput_ConAggregateValido(t *testing.T) {
	raw := json.RawMessage(`{"aggregate":{"field":"amount","function":"sum","groupBy":"account"}}`)
	input, err := ParseQueryToolInput(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if input.Aggregate == nil || input.Aggregate.Field != "amount" {
		t.Fatalf("aggregate no se parseó correctamente: %+v", input)
	}
}

func TestParseQueryToolInput_SinConditionGroupNiAggregateFalla(t *testing.T) {
	raw := json.RawMessage(`{}`)
	if _, err := ParseQueryToolInput(raw); err == nil {
		t.Fatal("esperaba error cuando no hay conditionGroup ni aggregate")
	}
}

func TestParseQueryToolInput_AggregateSinFieldFalla(t *testing.T) {
	raw := json.RawMessage(`{"aggregate":{"function":"sum"}}`)
	if _, err := ParseQueryToolInput(raw); err == nil {
		t.Fatal("esperaba error cuando aggregate.field está vacío")
	}
}

func TestParseQueryToolInput_LimitFueraDeRangoSeAjustaAlTope(t *testing.T) {
	raw := json.RawMessage(`{"conditionGroup":{"logic":"AND","conditions":[]},"limit":999999}`)
	input, err := ParseQueryToolInput(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if input.Limit != chatQueryMaxLimit {
		t.Errorf("Limit fuera de rango debería ajustarse a %d, got %d", chatQueryMaxLimit, input.Limit)
	}
}

func TestParseQueryToolInput_JSONInvalidoFalla(t *testing.T) {
	if _, err := ParseQueryToolInput(json.RawMessage(`{not json`)); err == nil {
		t.Fatal("esperaba error con JSON inválido")
	}
}

func TestParseArtifact_VacioDevuelveNilSinError(t *testing.T) {
	artifact, err := ParseArtifact(nil)
	if err != nil || artifact != nil {
		t.Fatalf("esperaba (nil, nil) para input vacío, got (%v, %v)", artifact, err)
	}
}

func TestParseArtifact_ChartValido(t *testing.T) {
	raw := json.RawMessage(`{"type":"chart","title":"Ventas por mes","chartSpec":{"chartType":"bar","data":[{"mes":"enero","total":100}],"xKey":"mes","yKeys":["total"]}}`)
	artifact, err := ParseArtifact(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if artifact.Type != models.ChatArtifactChart || artifact.ChartSpec.ChartType != models.ChatChartBar {
		t.Fatalf("artifact no se parseó correctamente: %+v", artifact)
	}
}

func TestParseArtifact_ChartSinDataFalla(t *testing.T) {
	raw := json.RawMessage(`{"type":"chart","chartSpec":{"chartType":"bar","data":[]}}`)
	if _, err := ParseArtifact(raw); err == nil {
		t.Fatal("esperaba error cuando chartSpec.data está vacío")
	}
}

func TestParseArtifact_ChartTypeInvalidoFalla(t *testing.T) {
	raw := json.RawMessage(`{"type":"chart","chartSpec":{"chartType":"scatter3d","data":[{"a":1}]}}`)
	if _, err := ParseArtifact(raw); err == nil {
		t.Fatal("esperaba error con chartType desconocido")
	}
}

func TestParseArtifact_TableValida(t *testing.T) {
	raw := json.RawMessage(`{"type":"table","title":"Detalle","chartSpec":{"data":[{"cliente":"Juan","total":500}]}}`)
	artifact, err := ParseArtifact(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if artifact.Type != models.ChatArtifactTable {
		t.Fatalf("Type = %q, want table", artifact.Type)
	}
}

func TestParseArtifact_CustomValido(t *testing.T) {
	raw := json.RawMessage(`{"type":"custom","title":"Mapa de calor","code":"<html><body>hola</body></html>"}`)
	artifact, err := ParseArtifact(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if artifact.Code == "" {
		t.Fatal("Code no debería quedar vacío")
	}
}

func TestParseArtifact_CustomSinCodeFalla(t *testing.T) {
	raw := json.RawMessage(`{"type":"custom","title":"Mapa de calor"}`)
	if _, err := ParseArtifact(raw); err == nil {
		t.Fatal("esperaba error cuando custom no trae code")
	}
}

func TestParseArtifact_TipoDesconocidoFalla(t *testing.T) {
	raw := json.RawMessage(`{"type":"video"}`)
	if _, err := ParseArtifact(raw); err == nil {
		t.Fatal("esperaba error con type desconocido")
	}
}

func TestParseArtifact_JSONInvalidoFalla(t *testing.T) {
	if _, err := ParseArtifact(json.RawMessage(`{not json`)); err == nil {
		t.Fatal("esperaba error con JSON inválido")
	}
}
```

- [ ] **Step 2: Correr los tests, verificar que fallan**

Run: `cd backend && go test ./internal/services/... -run 'TestParseQueryToolInput|TestParseArtifact' -v`
Expected: FAIL — nada de esto existe todavía.

- [ ] **Step 3: Crear `backend/internal/services/chat_llm_parsing.go`**

```go
package services

import (
	"encoding/json"
	"fmt"

	"github.com/thureos/compliance/internal/models"
)

// chatQueryMaxLimit topea cuántas filas puede pedir una consulta simple
// (no agregada) del chatbot — nunca se le permite pedir todo el dataset.
const chatQueryMaxLimit = 1000

// ChatQueryInput es la forma exacta del "input" que el LLM manda al llamar
// a la herramienta query_monitor_data (ver el contrato en chat_service.go).
type ChatQueryInput struct {
	ConditionGroup *models.ConditionGroup `json:"conditionGroup,omitempty"`
	Aggregate      *ChatAggregateSpec     `json:"aggregate,omitempty"`
	Limit          int                    `json:"limit,omitempty"`
}

// ParseQueryToolInput valida y normaliza el input crudo del tool call.
// Nunca deja pasar un Limit fuera de rango ni un input sin conditionGroup
// ni aggregate.
func ParseQueryToolInput(raw json.RawMessage) (ChatQueryInput, error) {
	var input ChatQueryInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return ChatQueryInput{}, fmt.Errorf("input de query_monitor_data inválido: %w", err)
	}
	if input.ConditionGroup == nil && input.Aggregate == nil {
		return ChatQueryInput{}, fmt.Errorf("query_monitor_data requiere conditionGroup o aggregate")
	}
	if input.Aggregate != nil && input.Aggregate.Field == "" {
		return ChatQueryInput{}, fmt.Errorf("aggregate.field es obligatorio")
	}
	if input.Limit <= 0 || input.Limit > chatQueryMaxLimit {
		input.Limit = chatQueryMaxLimit
	}
	return input, nil
}

// ParseArtifact valida el bloque de artefacto que el LLM devuelve junto a
// su respuesta final. Un artefacto inválido nunca aborta la respuesta —
// el llamador (chat_service.go) trata un error acá como "sin artefacto",
// nunca como falla de toda la pregunta.
func ParseArtifact(raw json.RawMessage) (*models.ChatArtifact, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var artifact models.ChatArtifact
	if err := json.Unmarshal(raw, &artifact); err != nil {
		return nil, fmt.Errorf("artefacto inválido: %w", err)
	}
	switch artifact.Type {
	case models.ChatArtifactChart:
		if artifact.ChartSpec == nil || len(artifact.ChartSpec.Data) == 0 {
			return nil, fmt.Errorf("artefacto chart requiere chartSpec con datos")
		}
		switch artifact.ChartSpec.ChartType {
		case models.ChatChartBar, models.ChatChartLine, models.ChatChartPie:
		default:
			return nil, fmt.Errorf("chartType inválido: %q", artifact.ChartSpec.ChartType)
		}
	case models.ChatArtifactTable:
		if artifact.ChartSpec == nil || len(artifact.ChartSpec.Data) == 0 {
			return nil, fmt.Errorf("artefacto table requiere chartSpec con datos")
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

- [ ] **Step 4: Correr los tests, verificar que pasan**

Run: `cd backend && go test ./internal/services/... -run 'TestParseQueryToolInput|TestParseArtifact' -v`
Expected: PASS (15/15).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/chat_llm_parsing.go backend/internal/services/chat_llm_parsing_test.go
git commit -m "feat(chatbot): parseo y validación de tool-calls y artefactos del LLM"
```

---

## Task 4: Repositorio de conversaciones y mensajes

**Files:**
- Create: `backend/internal/repository/chat_repo.go`

**Interfaces:**
- Consumes: `models.ChatConversation`, `models.ChatMessage` (Tarea 1).
- Produces: `ChatRepository`, `NewChatRepository(db *database.MongoDB) *ChatRepository`, `CreateConversation(ctx, *models.ChatConversation) error`, `GetConversation(ctx, id primitive.ObjectID) (*models.ChatConversation, error)`, `ListConversationsByMonitorAndUser(ctx, monitorID, userID primitive.ObjectID) ([]models.ChatConversation, error)`, `TouchConversation(ctx, id primitive.ObjectID) error`, `SetTitle(ctx, id primitive.ObjectID, title string) error`, `CreateMessage(ctx, *models.ChatMessage) error`, `ListMessagesByConversation(ctx, conversationID primitive.ObjectID) ([]models.ChatMessage, error)`.

Sin tests (capa Mongo — mismo criterio que `screening_repo.go` y el resto de los repositorios del proyecto; se verifica manualmente en la Tarea 8).

- [ ] **Step 1: Crear `backend/internal/repository/chat_repo.go`**

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

type ChatRepository struct {
	conversations *mongo.Collection
	messages      *mongo.Collection
}

func NewChatRepository(db *database.MongoDB) *ChatRepository {
	r := &ChatRepository{
		conversations: db.Collection("chat_conversations"),
		messages:      db.Collection("chat_messages"),
	}
	r.ensureIndexes()
	return r
}

func (r *ChatRepository) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _ = r.conversations.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "monitor_id", Value: 1}, {Key: "user_id", Value: 1}, {Key: "updated_at", Value: -1}},
	})
	_, _ = r.messages.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "conversation_id", Value: 1}, {Key: "created_at", Value: 1}},
	})
}

func (r *ChatRepository) CreateConversation(ctx context.Context, conv *models.ChatConversation) error {
	conv.CreatedAt = time.Now()
	conv.UpdatedAt = conv.CreatedAt
	res, err := r.conversations.InsertOne(ctx, conv)
	if err != nil {
		return fmt.Errorf("creando conversación: %w", err)
	}
	conv.ID = res.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *ChatRepository) GetConversation(ctx context.Context, id primitive.ObjectID) (*models.ChatConversation, error) {
	var conv models.ChatConversation
	if err := r.conversations.FindOne(ctx, bson.M{"_id": id}).Decode(&conv); err != nil {
		return nil, fmt.Errorf("conversación %s no encontrada: %w", id.Hex(), err)
	}
	return &conv, nil
}

func (r *ChatRepository) ListConversationsByMonitorAndUser(ctx context.Context, monitorID, userID primitive.ObjectID) ([]models.ChatConversation, error) {
	cursor, err := r.conversations.Find(ctx,
		bson.M{"monitor_id": monitorID, "user_id": userID},
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
	return results, nil
}

// TouchConversation actualiza updated_at — se llama después de cada
// intercambio para que ListConversationsByMonitorAndUser ordene por
// actividad reciente, no por fecha de creación.
func (r *ChatRepository) TouchConversation(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.conversations.UpdateByID(ctx, id, bson.M{"$set": bson.M{"updated_at": time.Now()}})
	if err != nil {
		return fmt.Errorf("actualizando conversación %s: %w", id.Hex(), err)
	}
	return nil
}

// SetTitle reemplaza el título — chat_service.go la llama una sola vez,
// después de la primera pregunta de la conversación (ver Ask en la Tarea 5).
func (r *ChatRepository) SetTitle(ctx context.Context, id primitive.ObjectID, title string) error {
	_, err := r.conversations.UpdateByID(ctx, id, bson.M{"$set": bson.M{"title": title}})
	if err != nil {
		return fmt.Errorf("renombrando conversación %s: %w", id.Hex(), err)
	}
	return nil
}

func (r *ChatRepository) CreateMessage(ctx context.Context, msg *models.ChatMessage) error {
	msg.CreatedAt = time.Now()
	res, err := r.messages.InsertOne(ctx, msg)
	if err != nil {
		return fmt.Errorf("creando mensaje: %w", err)
	}
	msg.ID = res.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *ChatRepository) ListMessagesByConversation(ctx context.Context, conversationID primitive.ObjectID) ([]models.ChatMessage, error) {
	cursor, err := r.messages.Find(ctx,
		bson.M{"conversation_id": conversationID},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("listando mensajes: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	results := []models.ChatMessage{}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("decodificando mensajes: %w", err)
	}
	return results, nil
}
```

- [ ] **Step 2: Verificar que compila**

Run: `cd backend && go build ./...`
Expected: sin errores.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/repository/chat_repo.go
git commit -m "feat(chatbot): repositorio de conversaciones y mensajes"
```

---

## Task 5: ChatService — orquestación del tool-calling

**Files:**
- Create: `backend/internal/services/chat_service.go`

**Interfaces:**
- Consumes: `repository.ChatRepository` (Tarea 4), `repository.MonitorRepository.{FindByID, QueryData, AggregateData}` (ya existentes), `repository.SystemConfigRepository.Get` (ya existente), `models.AIConfig`/`models.AIProviderAnthropic`/`models.AIProviderDeepSeek`/`models.DefaultAIBaseURLs` (ya existentes), `BuildMongoFilter` (`rule_engine.go`), `buildDataAggregatePipeline`/`ChatAggregateSpec` (Tarea 2), `ParseQueryToolInput`/`ParseArtifact`/`chatQueryMaxLimit` (Tarea 3), `models.ChatMessage`/`ChatConversation`/`ChatRole*`/`ChatArtifact` (Tarea 1).
- Produces: `ChatService`, `NewChatService(chatRepo *repository.ChatRepository, monitorRepo *repository.MonitorRepository, configRepo *repository.SystemConfigRepository) *ChatService`, `(s *ChatService) Ask(ctx context.Context, conversationID, userID primitive.ObjectID, userMessage string) (models.ChatMessage, error)`, `chatConversationTitle(firstMessage string) string` (título autogenerado, llama a `chatRepo.SetTitle` de la Tarea 4 en la primera pregunta de cada conversación).

Sin tests unitarios (capa Mongo + llamadas HTTP a proveedores externos — mismo criterio que `AIRulesService`/`ScreeningService`). Se verifica manualmente en la Tarea 8 (checklist al final del plan).

- [ ] **Step 1: Crear `backend/internal/services/chat_service.go`**

```go
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// chatMaxToolIterations acota cuántas veces el LLM puede llamar a
// query_monitor_data para UNA pregunta — evita un loop infinito si el
// proveedor insiste en pedir herramientas sin nunca responder.
const chatMaxToolIterations = 5

// queryToolDescription y queryToolJSONSchema definen el contrato de la
// única herramienta que el chatbot expone — el mismo texto/schema se usa
// para Anthropic y DeepSeek (formas de request distintas, mismo contrato).
const queryToolDescription = `Ejecuta una consulta REAL contra los datos del monitor actual. Usar SIEMPRE que la pregunta necesite datos reales (conteos, sumas, promedios, filtros, listados) — nunca inventar números. Se puede llamar varias veces si hace falta investigar en pasos (ej. primero contar, después agrupar por región si el total es alto).

Para traer filas usar "conditionGroup" (opcional "limit", tope 1000). Para totales agrupados (suma/conteo/promedio/mín/máx por categoría) usar "aggregate" — SIEMPRE devuelve TODOS los grupos, no hace falta pedir un umbral.`

var queryToolJSONSchema = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"conditionGroup": map[string]interface{}{
			"type":        "object",
			"description": "Filtro simple para traer filas. Omitir si se usa 'aggregate'.",
			"properties": map[string]interface{}{
				"logic": map[string]interface{}{"type": "string", "enum": []string{"AND", "OR"}},
				"conditions": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"field":    map[string]interface{}{"type": "string"},
							"operator": map[string]interface{}{"type": "string", "enum": []string{"eq", "neq", "gt", "lt", "gte", "lte", "contains", "regex", "in", "not_in", "between", "is_null", "is_not_null", "starts_with", "ends_with"}},
							"value":    map[string]interface{}{},
						},
						"required": []string{"field", "operator", "value"},
					},
				},
			},
		},
		"aggregate": map[string]interface{}{
			"type":        "object",
			"description": "Agregación agrupada. Devuelve TODOS los grupos, sin importar su valor.",
			"properties": map[string]interface{}{
				"field":      map[string]interface{}{"type": "string"},
				"function":   map[string]interface{}{"type": "string", "enum": []string{"sum", "count", "avg", "min", "max"}},
				"groupBy":    map[string]interface{}{"type": "string"},
				"timeField":  map[string]interface{}{"type": "string"},
				"timeWindow": map[string]interface{}{"type": "string", "description": "ej. '24h', '7d', '30d'"},
			},
			"required": []string{"field", "function"},
		},
		"limit": map[string]interface{}{
			"type":        "integer",
			"description": "Tope de filas para conditionGroup (máx 1000).",
		},
	},
}

var artifactBlockRe = regexp.MustCompile(`(?s)<artifact>\s*(.*?)\s*</artifact>`)

const chatSystemPromptTemplate = `Sos un analista de datos que responde preguntas sobre el monitor %q.

Schema de los datos disponibles (nombre de campo: tipo):
%s

Tenés una herramienta, query_monitor_data, para traer datos REALES — nunca
inventes números. Usala todas las veces que necesites para responder bien.

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

func buildSystemPrompt(monitor *models.Monitor) string {
	var schemaLines strings.Builder
	for _, f := range monitor.Schema {
		fmt.Fprintf(&schemaLines, "- %s: %s\n", f.Name, f.Type)
	}
	return fmt.Sprintf(chatSystemPromptTemplate, monitor.Name, schemaLines.String())
}

// chatConversationTitleMaxLen topea el título autogenerado — el primer
// mensaje del usuario puede ser largo, el título es solo para la lista.
const chatConversationTitleMaxLen = 60

// chatConversationTitle arma el título de una conversación a partir de su
// primera pregunta — trunca sin cortar una palabra a la mitad cuando es
// posible.
func chatConversationTitle(firstMessage string) string {
	trimmed := strings.TrimSpace(firstMessage)
	if len(trimmed) <= chatConversationTitleMaxLen {
		return trimmed
	}
	cut := trimmed[:chatConversationTitleMaxLen]
	if idx := strings.LastIndex(cut, " "); idx > 0 {
		cut = cut[:idx]
	}
	return cut + "…"
}

// extractArtifactAndText separa el bloque <artifact> (si lo hay) del texto
// de la respuesta. Un artefacto que no parsea se descarta sin abortar el
// texto — ver Global Constraints.
func extractArtifactAndText(raw string) (string, *models.ChatArtifact) {
	match := artifactBlockRe.FindStringSubmatch(raw)
	if match == nil {
		return strings.TrimSpace(raw), nil
	}
	text := strings.TrimSpace(artifactBlockRe.ReplaceAllString(raw, ""))
	artifact, err := ParseArtifact(json.RawMessage(match[1]))
	if err != nil {
		return text, nil
	}
	return text, artifact
}

type ChatService struct {
	chatRepo    *repository.ChatRepository
	monitorRepo *repository.MonitorRepository
	configRepo  *repository.SystemConfigRepository
}

func NewChatService(chatRepo *repository.ChatRepository, monitorRepo *repository.MonitorRepository, configRepo *repository.SystemConfigRepository) *ChatService {
	return &ChatService{chatRepo: chatRepo, monitorRepo: monitorRepo, configRepo: configRepo}
}

// executeQuery ejecuta lo que el LLM pidió vía query_monitor_data contra
// los datos reales del monitor y devuelve el resultado serializado — lo
// que se le manda de vuelta al LLM como resultado de la herramienta.
func (s *ChatService) executeQuery(ctx context.Context, monitor *models.Monitor, raw json.RawMessage) (string, error) {
	input, err := ParseQueryToolInput(raw)
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

// Ask procesa una pregunta del usuario: la persiste, arma/loopea la
// llamada al proveedor configurado, y persiste + devuelve la respuesta.
// conversation.UserID != userID nunca debe ocurrir salvo que alguien
// intente leer la conversación de otro usuario — se trata como "no
// encontrada", no se filtra la existencia (ver Global Constraints).
func (s *ChatService) Ask(ctx context.Context, conversationID, userID primitive.ObjectID, userMessage string) (models.ChatMessage, error) {
	conv, err := s.chatRepo.GetConversation(ctx, conversationID)
	if err != nil {
		return models.ChatMessage{}, fmt.Errorf("cargando conversación: %w", err)
	}
	if conv.UserID != userID {
		return models.ChatMessage{}, fmt.Errorf("conversación no encontrada")
	}

	monitor, err := s.monitorRepo.FindByID(ctx, conv.MonitorID)
	if err != nil {
		return models.ChatMessage{}, fmt.Errorf("cargando monitor: %w", err)
	}

	history, err := s.chatRepo.ListMessagesByConversation(ctx, conversationID)
	if err != nil {
		return models.ChatMessage{}, fmt.Errorf("cargando historial: %w", err)
	}

	isFirstMessage := len(history) == 0

	userMsg := models.ChatMessage{ConversationID: conversationID, Role: models.ChatRoleUser, Content: userMessage}
	if err := s.chatRepo.CreateMessage(ctx, &userMsg); err != nil {
		return models.ChatMessage{}, fmt.Errorf("guardando mensaje: %w", err)
	}
	history = append(history, userMsg)

	// El título se autogenera una sola vez, a partir de la primera
	// pregunta — no hay UI de renombrado (no se pidió, ver el spec).
	if isFirstMessage {
		_ = s.chatRepo.SetTitle(ctx, conversationID, chatConversationTitle(userMessage))
	}

	cfg, err := s.configRepo.Get(ctx)
	if err != nil {
		return models.ChatMessage{}, fmt.Errorf("cargando config de IA: %w", err)
	}
	if cfg.AI.APIKey == "" {
		assistantMsg := models.ChatMessage{
			ConversationID: conversationID,
			Role:           models.ChatRoleAssistant,
			Content:        "Configurá una API key de IA en Configuración → APIs para poder usar el chatbot.",
		}
		if err := s.chatRepo.CreateMessage(ctx, &assistantMsg); err != nil {
			return models.ChatMessage{}, fmt.Errorf("guardando respuesta: %w", err)
		}
		return assistantMsg, nil
	}

	var text string
	var artifact *models.ChatArtifact
	if cfg.AI.Provider == models.AIProviderDeepSeek {
		text, artifact, err = s.askDeepSeek(ctx, cfg.AI, monitor, historyToDeepSeek(history))
	} else {
		text, artifact, err = s.askAnthropic(ctx, cfg.AI, monitor, historyToAnthropic(history))
	}
	if err != nil {
		assistantMsg := models.ChatMessage{
			ConversationID: conversationID,
			Role:           models.ChatRoleAssistant,
			Content:        fmt.Sprintf("Hubo un error consultando al proveedor de IA: %s", err.Error()),
		}
		if err := s.chatRepo.CreateMessage(ctx, &assistantMsg); err != nil {
			return models.ChatMessage{}, fmt.Errorf("guardando respuesta: %w", err)
		}
		return assistantMsg, nil
	}

	assistantMsg := models.ChatMessage{
		ConversationID: conversationID,
		Role:           models.ChatRoleAssistant,
		Content:        text,
		Artifact:       artifact,
	}
	if err := s.chatRepo.CreateMessage(ctx, &assistantMsg); err != nil {
		return models.ChatMessage{}, fmt.Errorf("guardando respuesta: %w", err)
	}
	_ = s.chatRepo.TouchConversation(ctx, conversationID)
	return assistantMsg, nil
}

// ---------------------------------------------------------------------------
// Anthropic
// ---------------------------------------------------------------------------

func historyToAnthropic(history []models.ChatMessage) []anthropic.MessageParam {
	messages := make([]anthropic.MessageParam, 0, len(history))
	for _, m := range history {
		block := anthropic.NewTextBlock(m.Content)
		if m.Role == models.ChatRoleAssistant {
			messages = append(messages, anthropic.NewAssistantMessage(block))
		} else {
			messages = append(messages, anthropic.NewUserMessage(block))
		}
	}
	return messages
}

func (s *ChatService) askAnthropic(ctx context.Context, ai models.AIConfig, monitor *models.Monitor, history []anthropic.MessageParam) (string, *models.ChatArtifact, error) {
	client := anthropic.NewClient(option.WithAPIKey(ai.APIKey))
	model := ai.Model
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}

	tool := anthropic.ToolUnionParam{OfTool: &anthropic.ToolParam{
		Name:        "query_monitor_data",
		Description: param.NewOpt(queryToolDescription),
		InputSchema: anthropic.ToolInputSchemaParam{
			Type:       "object",
			Properties: queryToolJSONSchema["properties"],
		},
	}}

	systemPrompt := buildSystemPrompt(monitor)
	messages := append([]anthropic.MessageParam{}, history...)

	for i := 0; i < chatMaxToolIterations; i++ {
		message, err := client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     model,
			MaxTokens: 4096,
			System:    []anthropic.TextBlockParam{{Text: systemPrompt, Type: "text"}},
			Messages:  messages,
			Tools:     []anthropic.ToolUnionParam{tool},
		})
		if err != nil {
			return "", nil, fmt.Errorf("calling Anthropic API: %w", err)
		}

		if message.StopReason != anthropic.MessageStopReasonToolUse {
			return extractAnthropicFinalAnswer(message)
		}

		assistantBlocks := make([]anthropic.ContentBlockParamUnion, 0, len(message.Content))
		var toolResults []anthropic.ContentBlockParamUnion
		for _, block := range message.Content {
			assistantBlocks = append(assistantBlocks, block.ToParam())
			if block.Type == "tool_use" {
				result, err := s.executeQuery(ctx, monitor, block.Input)
				content := result
				isError := false
				if err != nil {
					content = err.Error()
					isError = true
				}
				toolResults = append(toolResults, anthropic.NewToolResultBlock(block.ID, content, isError))
			}
		}
		messages = append(messages, anthropic.NewAssistantMessage(assistantBlocks...))
		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}

	return "", nil, fmt.Errorf("se alcanzó el máximo de %d consultas sin una respuesta final", chatMaxToolIterations)
}

func extractAnthropicFinalAnswer(message *anthropic.Message) (string, *models.ChatArtifact, error) {
	var sb strings.Builder
	for _, block := range message.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}
	if sb.Len() == 0 {
		return "", nil, fmt.Errorf("respuesta vacía del proveedor")
	}
	text, artifact := extractArtifactAndText(sb.String())
	return text, artifact, nil
}

// ---------------------------------------------------------------------------
// DeepSeek — API compatible con OpenAI, tipos propios del chat (no se
// tocan los de ai_rules_service.go, que no necesitan tool-calling).
// ---------------------------------------------------------------------------

type chatDeepSeekToolFunc struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type chatDeepSeekTool struct {
	Type     string               `json:"type"`
	Function chatDeepSeekToolFunc `json:"function"`
}

type chatDeepSeekToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatDeepSeekToolCall struct {
	ID       string                   `json:"id"`
	Type     string                   `json:"type"`
	Function chatDeepSeekToolCallFunc `json:"function"`
}

type chatDeepSeekMessage struct {
	Role       string                  `json:"role"`
	Content    string                  `json:"content,omitempty"`
	ToolCalls  []chatDeepSeekToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string                  `json:"tool_call_id,omitempty"`
}

type chatDeepSeekRequest struct {
	Model     string                 `json:"model"`
	Messages  []chatDeepSeekMessage  `json:"messages"`
	Tools     []chatDeepSeekTool     `json:"tools,omitempty"`
	MaxTokens int                    `json:"max_tokens"`
}

type chatDeepSeekResponse struct {
	Choices []struct {
		Message      chatDeepSeekMessage `json:"message"`
		FinishReason string              `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func historyToDeepSeek(history []models.ChatMessage) []chatDeepSeekMessage {
	messages := make([]chatDeepSeekMessage, 0, len(history))
	for _, m := range history {
		messages = append(messages, chatDeepSeekMessage{Role: string(m.Role), Content: m.Content})
	}
	return messages
}

func (s *ChatService) askDeepSeek(ctx context.Context, ai models.AIConfig, monitor *models.Monitor, history []chatDeepSeekMessage) (string, *models.ChatArtifact, error) {
	baseURL := ai.BaseURL
	if baseURL == "" {
		baseURL = models.DefaultAIBaseURLs[models.AIProviderDeepSeek]
	}
	model := ai.Model
	if model == "" {
		model = "deepseek-chat"
	}

	tool := chatDeepSeekTool{
		Type: "function",
		Function: chatDeepSeekToolFunc{
			Name:        "query_monitor_data",
			Description: queryToolDescription,
			Parameters:  queryToolJSONSchema,
		},
	}

	messages := append([]chatDeepSeekMessage{
		{Role: "system", Content: buildSystemPrompt(monitor)},
	}, history...)

	for i := 0; i < chatMaxToolIterations; i++ {
		reqBody := chatDeepSeekRequest{Model: model, Messages: messages, Tools: []chatDeepSeekTool{tool}, MaxTokens: 4096}
		body, err := json.Marshal(reqBody)
		if err != nil {
			return "", nil, fmt.Errorf("armando la petición a DeepSeek: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			return "", nil, fmt.Errorf("armando la petición a DeepSeek: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+ai.APIKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", nil, fmt.Errorf("llamando a DeepSeek: %w", err)
		}
		var parsed chatDeepSeekResponse
		decodeErr := json.NewDecoder(resp.Body).Decode(&parsed)
		_ = resp.Body.Close()
		if decodeErr != nil {
			return "", nil, fmt.Errorf("la respuesta de DeepSeek no es JSON válido: %w", decodeErr)
		}
		if parsed.Error != nil {
			return "", nil, fmt.Errorf("deepseek: %s", parsed.Error.Message)
		}
		if len(parsed.Choices) == 0 {
			return "", nil, fmt.Errorf("respuesta vacía de deepseek")
		}
		choice := parsed.Choices[0]

		if choice.FinishReason != "tool_calls" || len(choice.Message.ToolCalls) == 0 {
			text, artifact := extractArtifactAndText(choice.Message.Content)
			if text == "" {
				return "", nil, fmt.Errorf("respuesta vacía del proveedor")
			}
			return text, artifact, nil
		}

		messages = append(messages, choice.Message)
		for _, call := range choice.Message.ToolCalls {
			result, err := s.executeQuery(ctx, monitor, json.RawMessage(call.Function.Arguments))
			content := result
			if err != nil {
				content = err.Error()
			}
			messages = append(messages, chatDeepSeekMessage{
				Role:       "tool",
				Content:    content,
				ToolCallID: call.ID,
			})
		}
	}

	return "", nil, fmt.Errorf("se alcanzó el máximo de %d consultas sin una respuesta final", chatMaxToolIterations)
}
```

- [ ] **Step 2: Verificar que compila**

Run: `cd backend && go build ./... && go vet ./...`
Expected: sin errores.

- [ ] **Step 3: Correr todos los tests del paquete (regresión)**

Run: `cd backend && go test ./internal/services/... -v`
Expected: PASS — nada de lo existente se rompe.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/services/chat_service.go
git commit -m "feat(chatbot): ChatService con loop de tool-calling (Anthropic + DeepSeek)"
```

---

## Task 6: Handlers, rutas y wiring en main.go

**Files:**
- Create: `backend/internal/handlers/chat_handler.go`
- Modify: `backend/internal/router/router.go`
- Modify: `backend/cmd/server/main.go`

**Interfaces:**
- Consumes: `repository.ChatRepository` (Tarea 4), `services.ChatService.Ask` (Tarea 5), `models.ChatConversation`/`ChatMessage` (Tarea 1).
- Produces: `ChatHandler`, `NewChatHandler(chatRepo *repository.ChatRepository, chatService *services.ChatService) *ChatHandler`, rutas `POST /chat/conversations`, `GET /chat/conversations`, `GET /chat/conversations/:id/messages`, `POST /chat/conversations/:id/messages`.

- [ ] **Step 1: Crear `backend/internal/handlers/chat_handler.go`**

```go
package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
	"github.com/thureos/compliance/internal/services"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ChatHandler struct {
	chatRepo    *repository.ChatRepository
	chatService *services.ChatService
}

func NewChatHandler(chatRepo *repository.ChatRepository, chatService *services.ChatService) *ChatHandler {
	return &ChatHandler{chatRepo: chatRepo, chatService: chatService}
}

// CreateConversation arranca una conversación nueva sobre un monitor —
// privada del usuario autenticado (ver Global Constraints del plan).
func (h *ChatHandler) CreateConversation(c *fiber.Ctx) error {
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

	conv := models.ChatConversation{MonitorID: monitorID, UserID: userID, Title: "Nueva conversación"}
	if err := h.chatRepo.CreateConversation(c.Context(), &conv); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(conv)
}

// ListConversations devuelve solo las conversaciones del usuario
// autenticado para el monitor pedido — nunca las de otro usuario.
func (h *ChatHandler) ListConversations(c *fiber.Ctx) error {
	monitorID, err := primitive.ObjectIDFromHex(c.Query("monitorId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "monitorId inválido"})
	}
	userID, err := primitive.ObjectIDFromHex(c.Locals("userId").(string))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "usuario inválido"})
	}

	convs, err := h.chatRepo.ListConversationsByMonitorAndUser(c.Context(), monitorID, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(convs)
}

func (h *ChatHandler) ListMessages(c *fiber.Ctx) error {
	convID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	userID, err := primitive.ObjectIDFromHex(c.Locals("userId").(string))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "usuario inválido"})
	}

	conv, err := h.chatRepo.GetConversation(c.Context(), convID)
	if err != nil || conv.UserID != userID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "conversación no encontrada"})
	}

	msgs, err := h.chatRepo.ListMessagesByConversation(c.Context(), convID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(msgs)
}

func (h *ChatHandler) Ask(c *fiber.Ctx) error {
	convID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := c.BodyParser(&body); err != nil || body.Content == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "content es obligatorio"})
	}
	userID, err := primitive.ObjectIDFromHex(c.Locals("userId").(string))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "usuario inválido"})
	}

	msg, err := h.chatService.Ask(c.Context(), convID, userID, body.Content)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(msg)
}
```

- [ ] **Step 2: Agregar el campo `Chat` a `router.Handlers` y las rutas**

En `backend/internal/router/router.go`, agregar al struct `Handlers` (junto al resto de los campos, ej. cerca de `Screening *handlers.ScreeningHandler`):

```go
	Chat          *handlers.ChatHandler
```

Y, cerca de donde se registra el grupo `screening := protected.Group("/screening")`, agregar (todos los roles, es de solo lectura — ninguna llamada a `middleware.RequireComplianceOrAbove()` ni `RequireRole`):

```go
	chat := protected.Group("/chat")
	chat.Post("/conversations", h.Chat.CreateConversation)
	chat.Get("/conversations", h.Chat.ListConversations)
	chat.Get("/conversations/:id/messages", h.Chat.ListMessages)
	chat.Post("/conversations/:id/messages", h.Chat.Ask)
```

- [ ] **Step 3: Wiring en `backend/cmd/server/main.go`**

Cerca de donde se construye `aiRulesService := services.NewAIRulesService(systemConfigRepo)`, agregar:

```go
	chatRepo := repository.NewChatRepository(mongo)
	chatService := services.NewChatService(chatRepo, monitorRepo, systemConfigRepo)
```

Y en el literal `router.Handlers{...}`, junto a `Rule: handlers.NewRuleHandler(...)`, agregar:

```go
		Chat: handlers.NewChatHandler(chatRepo, chatService),
```

- [ ] **Step 4: Verificar que compila**

Run: `cd backend && go build ./... && go vet ./...`
Expected: sin errores.

- [ ] **Step 5: Correr el gate completo del backend**

Run: `cd backend && gofmt -l . && go vet ./... && golangci-lint run ./... && go test ./...`
Expected: `gofmt -l .` sin salida, sin issues de lint, todos los tests pasan.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handlers/chat_handler.go backend/internal/router/router.go backend/cmd/server/main.go
git commit -m "feat(chatbot): handlers, rutas y wiring completo del backend"
```

---

## Task 7: Frontend — cliente API

**Files:**
- Create: `frontend/src/lib/api/chat.ts`

**Interfaces:**
- Consumes: endpoints de la Tarea 6 (`POST /chat/conversations`, `GET /chat/conversations`, `GET /chat/conversations/:id/messages`, `POST /chat/conversations/:id/messages`).
- Produces: `chatApi.{createConversation, listConversations, listMessages, ask}`, tipos `ChatConversation`, `ChatMessage`, `ChatArtifact`, `ChartSpec`.

- [ ] **Step 1: Crear `frontend/src/lib/api/chat.ts`**

```typescript
import { api } from "./client";

export interface ChartSpec {
  chartType?: "bar" | "line" | "pie";
  data: Record<string, unknown>[];
  xKey?: string;
  yKeys?: string[];
}

export type ChatArtifactType = "chart" | "table" | "custom";

export interface ChatArtifact {
  type: ChatArtifactType;
  title: string;
  chartSpec?: ChartSpec;
  code?: string;
}

export type ChatRole = "user" | "assistant";

export interface ChatMessage {
  id: string;
  conversationId: string;
  role: ChatRole;
  content: string;
  artifact?: ChatArtifact;
  createdAt: string;
}

export interface ChatConversation {
  id: string;
  monitorId: string;
  userId: string;
  title: string;
  createdAt: string;
  updatedAt: string;
}

export const chatApi = {
  createConversation: (monitorId: string) =>
    api.post<ChatConversation>("/chat/conversations", { monitorId }),

  listConversations: (monitorId: string) =>
    api.get<ChatConversation[]>(
      `/chat/conversations?monitorId=${monitorId}`,
    ),

  listMessages: (conversationId: string) =>
    api.get<ChatMessage[]>(`/chat/conversations/${conversationId}/messages`),

  ask: (conversationId: string, content: string) =>
    api.post<ChatMessage>(`/chat/conversations/${conversationId}/messages`, {
      content,
    }),
};
```

- [ ] **Step 2: Verificar que compila**

Run: `cd frontend && npx tsc --noEmit`
Expected: sin errores.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/api/chat.ts
git commit -m "feat(chatbot): cliente API del chat"
```

---

## Task 8: Frontend — página del chatbot (selector, conversaciones, hilo de chat)

**Files:**
- Create: `frontend/src/app/(dashboard)/chatbot/page.tsx`
- Modify: `frontend/src/components/layout/top-nav.tsx`

**Interfaces:**
- Consumes: `chatApi` (Tarea 7), `monitorsApi.list` (ya existente), `useToast` (ya existente).
- Produces: página en `/chatbot`, entrada de nav "Analista IA". Estado `activeArtifact` expuesto para que la Tarea 9 monte el panel canvas sin tocar el resto de este archivo.

Esta tarea entrega un chat de solo texto completo y funcional — persistencia, historial, selector de monitor y de conversación. El panel canvas de artefactos se agrega en la Tarea 9 (esta tarea no lo renderiza; los mensajes con artefacto simplemente no muestran nada extra todavía, sin ser un error ni un placeholder — es la tarea siguiente la que lo ilumina).

- [ ] **Step 1: Crear `frontend/src/app/(dashboard)/chatbot/page.tsx`**

```tsx
"use client";

import { useCallback, useEffect, useState } from "react";
import { Header } from "@/components/layout/header";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Bot, Send, Plus } from "lucide-react";
import { useToast } from "@/lib/use-toast";
import { monitorsApi } from "@/lib/api/monitors";
import type { Monitor } from "@/lib/types";
import {
  chatApi,
  type ChatConversation,
  type ChatMessage,
} from "@/lib/api/chat";

export default function ChatbotPage() {
  const { toastError } = useToast();

  const [monitors, setMonitors] = useState<Monitor[]>([]);
  const [monitorId, setMonitorId] = useState("");

  const [conversations, setConversations] = useState<ChatConversation[]>([]);
  const [conversationId, setConversationId] = useState<string | null>(null);

  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState("");
  const [sending, setSending] = useState(false);

  useEffect(() => {
    monitorsApi
      .list()
      .then(setMonitors)
      .catch(() => toastError("Error al cargar monitores"));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const loadConversations = useCallback(
    async (forMonitorId: string) => {
      try {
        const convs = await chatApi.listConversations(forMonitorId);
        setConversations(convs);
      } catch {
        toastError("Error al cargar conversaciones");
      }
    },
    [toastError],
  );

  useEffect(() => {
    if (!monitorId) return;
    setConversationId(null);
    setMessages([]);
    loadConversations(monitorId);
  }, [monitorId, loadConversations]);

  const loadMessages = useCallback(
    async (id: string) => {
      try {
        const msgs = await chatApi.listMessages(id);
        setMessages(msgs);
      } catch {
        toastError("Error al cargar la conversación");
      }
    },
    [toastError],
  );

  useEffect(() => {
    if (conversationId) loadMessages(conversationId);
  }, [conversationId, loadMessages]);

  async function newConversation() {
    if (!monitorId) return;
    try {
      const conv = await chatApi.createConversation(monitorId);
      setConversations((prev) => [conv, ...prev]);
      setConversationId(conv.id);
      setMessages([]);
    } catch {
      toastError("Error al crear la conversación");
    }
  }

  async function send(e: React.FormEvent) {
    e.preventDefault();
    if (!input.trim()) return;

    let activeId = conversationId;
    if (!activeId) {
      try {
        const conv = await chatApi.createConversation(monitorId);
        setConversations((prev) => [conv, ...prev]);
        setConversationId(conv.id);
        activeId = conv.id;
      } catch {
        toastError("Error al crear la conversación");
        return;
      }
    }

    const question = input;
    setInput("");
    setMessages((prev) => [
      ...prev,
      {
        id: `local-${Date.now()}`,
        conversationId: activeId!,
        role: "user",
        content: question,
        createdAt: new Date().toISOString(),
      },
    ]);
    setSending(true);
    try {
      const answer = await chatApi.ask(activeId!, question);
      setMessages((prev) => [...prev, answer]);
    } catch {
      toastError("Error al consultar al chatbot");
    } finally {
      setSending(false);
    }
  }

  return (
    <>
      <Header title="Analista IA" />
      <div className="p-6 space-y-4">
        <div className="max-w-xs space-y-1.5">
          <Label>Monitor</Label>
          <select
            value={monitorId}
            onChange={(e) => setMonitorId(e.target.value)}
            className="w-full h-9 rounded-md border bg-background px-3 text-sm"
          >
            <option value="">Seleccioná un monitor</option>
            {monitors.map((m) => (
              <option key={m.id} value={m.id}>
                {m.name}
              </option>
            ))}
          </select>
        </div>

        {monitorId && (
          <div className="grid grid-cols-[220px_1fr] gap-4">
            <div className="space-y-2">
              <Button
                variant="outline"
                size="sm"
                className="w-full justify-start gap-2"
                onClick={newConversation}
              >
                <Plus className="h-4 w-4" /> Nueva conversación
              </Button>
              <div className="space-y-1">
                {conversations.map((c) => (
                  <button
                    key={c.id}
                    onClick={() => setConversationId(c.id)}
                    className={`w-full text-left rounded-md px-2 py-1.5 text-sm truncate ${
                      c.id === conversationId
                        ? "bg-accent text-accent-foreground"
                        : "hover:bg-accent/50"
                    }`}
                  >
                    {c.title}
                  </button>
                ))}
              </div>
            </div>

            <Card className="flex flex-col h-[70vh]">
              <CardContent className="flex-1 overflow-y-auto p-4 space-y-3">
                {messages.length === 0 && (
                  <p className="text-sm text-muted-foreground flex items-center gap-2">
                    <Bot className="h-4 w-4" /> Preguntame algo sobre este
                    monitor.
                  </p>
                )}
                {messages.map((m) => (
                  <div
                    key={m.id}
                    className={`rounded-md px-3 py-2 text-sm max-w-[80%] whitespace-pre-wrap ${
                      m.role === "user"
                        ? "ml-auto bg-primary text-primary-foreground"
                        : "bg-muted"
                    }`}
                  >
                    {m.content}
                  </div>
                ))}
                {sending && (
                  <p className="text-sm text-muted-foreground">Pensando…</p>
                )}
              </CardContent>
              <form
                onSubmit={send}
                className="flex gap-2 border-t p-3"
              >
                <Input
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                  placeholder="Preguntá algo sobre los datos de este monitor"
                  disabled={sending}
                />
                <Button type="submit" disabled={sending || !input.trim()}>
                  <Send className="h-4 w-4" />
                </Button>
              </form>
            </Card>
          </div>
        )}
      </div>
    </>
  );
}
```

- [ ] **Step 2: Agregar la entrada de nav en `frontend/src/components/layout/top-nav.tsx`**

Agregar `Bot` al import de `lucide-react`.

En el área `"Monitoreo"`, agregar después de `"Dashboards"`:

```typescript
      { name: "Analista IA", href: "/chatbot", icon: Bot },
```

- [ ] **Step 3: Lint y build**

Run: `cd frontend && npm run lint`
Expected: 0 errores (warnings preexistentes sin cambios).

Run: `cd frontend && npm run build`
Expected: compila, `/chatbot` aparece en la lista de rutas.

- [ ] **Step 4: Commit**

```bash
git add "frontend/src/app/(dashboard)/chatbot/page.tsx" frontend/src/components/layout/top-nav.tsx
git commit -m "feat(chatbot): página del chatbot con selector de monitor e historial"
```

---

## Task 9: Frontend — panel canvas de artefactos

**Files:**
- Create: `frontend/src/components/chat/artifact-canvas.tsx`
- Modify: `frontend/src/app/(dashboard)/chatbot/page.tsx`

**Interfaces:**
- Consumes: `ChatArtifact`/`ChartSpec` (Tarea 7).
- Produces: `ArtifactCanvas` (componente), estado `activeArtifact` montado en la página del chat.

- [ ] **Step 1: Crear `frontend/src/components/chat/artifact-canvas.tsx`**

```tsx
"use client";

import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Line,
  LineChart,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip as RechartsTooltip,
  XAxis,
  YAxis,
} from "recharts";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { ChatArtifact } from "@/lib/api/chat";

const CHART_COLORS = [
  "var(--chart-1)",
  "var(--chart-2)",
  "var(--chart-3)",
  "var(--chart-4)",
  "var(--chart-5)",
  "var(--chart-6)",
  "var(--chart-7)",
  "var(--chart-8)",
];

export function ArtifactCanvas({ artifact }: { artifact: ChatArtifact }) {
  return (
    <Card className="h-[70vh] flex flex-col">
      <CardHeader className="pb-2">
        <CardTitle className="text-base">{artifact.title}</CardTitle>
      </CardHeader>
      <CardContent className="flex-1 overflow-auto" id="artifact-canvas-content">
        {artifact.type === "chart" && artifact.chartSpec && (
          <ChartArtifact spec={artifact.chartSpec} />
        )}
        {artifact.type === "table" && artifact.chartSpec && (
          <TableArtifact data={artifact.chartSpec.data} />
        )}
        {artifact.type === "custom" && artifact.code && (
          <CustomArtifact code={artifact.code} />
        )}
      </CardContent>
    </Card>
  );
}

function ChartArtifact({
  spec,
}: {
  spec: NonNullable<ChatArtifact["chartSpec"]>;
}) {
  const yKeys = spec.yKeys ?? [];

  if (spec.chartType === "pie") {
    const dataKey = yKeys[0] ?? "value";
    return (
      <ResponsiveContainer width="100%" height={320}>
        <PieChart>
          <Pie
            data={spec.data}
            dataKey={dataKey}
            nameKey={spec.xKey ?? "name"}
            outerRadius={110}
            label
          >
            {spec.data.map((_, i) => (
              <Cell key={i} fill={CHART_COLORS[i % CHART_COLORS.length]} />
            ))}
          </Pie>
          <RechartsTooltip />
        </PieChart>
      </ResponsiveContainer>
    );
  }

  const ChartComponent = spec.chartType === "line" ? LineChart : BarChart;

  return (
    <ResponsiveContainer width="100%" height={320}>
      <ChartComponent data={spec.data}>
        <CartesianGrid strokeDasharray="3 3" />
        <XAxis dataKey={spec.xKey} />
        <YAxis />
        <RechartsTooltip />
        {yKeys.map((key, i) =>
          spec.chartType === "line" ? (
            <Line
              key={key}
              type="monotone"
              dataKey={key}
              stroke={CHART_COLORS[i % CHART_COLORS.length]}
            />
          ) : (
            <Bar
              key={key}
              dataKey={key}
              fill={CHART_COLORS[i % CHART_COLORS.length]}
            />
          ),
        )}
      </ChartComponent>
    </ResponsiveContainer>
  );
}

function TableArtifact({ data }: { data: Record<string, unknown>[] }) {
  if (data.length === 0) return null;
  const columns = Object.keys(data[0]);

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm" id="artifact-canvas-table">
        <thead>
          <tr className="border-b">
            {columns.map((col) => (
              <th key={col} className="text-left p-2 font-medium">
                {col}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {data.map((row, i) => (
            <tr key={i} className="border-b last:border-0">
              {columns.map((col) => (
                <td key={col} className="p-2">
                  {String(row[col] ?? "")}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// CustomArtifact renderiza código generado por el LLM en un iframe
// sandboxeado — sandbox="allow-scripts" SIN allow-same-origin, SIN
// allow-top-navigation, SIN allow-popups. El iframe no tiene forma de leer
// cookies/localStorage/token de sesión ni de llamar a la API de la app: el
// único dato que ve es el que ya viene embebido en artifact.code (ver
// Global Constraints del plan).
function CustomArtifact({ code }: { code: string }) {
  return (
    <iframe
      srcDoc={code}
      sandbox="allow-scripts"
      className="w-full h-full min-h-[320px] border-0"
      title="Artefacto generado"
    />
  );
}
```

- [ ] **Step 2: Montar `ArtifactCanvas` en `frontend/src/app/(dashboard)/chatbot/page.tsx`**

Agregar el import: `import { ArtifactCanvas } from "@/components/chat/artifact-canvas";`

Agregar estado, junto a `const [messages, setMessages] = useState<ChatMessage[]>([]);`:

```typescript
  const [activeArtifact, setActiveArtifact] = useState<
    ChatMessage["artifact"] | null
  >(null);
```

En `send`, después de `setMessages((prev) => [...prev, answer]);`, agregar:

```typescript
      if (answer.artifact) setActiveArtifact(answer.artifact);
```

En `loadMessages` (dentro del `.then`/`try`, después de `setMessages(msgs)`), agregar:

```typescript
      const lastWithArtifact = [...msgs].reverse().find((m) => m.artifact);
      setActiveArtifact(lastWithArtifact?.artifact ?? null);
```

Cambiar el grid de dos columnas a tres cuando hay un artefacto activo — reemplazar:

```tsx
          <div className="grid grid-cols-[220px_1fr] gap-4">
```

por:

```tsx
          <div
            className={`grid gap-4 ${
              activeArtifact
                ? "grid-cols-[220px_1fr_1fr]"
                : "grid-cols-[220px_1fr]"
            }`}
          >
```

Y, justo después del `</Card>` que cierra la tarjeta del hilo de chat (antes del cierre del `div` del grid), agregar:

```tsx
            {activeArtifact && <ArtifactCanvas artifact={activeArtifact} />}
```

- [ ] **Step 3: Lint y build**

Run: `cd frontend && npm run lint`
Expected: 0 errores.

Run: `cd frontend && npm run build`
Expected: compila.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/chat/artifact-canvas.tsx "frontend/src/app/(dashboard)/chatbot/page.tsx"
git commit -m "feat(chatbot): panel canvas de artefactos (chart/tabla/código sandboxeado)"
```

---

## Task 10: Frontend — export de artefactos

**Files:**
- Create: `frontend/src/lib/chat-export.ts`
- Modify: `frontend/src/components/chat/artifact-canvas.tsx`

**Interfaces:**
- Consumes: `ChatArtifact` (Tarea 7), el DOM montado por `ArtifactCanvas` (Tarea 9, vía los IDs `artifact-canvas-content`/`artifact-canvas-table` ya presentes).
- Produces: `exportChartAsPNG(containerId: string, filename: string): void`, `exportTableAsCSV(data: Record<string, unknown>[], filename: string): void`.

Export de gráfico: PNG vía serialización nativa del SVG que ya renderiza Recharts (sin dependencia nueva). Export de tabla: CSV nativo (sin dependencia — un CSV no necesita ninguna librería) usando `Blob`/`URL.createObjectURL`, mismo mecanismo de descarga que usaría un PDF pero mucho más simple para datos tabulares; se deja el PDF vía `jspdf`/`jspdf-autotable` (ya en el proyecto, patrón de `red-flag-report.ts`) fuera de esta tarea por YAGNI — el CSV cubre el caso de uso de "llevarme la tabla" sin agregar código nuevo de maquetado.

- [ ] **Step 1: Crear `frontend/src/lib/chat-export.ts`**

```typescript
/**
 * Export de artefactos del chatbot: PNG para gráficos (serializa el SVG
 * que ya renderiza Recharts, sin librería nueva) y CSV para tablas.
 */

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

/** Serializa el primer <svg> dentro de containerId y lo baja como PNG. */
export function exportChartAsPNG(containerId: string, filename: string) {
  const container = document.getElementById(containerId);
  const svg = container?.querySelector("svg");
  if (!svg) return;

  const { width, height } = svg.getBoundingClientRect();
  const clone = svg.cloneNode(true) as SVGSVGElement;
  clone.setAttribute("width", String(width));
  clone.setAttribute("height", String(height));

  const svgData = new XMLSerializer().serializeToString(clone);
  const svgBlob = new Blob([svgData], { type: "image/svg+xml;charset=utf-8" });
  const svgUrl = URL.createObjectURL(svgBlob);

  const img = new Image();
  img.onload = () => {
    const canvas = document.createElement("canvas");
    canvas.width = width * 2; // @2x para exportar nítido
    canvas.height = height * 2;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;
    ctx.scale(2, 2);
    ctx.fillStyle = "#ffffff";
    ctx.fillRect(0, 0, width, height);
    ctx.drawImage(img, 0, 0, width, height);
    URL.revokeObjectURL(svgUrl);

    canvas.toBlob((blob) => {
      if (blob) downloadBlob(blob, filename);
    }, "image/png");
  };
  img.src = svgUrl;
}

/** Arma un CSV simple (con comillas escapadas) y lo baja. */
export function exportTableAsCSV(
  data: Record<string, unknown>[],
  filename: string,
) {
  if (data.length === 0) return;
  const columns = Object.keys(data[0]);

  function escapeCell(value: unknown): string {
    const s = value === null || value === undefined ? "" : String(value);
    if (s.includes(",") || s.includes('"') || s.includes("\n")) {
      return `"${s.replace(/"/g, '""')}"`;
    }
    return s;
  }

  const lines = [
    columns.join(","),
    ...data.map((row) => columns.map((col) => escapeCell(row[col])).join(",")),
  ];
  const blob = new Blob([lines.join("\n")], {
    type: "text/csv;charset=utf-8",
  });
  downloadBlob(blob, filename);
}
```

- [ ] **Step 2: Agregar el botón de export en `frontend/src/components/chat/artifact-canvas.tsx`**

Agregar el import: `import { Download } from "lucide-react";`, `import { Button } from "@/components/ui/button";`, `import { exportChartAsPNG, exportTableAsCSV } from "@/lib/chat-export";`.

Cambiar el `CardHeader` para incluir el botón de export (solo para `"chart"`/`"table"` — `"custom"` no expone export en esta tarea, ya que el contenido vive en un iframe aislado y no hay forma segura de leerlo desde afuera sin romper el sandbox):

```tsx
      <CardHeader className="pb-2 flex flex-row items-center justify-between">
        <CardTitle className="text-base">{artifact.title}</CardTitle>
        {artifact.type === "chart" && (
          <Button
            variant="outline"
            size="sm"
            onClick={() =>
              exportChartAsPNG("artifact-canvas-content", `${artifact.title}.png`)
            }
          >
            <Download className="h-4 w-4" />
          </Button>
        )}
        {artifact.type === "table" && artifact.chartSpec && (
          <Button
            variant="outline"
            size="sm"
            onClick={() =>
              exportTableAsCSV(artifact.chartSpec!.data, `${artifact.title}.csv`)
            }
          >
            <Download className="h-4 w-4" />
          </Button>
        )}
      </CardHeader>
```

- [ ] **Step 3: Lint y build**

Run: `cd frontend && npm run lint`
Expected: 0 errores.

Run: `cd frontend && npm run build`
Expected: compila.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/chat-export.ts frontend/src/components/chat/artifact-canvas.tsx
git commit -m "feat(chatbot): export de gráficos (PNG) y tablas (CSV)"
```

---

## Criterio de cierre del lote

- Gate (`go build`, `go vet`, `gofmt -l .`, `golangci-lint run ./...`, `go test ./...`, `npm run lint`, `npm run build`) sin bloqueantes en las 10 tareas.
- Recorrido manual contra Mongo real y un monitor con datos:
  - Pregunta que dispara una consulta simple (`eq`/`gt`) → respuesta con números reales, verificables contra los datos.
  - Pregunta que dispara una agregación (`sum`/`count` agrupado) → todos los grupos presentes, no solo los que superarían un umbral.
  - Pregunta que el LLM decide graficar → aparece el panel canvas con el gráfico correcto (bar/line/pie según corresponda).
  - Pregunta lo bastante compleja como para forzar `type: "custom"` (ej. pedir explícitamente una visualización no estándar) → el iframe renderiza, y se verifica en devtools que no tiene acceso a `document.cookie`/`localStorage` del padre ni puede hacer `fetch` a la API.
  - Descargar un gráfico como PNG y una tabla como CSV → archivos válidos, abribles.
  - Sin API key de IA configurada → el chat responde con el mensaje explicativo, nunca un 500.
  - Dos usuarios distintos, mismo monitor → cada uno ve solo sus propias conversaciones.
  - Repetir el flujo completo con el proveedor en `deepseek` y en `anthropic` (cambiar en Configuración → APIs → IA) → ambos funcionan con el mismo contrato de herramienta.
