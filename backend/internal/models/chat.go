package models

import (
	"encoding/json"
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
	// MonitorID es el VÍNCULO: el ObjectID del monitor contra el que esta
	// fuente se resolvió al crear el artefacto. Se estampa una sola vez
	// (bindArtifactSources) y a partir de ahí es por él —no por el
	// alias— que la fuente se resuelve al re-ejecutar.
	//
	// Existe porque el alias se deriva del NOMBRE y se recalcula en cada
	// corrida contra art.MonitorIDs, que es un subconjunto reordenado de
	// conv.MonitorIDs: dos monitores cuyos nombres colisionan al truncar
	// en 40 caracteres se reparten el alias base y el "_2" según el orden
	// de la lista, así que con el orden invertido la fuente 0 terminaba
	// consultando al monitor de la fuente 1 —con su rótulo puesto— sin un
	// solo error. El id no depende del orden ni del nombre.
	//
	// `json:"-"` NO ES COSMÉTICO. llmArtifactInput.Sources (ver
	// chat_llm_parsing.go) es []ArtifactSource, así que CUALQUIER campo
	// con tag json de este struct es escribible por el LLM —cuya salida
	// está moldeada por los CSV que suben los usuarios— y un id de monitor
	// escribible sería exactamente el agujero que el DTO angosto cerró:
	// elegir el monitor a consultar. Sin tag json no hay dónde aterrizar
	// ni al deserializar ni al serializar. La autorización tampoco se
	// apoya en este campo: al re-ejecutar se exige igual que el id esté
	// entre los monitores de art.MonitorIDs (ver resolveArtifactMonitors).
	MonitorID primitive.ObjectID `bson:"monitor_id,omitempty" json:"-"`
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
// Para Type == "custom", Code es HTML/JS autocontenido que corre aislado
// en un iframe sandboxeado (ver Global Constraints) — nunca en el
// contexto de la app.
type ChatArtifact struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID         primitive.ObjectID `bson:"user_id" json:"userId"`
	ConversationID primitive.ObjectID `bson:"conversation_id" json:"conversationId"`
	MessageID      primitive.ObjectID `bson:"message_id,omitempty" json:"messageId,omitempty"`
	// MonitorIDs es la allowlist del artefacto: se deriva de las sources
	// ya resueltas del propio artefacto al crearlo, acotadas contra los
	// monitores de la conversación, y es contra esto que se resuelven los
	// alias de las sources al re-ejecutar. Un artefacto nunca puede ganar
	// acceso a un monitor que su conversación no tenía.
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

// artifactJSON es un tipo DEFINIDO a partir de ChatArtifact: hereda sus
// campos pero NO sus métodos, que es lo que evita que MarshalJSON se
// llame a sí mismo para siempre.
type artifactJSON ChatArtifact

// MarshalJSON omite del JSON los tres campos cuyo cero no significa
// "cero" sino "no hay": el id, el id del mensaje y la fecha de corrida.
//
// POR QUÉ NO ALCANZA `omitempty`. primitive.ObjectID es [12]byte y
// time.Time es un struct: encoding/json nunca los considera "empty", así
// que `json:"id,omitempty"` es una promesa que no se cumple. Un artefacto
// legacy (FromLegacyArtifact: sin id, sin corrida) se serializaba con
// "id":"000000000000000000000000" y "ranAt":"0001-01-01T00:00:00Z" —dos
// valores que en JavaScript son TRUTHY y una fecha VÁLIDA—, así que los
// guardas del frontend (`artifact.id ?? ...`, `disabled={!artifact.id}`)
// no se disparaban nunca: "Guardar" quedaba habilitado y devolvía 404, y
// todos los artefactos legacy compartían la misma key de React, por lo
// que el canvas no se remontaba al cambiar de conversación y pintaba el
// título nuevo sobre las filas del anterior.
//
// Se eligió el marshaler propio antes que cambiar ID a *primitive.ObjectID:
// el campo se usa como valor en repositorio, handlers y servicio (art.ID,
// &artifact.ID), y volverlo puntero repartiría desreferencias nileables
// por todo el camino caliente para arreglar un problema que es solo de
// serialización. Acá queda en un punto, al lado del struct.
//
// El receptor es por VALOR a propósito: así el método está también en el
// method set de *ChatArtifact, y tanto c.JSON(art) (puntero) como
// c.JSON(arts) ([]ChatArtifact, la biblioteca) pasan por acá.
func (a ChatArtifact) MarshalJSON() ([]byte, error) {
	// Los campos del struct anónimo están a profundidad 0 y los de
	// artifactJSON a profundidad 1: encoding/json resuelve el conflicto a
	// favor del menos profundo, así que estos tres ganan y su omitempty
	// —sobre punteros y no sobre arreglos— sí funciona.
	shadow := struct {
		artifactJSON
		ID        *primitive.ObjectID `json:"id,omitempty"`
		MessageID *primitive.ObjectID `json:"messageId,omitempty"`
		RanAt     *time.Time          `json:"ranAt,omitempty"`
	}{artifactJSON: artifactJSON(a)}
	if !a.ID.IsZero() {
		shadow.ID = &a.ID
	}
	if !a.MessageID.IsZero() {
		shadow.MessageID = &a.MessageID
	}
	if !a.RanAt.IsZero() {
		shadow.RanAt = &a.RanAt
	}
	return json.Marshal(shadow)
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

// AppendMonitorID devuelve el conjunto de monitores de una conversación
// con monitorID agregado al final, junto con un booleano que indica si
// hubo cambio. Es el cálculo que el repositorio persiste tal cual, y vive
// acá —al lado de Normalize— porque las dos implementan la misma regla:
// MonitorIDs es la verdad, el campo legacy es residuo.
//
// Dos invariantes que el llamador necesita:
//   - El orden se conserva y lo nuevo va al final. buildMonitorAliases usa
//     el orden de MonitorIDs para decidir qué monitor se queda con el alias
//     base cuando dos nombres colisionan; reordenar cambiaría alias ya
//     usados en el historial.
//   - Agregar un monitor ya presente no cambia nada (changed == false), así
//     que repetir la llamada es un no-op.
//
// existing debe venir de una conversación ya normalizada: para un
// documento legacy eso es exactamente el monitor del campo escalar, que
// por lo tanto queda primero en el resultado.
func AppendMonitorID(existing []primitive.ObjectID, monitorID primitive.ObjectID) ([]primitive.ObjectID, bool) {
	for _, id := range existing {
		if id == monitorID {
			return existing, false
		}
	}
	next := make([]primitive.ObjectID, len(existing), len(existing)+1)
	copy(next, existing)
	return append(next, monitorID), true
}
