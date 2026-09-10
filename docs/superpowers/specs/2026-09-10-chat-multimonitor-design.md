# Chat multi-monitor — Diseño

**Fecha:** 2026-09-10
**Estado:** Aprobado para pasar a plan de implementación
**Parte 1 de 2.** La parte 2 (`2026-09-10-chat-artefactos-design.md`) depende de
esta: el artefacto de primera clase necesita registrar de qué monitores salieron
sus datos, y ese conjunto lo define este spec.

## Objetivo

Que una conversación del Analista IA consulte **varios monitores a la vez**, de
modo que el asistente pueda comparar y correlacionar entre ellos en una misma
respuesta ("compará el volumen de transferencias del monitor A contra el de
alertas SWIFT del monitor B").

Hoy `ChatConversation` está atada a un único `MonitorID` y la herramienta
`query_monitor_data` no tiene selector de monitor: significa implícitamente "el
monitor actual".

## Alcance del cruce (explícito)

**Correlación en la respuesta, no join en la base.** El LLM consulta cada
monitor por separado —una o más llamadas a la herramienta por monitor— y
correlaciona los resultados al redactar: compara totales, arma un gráfico con
una serie por monitor, señala diferencias.

**No hay `$lookup` entre colecciones.** Preguntas del tipo "qué clientes
aparecen en los dos monitores" no se responden con un join real; el asistente
puede traer ambas listas y compararlas, con el límite de tamaño que eso implica.
Un join a nivel base requeriría declarar una clave de cruce por monitor,
indexarla en cada colección y un tipo de operación nuevo en la herramienta.
Queda fuera, y el contrato de la herramienta se diseña para que agregarlo
después sea aditivo (ver "Extensiones aditivas").

Esto es coherente con el hecho de que cada monitor vive en su propia colección
`data_<monitor_id>` con su propio schema detectado, sin ninguna garantía de
campos comunes entre monitores.

## Fuera de alcance

- **Streaming de tokens (SSE).** Sigue fuera, como en el diseño original.
- **Scoping de monitores por usuario.** Hoy `GET /monitors`
  (`monitor_handler.go:178`) devuelve `FindAll` a cualquier usuario autenticado:
  no hay noción de "monitores asignados". Este spec no la introduce. La
  consecuencia de seguridad se trata abajo, en "Autorización".
- **Quitar un monitor de una conversación existente.** Se pueden agregar, no
  quitar: el historial ya referenció ese monitor y sus datos, y removerlo
  dejaría mensajes previos hablando de algo que el asistente ya no puede ver.

## Modelo de datos

`models.ChatConversation`:

```go
MonitorIDs []primitive.ObjectID `bson:"monitor_ids" json:"monitorIds"`
```

reemplaza a `MonitorID primitive.ObjectID` (`bson:"monitor_id"`).

**Compatibilidad con conversaciones existentes.** Los documentos ya creados
traen `monitor_id` (escalar). No se hace migración: la struct conserva un campo
legacy no exportado en JSON y se normaliza al leer.

```go
type ChatConversation struct {
    ...
    MonitorIDs []primitive.ObjectID `bson:"monitor_ids,omitempty" json:"monitorIds"`
    // LegacyMonitorID: conversaciones creadas antes del multi-monitor. Nunca
    // se escribe; solo se lee para normalizar en Normalize().
    LegacyMonitorID *primitive.ObjectID `bson:"monitor_id,omitempty" json:"-"`
}

// Normalize colapsa el campo legacy en MonitorIDs. Todo lector de
// conversaciones debe llamarla justo después de decodificar.
func (c *ChatConversation) Normalize()
```

Los puntos de lectura que deben llamar `Normalize()`: `GetConversation` y
`ListConversationsByUser` en `chat_repo.go`. Se centraliza ahí para que ningún
llamador pueda olvidarlo.

**Cota:** máximo 6 monitores por conversación. Cada monitor agrega un bloque de
schema al system prompt y al menos una iteración de herramienta; más allá de eso
el prompt se vuelve caro y la respuesta lenta sin que el caso de uso lo pida. Se
valida en el handler, con mensaje explícito.

### Índices

El índice actual `{monitor_id: 1, user_id: 1, updated_at: -1}`
(`chat_repo.go:32`) deja de servir: la lista pasa a ser por usuario (ver abajo).
Se reemplaza por:

- `{user_id: 1, updated_at: -1}` — la consulta principal del sidebar.
- `{user_id: 1, monitor_ids: 1}` — el filtro opcional "solo las que incluyen X".
  Mongo indexa arrays elemento a elemento, así que un índice multikey sobre
  `monitor_ids` sirve para ese filtro sin nada especial.

El índice viejo no se borra automáticamente; queda anotado en el plan de
implementación como paso manual opcional (no molesta si queda).

## Alias de monitor

La herramienta necesita que el LLM diga **a qué monitor** consulta. No se usa el
ObjectID: son 24 caracteres hex que el modelo transcribe mal con frecuencia
suficiente como para que sea un problema real, y son ilegibles en el prompt.

Cada monitor de la conversación recibe un **alias corto, estable dentro de la
conversación**, derivado de su nombre:

- Normalizar (NFD, quitar diacríticos), minúsculas, no alfanumérico → `_`,
  colapsar `_` repetidos, recortar a 40 caracteres.
- Colisiones: sufijo `_2`, `_3`… en orden de `MonitorIDs`.
- Si el resultado queda vacío (nombre solo con símbolos): `monitor_<n>`.

El alias se **deriva, no se persiste**: se recalcula en cada request a partir de
`MonitorIDs` y los nombres actuales. Es determinista para un mismo conjunto de
monitores y nombres, que es la garantía que hace falta dentro de una
conversación. Renombrar un monitor cambia su alias, pero el alias solo vive
dentro de un turno (prompt → tool call → respuesta), nunca cruza turnos.

> Nota para la parte 2: los artefactos guardados **sí** persisten el alias como
> parte de su `sources`. Ese spec resuelve la revalidación del alias contra los
> monitores del artefacto al re-ejecutar.

```go
// buildMonitorAliases arma el mapa alias -> monitor para una conversación.
// Es la ÚNICA fuente de verdad de qué monitores puede tocar el LLM en este
// turno: se construye a partir de conv.MonitorIDs y nada más.
func buildMonitorAliases(monitors []models.Monitor) map[string]*models.Monitor
```

## Contrato de la herramienta

`queryToolJSONSchema` gana una propiedad obligatoria:

```json
"monitor": {
  "type": "string",
  "description": "Alias del monitor a consultar. Debe ser uno de los alias listados en el prompt."
}
```

`"monitor"` se agrega a `required`. `ParseQueryToolInput`
(`chat_llm_parsing.go`) suma el campo y valida que no esté vacío.

`queryToolDescription` agrega:

> Cada llamada consulta UN monitor. Para comparar entre monitores, llamá una vez
> por cada uno y correlacioná los resultados al responder.

### System prompt

`buildSystemPrompt` pasa de recibir un `*models.Monitor` a recibir el mapa de
alias. Emite un bloque por monitor:

```
Tenés acceso a estos monitores. Usá el alias para consultarlos:

### transacciones — "Transacciones Bancolombia"
- fecha: date
- monto: number
...

### alertas_swift — "Alertas SWIFT"
- ...
```

## Autorización

**Esta es la parte que importa y no debe diluirse.**

`executeQuery` resuelve el alias **exclusivamente contra el mapa construido a
partir de `conv.MonitorIDs`**. Un alias desconocido devuelve un error de
herramienta —que el LLM ve y puede corregir en la siguiente iteración— y **nunca
ejecuta una consulta**. No se acepta un ObjectID crudo en el campo `monitor`
bajo ninguna circunstancia; si llega algo que parece un hex, es un alias
desconocido como cualquier otro.

El motivo no es hipotético: **los datos que el LLM lee son archivos subidos por
usuarios**. Una celda de un CSV con una inyección de prompt puede intentar que el
asistente consulte un monitor fuera de la conversación. Con el alias resuelto
contra la allowlist, ese ataque no tiene a dónde ir: el nombre no existe en el
mapa y la consulta no ocurre.

Como `GET /monitors` no scopea por usuario, `conv.MonitorIDs` —fijado al crear
la conversación por el propio usuario— **es el único límite real** de qué datos
toca el asistente. Por eso la validación vive en `executeQuery`, el punto por el
que pasa toda consulta, y no en el handler.

`CreateConversation` valida que cada `monitorId` recibido exista antes de
guardar; el conjunto no se puede alterar después salvo por el endpoint de
agregar monitor, que valida igual.

## Cotas de iteración y timeout

Ambas están dimensionadas hoy para un solo monitor y quedan cortas:

| | Hoy | Nuevo |
|---|---|---|
| Iteraciones de herramienta | `5` fijo | `5 + 3×len(MonitorIDs)`, tope `20` |
| Timeout de `Ask` | `90s` fijo | `60s + 30s×len(MonitorIDs)`, tope `240s` |

Una pregunta cruzada necesita al menos una llamada por monitor antes de poder
empezar a razonar, así que un tope fijo de 5 se agota sin haber respondido.

El fallback existente —una última llamada sin la herramienta para forzar una
respuesta de texto con lo que el LLM ya tenga— se conserva tal cual.

## Historial y artefactos

`historyToAnthropic` / `historyToDeepSeek` serializan hoy **solo `m.Content`**,
y descartan `m.Artifact`. Eso significa que el asistente no tiene forma de saber
que en el turno 3 generó un gráfico: "volvé a mostrarme la tabla de antes" no
puede funcionar.

Se agrega una línea de referencia al serializar un mensaje del asistente que
llevaba artefacto:

```
[Generaste un artefacto: tipo=table, título="Ventas por región"]
```

Sin los datos —serían miles de tokens por turno y el LLM ya los describió en su
texto—, solo la referencia. Es la base sobre la que la parte 2 construye la
invocación de artefactos guardados.

## API

| Método | Ruta | Cambio |
|---|---|---|
| `POST` | `/chat/conversations` | body `{monitorId}` → `{monitorIds: []}`; valida 1..6 y existencia |
| `GET` | `/chat/conversations` | ya no requiere `monitorId`; scopea por usuario. `?monitorId=` pasa a ser filtro **opcional** |
| `POST` | `/chat/conversations/:id/monitors` | **nuevo** — agrega un monitor a la conversación (valida cota y existencia) |

`GET /chat/conversations/:id/messages`, `POST .../messages` y
`DELETE /chat/conversations/:id` no cambian de forma.

`ListConversationsByMonitorAndUser` en el repositorio se reemplaza por
`ListConversationsByUser(ctx, userID, monitorFilter *primitive.ObjectID)`.

## Frontend

**Inversión del flujo.** Hoy el `<select>` de monitor es el que manda: hasta que
no elegís uno no hay nada, y cambiarlo resetea el hilo
(`page.tsx:38-45`, el patrón `prevMonitorId`). Con conversaciones que abarcan
varios monitores eso deja de tener sentido: una conversación sobre tres
monitores no pertenece a la lista de ninguno.

- **El selector superior de monitor desaparece.** El patrón `prevMonitorId` se
  elimina por completo.
- **El sidebar lista las conversaciones del usuario**, ordenadas por actividad,
  con chips de monitor en cada fila y un filtro opcional por monitor.
- **"Nueva conversación" abre un multi-select con búsqueda y chips** para elegir
  de 1 a 6 monitores. Es el único momento en que se define el conjunto (más el
  botón de agregar monitor dentro de la conversación abierta).
- **La cabecera de la conversación abierta muestra los chips de sus monitores**,
  para que siempre esté claro sobre qué datos se está preguntando.
- El estado de carga pasa de `"Pensando…"` suelto a un indicador en el hilo,
  consistente con el resto de la app.

`lib/api/chat.ts` acompaña: `ChatConversation.monitorId` → `monitorIds`,
`createConversation(monitorIds: string[])`, `listConversations(monitorId?)`,
`addMonitor(conversationId, monitorId)`.

## Manejo de errores

| Situación | Comportamiento |
|---|---|
| Alias desconocido en el tool call | Error de herramienta al LLM, sin consultar. El LLM puede reintentar con un alias válido. |
| Monitor borrado que sigue en `MonitorIDs` | Se excluye del mapa de alias y del prompt; la conversación sigue funcionando con el resto. Si no queda ninguno, `ErrMonitorNotFound` (mismo sentinel de hoy). |
| Más de 6 monitores al crear | `400` con mensaje explícito. |
| `monitorIds` vacío | `400`. |
| Conversación de otro usuario | `ErrConversationNotFound`, indistinguible de "no existe" — se conserva el criterio actual. |

## Testing

Go, siguiendo el patrón de `chat_query_test.go`:

- `buildMonitorAliases`: derivación, colisiones (`_2`, `_3`), nombres con
  diacríticos, nombre solo con símbolos → `monitor_<n>`.
- **Autorización**: alias fuera de `conv.MonitorIDs` → error, y verificación de
  que no se ejecutó consulta. ObjectID crudo en `monitor` → tratado como alias
  desconocido.
- `Normalize()`: documento legacy con `monitor_id` → `MonitorIDs` de un
  elemento; documento nuevo → sin cambios; ambos campos presentes → gana
  `monitor_ids`.
- `buildSystemPrompt` con varios monitores: un bloque por monitor, cada uno con
  su alias.
- Cotas: iteraciones y timeout calculados según cantidad de monitores.
- `ParseQueryToolInput` con `monitor` ausente o vacío → error.

## Extensiones aditivas (anotadas, fuera de alcance)

- **Join real entre monitores**: se agregaría como un tipo de operación nuevo en
  el schema de la herramienta (`join` junto a `conditionGroup` / `aggregate`),
  con clave de cruce declarada por monitor. El campo `monitor` ya presente y la
  resolución contra allowlist no cambian.
- **Scoping de monitores por usuario**: cuando `GET /monitors` deje de ser
  `FindAll`, `CreateConversation` debe validar `MonitorIDs` contra los monitores
  accesibles del usuario, además de contra su existencia.
