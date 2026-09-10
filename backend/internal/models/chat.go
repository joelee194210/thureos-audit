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
