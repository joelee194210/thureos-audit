# Chatbot "Analista IA" — Diseño

**Fecha:** 2026-09-07
**Estado:** Aprobado para pasar a plan de implementación

## Objetivo

Un chatbot conversacional que responde preguntas en lenguaje natural sobre los
datos de un monitor específico, con respuestas basadas en consultas reales
contra los datos (nunca inventadas), capaz de generar artefactos visuales
(gráficos, tablas) en un panel canvas separado del chat, y de exportarlos.

## Fuera de alcance (explícito)

- **Streaming de tokens (SSE).** Las respuestas se devuelven completas, no
  token a token. Queda como mejora futura — se puede agregar después sin
  rediseñar el resto (el contrato `POST .../messages` → respuesta JSON
  completa no cambia de forma al agregar streaming más adelante).
- **Conversaciones compartidas en equipo.** Cada conversación es privada del
  usuario que la creó (ver Modelo de datos).
- **Editar/reintentar mensajes ya enviados.** Solo mandar mensajes nuevos.

## Arquitectura

Subsistema nuevo, aislado del resto de la app salvo por dos puntos de
reutilización deliberados:

1. **`AIConfig`** (`system_config`, ya existe): mismo mecanismo de provider
   (Anthropic/DeepSeek) + API key que usa hoy `AIRulesService`. El chatbot no
   tiene su propia configuración de proveedor — usa la que ya está en
   Configuración → APIs → IA.
2. **`BuildMongoFilter` y los constructores de pipeline de agregación**
   (`rule_engine.go`, funciones de paquete ya existentes y reutilizables sin
   depender de una `Rule` persistida): el chatbot arma un `ConditionGroup` /
   `AggregateCondition` a partir de lo que decide el LLM, y usa exactamente
   la misma maquinaria de traducción a Mongo que ya usa el motor de reglas.
   No se duplica lógica de consulta.

```
Usuario ── pregunta ──> ChatHandler ──> ChatService
                                           │
                                           ├─ carga historial de la conversación (chat_repo)
                                           ├─ arma system prompt (schema del monitor + contrato de herramientas)
                                           ├─ llama al LLM (AIConfig: Anthropic o DeepSeek) con tool "query_monitor_data"
                                           │
                                           │   loop (máx 5 iteraciones):
                                           ├─ si el LLM pide la herramienta:
                                           │     ConditionGroup/AggregateCondition ──BuildMongoFilter──> Mongo (acotado)
                                           │     resultado real ──> vuelve al LLM
                                           │
                                           └─ respuesta final (texto + artefacto opcional)
                                                  │
                                           chat_repo.Create (user msg + assistant msg)
                                                  │
Frontend <── mensaje + artefacto ─────────────────┘
   │
   ├─ texto → hilo de chat
   └─ artefacto → panel canvas (Recharts o iframe sandboxeado)
```

## Modelo de datos

`backend/internal/models/chat.go` (nuevo):

- **`ChatConversation`**: `ID`, `MonitorID`, `UserID` (dueño — privada, no
  compartida en equipo), `Title` (se autogenera a partir del primer mensaje
  del usuario, sin UI de renombrado — no se pidió), `CreatedAt`, `UpdatedAt`.
- **`ChatMessage`**: `ID`, `ConversationID`, `Role` (`"user"` | `"assistant"`),
  `Content` (texto), `Artifact *ChatArtifact` (nil si la respuesta es solo
  texto), `CreatedAt`.
- **`ChatArtifact`**: `Type` (`"chart"` | `"table"` | `"custom"`), `Title`,
  `ChartSpec *ChartSpec` (para `"chart"`/`"table"`), `Code string` (JS/HTML
  para `"custom"` — ver Seguridad).
- **`ChartSpec`**: `ChartType` (`"bar"` | `"line"` | `"pie"` — solo aplica
  cuando `ChatArtifact.Type == "chart"`; se ignora si `Type == "table"`),
  `Data []map[string]interface{}` (filas ya resueltas, listas para
  Recharts o para una tabla simple), `XKey string`, `YKeys []string` (solo
  usados por `"chart"`; una tabla simplemente renderiza todas las claves de
  `Data` como columnas).

Colecciones Mongo nuevas: `chat_conversations`, `chat_messages`. Índices:
`chat_conversations` por `{monitor_id, user_id, updated_at}`; `chat_messages`
por `{conversation_id, created_at}`.

## Componentes

### Backend

- **`repository/chat_repo.go`** (nuevo, sin tests — capa Mongo, mismo
  criterio que el resto del proyecto): `CreateConversation`,
  `ListConversationsByMonitorAndUser`, `CreateMessage`,
  `ListMessagesByConversation`, `GetConversation` (para validar ownership).
- **`services/chat_service.go`** (nuevo): `Ask(ctx, conversationID,
  userID string, userMessage string) (models.ChatMessage, error)`.
  Responsabilidades:
  - Cargar la conversación y su historial; rechazar si `conversation.UserID
    != userID` (una conversación es del usuario que la creó, nadie más la
    puede leer ni escribirle).
  - Cargar el monitor (schema) para el system prompt.
  - Dispatch de proveedor: mismo patrón `switch cfg.AI.Provider` que
    `AIRulesService`, pero con soporte de tool-calling agregado a las dos
    implementaciones (Anthropic SDK ya soporta `Tools` en
    `MessageNewParams`; DeepSeek es API compatible con OpenAI — se agrega
    `tools`/`tool_choice` al `deepSeekRequest` existente).
  - Loop de herramienta acotado a 5 iteraciones: si el LLM pide
    `query_monitor_data(conditionGroup, aggregate?)`, ejecutar contra
    `monitor.CollectionID` con tope de 1000 filas y timeout de contexto
    (ej. 10s), devolver el resultado (o el error) al LLM como resultado de
    la herramienta, y continuar.
  - Al recibir la respuesta final del LLM: parsear el artefacto opcional
    (`ChartSpec` o `Code`) desde un bloque estructurado que el prompt le
    pide devolver; si no parsea, se descarta el artefacto y se conserva
    solo el texto (nunca se le muestra un error de parseo al usuario).
  - Persistir mensaje de usuario + mensaje de asistente (con artefacto si
    hay) vía `chat_repo`.
- **`handlers/chat_handler.go`** (nuevo) + rutas bajo `protected` (sin
  restricción de rol — todos los roles pueden usar el chatbot, es de solo
  lectura):
  - `POST /chat/conversations` (crea, recibe `monitorId`)
  - `GET /chat/conversations?monitorId=` (lista las propias del usuario
    autenticado para ese monitor)
  - `GET /chat/conversations/:id/messages`
  - `POST /chat/conversations/:id/messages` (manda una pregunta, devuelve
    el mensaje de asistente creado)

### Frontend

- **`lib/api/chat.ts`** (nuevo): cliente tipado, mismo patrón que
  `lib/api/screening.ts`.
- **Nav + página nueva**: entrada "Analista IA" en `top-nav.tsx` →
  `app/(dashboard)/chatbot/page.tsx`. Selector de monitor al entrar; una vez
  elegido, tres columnas: lista de conversaciones del usuario para ese
  monitor (izquierda, angosta), hilo de chat (centro), panel canvas de
  artefactos (derecha, aparece solo cuando hay un artefacto activo).
- **`components/chat/artifact-canvas.tsx`** (nuevo): recibe un
  `ChatArtifact` y lo renderiza:
  - `"chart"`/`"table"` → Recharts (biblioteca ya en uso en dashboards), sin
    ejecutar nada del LLM.
  - `"custom"` → `<iframe sandbox="allow-scripts">` (sin
    `allow-same-origin`, sin `allow-top-navigation`, sin `allow-popups`),
    con el `Code` inyectado como contenido del iframe. El iframe recibe los
    datos que ya trajo la consulta como una constante JSON embebida en el
    documento — no le hace `fetch` a nada, no tiene forma de llamar a la
    API de la app ni leer el token de sesión.
- **Export**: botón en el panel canvas — PNG del gráfico renderizado (nueva
  dependencia liviana tipo `html-to-image` o equivalente, a definir en el
  plan) o CSV/PDF de la tabla (reusando el patrón `jsPDF`/`jspdf-autotable`
  que ya existe en `red-flag-report.ts`).

## Seguridad — código generado por el LLM

Este es el único punto del diseño con superficie de riesgo real, así que
queda explícito: el `Code` que el LLM puede generar para un artefacto
`"custom"` se ejecuta **exclusivamente** dentro de un iframe con
`sandbox="allow-scripts"` y nada más. Concretamente eso significa:

- Sin `allow-same-origin`: el contenido del iframe se trata como de origen
  opaco — no puede leer `localStorage`/cookies de la app aunque corra JS.
- Sin acceso de red a nuestra propia API (no se le da URL, headers ni
  token — el JWT nunca llega al iframe).
- Sin `allow-top-navigation` ni `allow-popups`: no puede redirigir la
  pestaña ni abrir ventanas.
- Los únicos datos que ve son los que ya trajo la consulta real (inyectados
  como JSON estático) — no puede pedir más datos por su cuenta.

Esto es el mismo patrón de aislamiento que ya se usa para renderizar
artefactos de código no confiable en general: sandboxing por iframe, nunca
`eval`/`Function()` en el contexto de la página principal.

## Manejo de errores

- Sin API key configurada (`cfg.AI.APIKey == ""`) → mensaje de asistente
  con texto claro ("Configurá una API key de IA en Configuración → APIs"),
  mismo patrón que `AIRulesService.GenerateRules` hoy — no es una
  excepción, es una respuesta de chat normal explicando el problema.
- Error del proveedor LLM (red, rate limit, respuesta inválida) → mensaje
  de asistente con el error, la conversación queda intacta y se puede
  reintentar con un mensaje nuevo.
- Consulta de la herramienta mal armada o que excede el tope de
  filas/tiempo → se le devuelve el error al LLM como resultado de la
  herramienta (puede ajustar la consulta o explicarle la limitación al
  usuario), nunca aborta la conversación.
- Artefacto mal formado (JSON roto en el `ChartSpec`, código que no genera
  HTML válido) → se descarta el artefacto, se conserva el texto de la
  respuesta. El usuario nunca ve un error de parseo crudo.
- Loop de herramienta que llega al tope de 5 iteraciones sin respuesta
  final → se le pide al LLM una última vez que responda con lo que tenga,
  sin más llamadas a herramientas.

## Testing

- **TDD completo** para las piezas puras y testeables sin Mongo/LLM: el
  mapeo de la respuesta de la herramienta del LLM a
  `models.ConditionGroup`/`models.AggregateCondition`, y la validación de
  un `ChartSpec` recibido (tipos válidos, `Data` no vacío, etc.).
- **Sin tests unitarios** para `ChatService`/`chat_repo` (capa
  Mongo/LLM) — mismo criterio ya establecido en todo el proyecto
  (`AIRulesService`, `ScreeningService`, etc.): se verifica manualmente.
- **Verificación manual** (previa al cierre del lote): pregunta que
  dispara una consulta simple (`eq`/`gt`), una que dispara una
  agregación (`sum`/`count` agrupado), una que el LLM decide graficar
  (chart-spec), una lo bastante compleja como para forzar el fallback de
  código sandboxeado, exportar un artefacto de cada tipo, y una pregunta
  sin API key configurada (verificar el mensaje de error, no un 500).
