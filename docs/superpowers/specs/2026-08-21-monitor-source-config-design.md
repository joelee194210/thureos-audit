# Parámetros de carga configurables por tipo de fuente

Fecha: 2026-08-21 · Estado: aprobado en chat, pendiente de revisión del spec escrito

## Suposiciones declaradas

El usuario no respondió dos preguntas de aclaración durante el brainstorming (ambas
tras 60s de espera). Se avanzó con la opción recomendada en cada caso; son revisables
y están marcadas **[S1]**–**[S2]**.

- **[S1] Frecuencia del pull de API.** Minutos/horas configurables por monitor, en vez de
  reusar los presets diarios/semanales/mensuales que ya usan las reglas — esos son
  demasiado infrecuentes para monitoreo transaccional.
- **[S2] Formato TXT.** Delimitado (como CSV pero con separador configurable), no logs de
  línea libre con regex. Reutiliza el parser de CSV existente, solo parametrizando el
  delimitador.

Todo lo demás (alcance de las tres piezas juntas, ambos modos push+pull de API) fue
confirmado explícitamente por el usuario en el chat.

## Problema

El diálogo de creación de monitor (`frontend/src/app/(dashboard)/monitors/page.tsx`) solo
pide `name`, `description` y `sourceType` — nada más, sin importar qué tipo de fuente se
elija. El backend sí tiene campos `apiEndpoint` y `schedule` en `Monitor`
(`backend/internal/models/monitor.go`), pero el frontend nunca los expone y nada en el
backend los usa para nada funcional.

Más grave: `MonitorHandler.UploadData` (`backend/internal/handlers/monitor_handler.go:127`)
solo sabe procesar `csv`, `json` y `excel`. Si `monitor.SourceType == "api"`, la ruta cae en
el `default` y responde `"unsupported source type"`. **No existe ningún camino, hoy, para
que un monitor tipo API reciba datos alguna vez** — ni push ni pull, ni endpoint receptor
ni poller. Tampoco existe `txt` como `SourceType` en el enum actual (solo
`csv | excel | json | api`).

CSV, Excel y JSON sí funcionan end-to-end vía carga manual de archivo
(`IngestCSV`/`IngestExcel`/`IngestJSON` en `backend/internal/services/ingestion_service.go`),
con detección automática de schema. Ese camino no está roto — se deja intacto.

## Enfoque elegido

**Extender `Monitor` con un `SourceConfig` tipado**, manteniendo la relación 1:1
monitor↔fuente que ya existe (no se introduce una entidad `DataSource` separada — sería
sobre-ingeniería para el caso actual, YAGNI). El pull de API reutiliza el patrón de
`Scheduler` (`backend/internal/services/scheduler.go`): un cron que corre cada minuto y
revisa qué está vencido, en vez de una librería/mecanismo nuevo.

Alternativas descartadas:
- **Entidad `DataSource` normalizada, separada de `Monitor`.** Más flexible a futuro
  (multi-fuente por monitor), pero hoy cada monitor tiene exactamente una fuente — la
  indirección extra no se paga con nada.
- **Resolver solo API push, dejar pull y TXT para después.** No cubre lo que el usuario
  pidió explícitamente ("ambos modos, configurable por monitor").

## Diseño

### 1. Modelo de datos

`backend/internal/models/monitor.go` — nuevo campo `SourceConfig` (bson subdocumento,
`omitempty`), forma según `SourceType`. Todos los campos son opcionales con default igual
al comportamiento actual, así que monitores existentes sin `SourceConfig` siguen
funcionando sin cambios.

```go
type SourceType string

const (
    SourceCSV   SourceType = "csv"
    SourceExcel SourceType = "excel"
    SourceJSON  SourceType = "json"
    SourceTXT   SourceType = "txt"   // nuevo
    SourceAPI   SourceType = "api"
)

type APIMode string

const (
    APIModePush APIMode = "push"
    APIModePull APIMode = "pull"
)

type APIAuthType string

const (
    APIAuthNone      APIAuthType = "none"
    APIAuthAPIKey    APIAuthType = "api_key_header"
    APIAuthBearer    APIAuthType = "bearer"
)

type SourceConfig struct {
    // csv / txt
    Delimiter    string `bson:"delimiter,omitempty" json:"delimiter,omitempty"`
    HasHeaderRow *bool  `bson:"has_header_row,omitempty" json:"hasHeaderRow,omitempty"`

    // excel
    SheetName string `bson:"sheet_name,omitempty" json:"sheetName,omitempty"`

    // json / api (extracción de array anidado)
    RootPath string `bson:"root_path,omitempty" json:"rootPath,omitempty"`

    // api
    Mode                APIMode     `bson:"mode,omitempty" json:"mode,omitempty"`
    PushToken           string      `bson:"push_token,omitempty" json:"-"` // nunca se serializa al cliente tras la creación
    PullURL             string      `bson:"pull_url,omitempty" json:"pullUrl,omitempty"`
    PullMethod          string      `bson:"pull_method,omitempty" json:"pullMethod,omitempty"` // GET | POST
    PullAuthType        APIAuthType `bson:"pull_auth_type,omitempty" json:"pullAuthType,omitempty"`
    PullAuthHeaderName  string      `bson:"pull_auth_header_name,omitempty" json:"pullAuthHeaderName,omitempty"`
    PullAuthValue       string      `bson:"pull_auth_value,omitempty" json:"-"` // cifrado, mismo mecanismo que la API key de IA
    PullIntervalMinutes int         `bson:"pull_interval_minutes,omitempty" json:"pullIntervalMinutes,omitempty"`
    NextPullAt          *time.Time  `bson:"next_pull_at,omitempty" json:"nextPullAt,omitempty"`
    LastPullAt          *time.Time  `bson:"last_pull_at,omitempty" json:"lastPullAt,omitempty"`
    LastPullStatus      string      `bson:"last_pull_status,omitempty" json:"lastPullStatus,omitempty"` // "ok" | "error"
    LastPullError       string      `bson:"last_pull_error,omitempty" json:"lastPullError,omitempty"`
}
```

`PushToken` y `PullAuthValue` nunca se devuelven en el JSON de respuesta (`json:"-"`) —
mismo patrón que ya usa `SystemConfig.APIKey` para la API key de IA
(`backend/internal/models/system_config.go:32`). **Corrección tras autorevisión:** ese
patrón existente NO cifra en reposo — el valor vive en texto plano en MongoDB, solo se
oculta de las respuestas JSON (`json:"-"`) y se enmascara para mostrar (`MaskAPIKey`). Este
diseño sigue exactamente ese mismo nivel de protección (no más, no menos) para
consistencia con el resto del sistema; no se introduce cifrado nuevo. Si se quiere cifrado
real en reposo, es una mejora separada que afecta también a la API key de IA existente, no
algo específico de este cambio.

### 2. Ingesta push de API

Nuevo endpoint **público** (fuera de `protected`, sin JWT — un sistema externo no tiene
sesión de usuario):

```go
// router.go, fuera del grupo protected
api.Post("/ingest/:monitorId", h.Monitor.IngestPush)
```

`MonitorHandler.IngestPush`:
1. Busca el monitor por `:monitorId`.
2. Verifica `monitor.SourceType == SourceAPI && monitor.SourceConfig.Mode == APIModePush`.
3. Compara el header `X-Ingest-Token` contra `monitor.SourceConfig.PushToken` (comparación
   en tiempo constante, `subtle.ConstantTimeCompare` — evita timing attacks). **Nunca en la
   URL** — un token en la URL queda en logs de acceso, historial del navegador y headers
   `Referer` si el sistema externo navega desde ahí.
4. Si coincide, parsea el body como JSON (reutiliza `IngestJSON`, con `RootPath` si está
   configurado) y responde 202.
5. Si no coincide: 401, sin filtrar si el monitor existe (mismo mensaje que token ausente).

Al crear un monitor `api` + `push`, el backend genera `PushToken` (32 bytes aleatorios,
`crypto/rand`, nunca `math/rand`) y lo devuelve **una sola vez** en la respuesta de
creación. Como `PushToken` usa `json:"-"`, el `Monitor` normal jamás lo serializa —
`MonitorHandler.Create` necesita devolver una respuesta ad-hoc
(`fiber.Map{"monitor": monitor, "pushToken": token}`) solo cuando corresponda, en vez de
`c.JSON(monitor)` a secas. El frontend muestra un modal con la URL completa
(`{API_URL}/ingest/{monitorId}`) + el token, con botón de copiar, y advierte que no se
volverá a mostrar.

### 3. Ingesta pull de API

Nuevo `MonitorPuller` (`backend/internal/services/monitor_puller.go`), mismo patrón que
`Scheduler`: cron `* * * * *`, cada minuto busca monitores con
`sourceType == api && sourceConfig.mode == pull && sourceConfig.nextPullAt <= now`.

Por cada uno:
1. Construye el request HTTP (`pullMethod`, `pullUrl`, header de auth según `pullAuthType`).
2. Timeout corto (ej. 30s) — una API externa lenta no debe bloquear el ciclo de polling de
   otros monitores.
3. Extrae el array de datos vía `rootPath` (default: raíz del JSON es el array).
4. Reutiliza `IngestJSON`.
5. Recalcula `nextPullAt = now + pullIntervalMinutes`, incluso si el request falló (evita
   reintentos en bucle apretado contra una API caída).
6. Registra el resultado. **Corrección tras autorevisión:** `IngestionHistory`
   (`MonitorRepository.GetIngestionHistory`) no es una colección de bitácora — es un
   agregado calculado al vuelo sobre los documentos ya insertados de cada monitor
   (`$group` por `_ingested_at`). Un pull fallido no inserta nada, así que ahí no queda
   registro alguno; el fallo sería completamente invisible, repitiendo el problema original
   ("esto no hace nada y no se nota"). En su lugar, `SourceConfig` gana
   `LastPullAt *time.Time`, `LastPullStatus "ok"|"error"`, `LastPullError string` — se
   actualizan en cada ciclo, éxito o no, y se muestran en la página de detalle del monitor
   (`monitors/[id]`). Una bitácora dedicada de intentos (más granular que "el último
   resultado") queda fuera de alcance de este cambio — ver sección final.

### 4. TXT

Se agrega `SourceTXT` al enum. `IngestionService` gana una función `IngestDelimited` que es
`IngestCSV` parametrizado: en vez de `csv.NewReader(file)` a secas, se usa
`reader.Comma = rune(monitor.SourceConfig.Delimiter[0])` cuando está configurado (default:
tab para txt, coma para csv). Cero parsers nuevos — es el mismo código con un campo
configurable. `HasHeaderRow` (default `true`) controla si la primera fila se trata como
encabezados o como datos (si es `false`, se generan nombres `col_1`, `col_2`, …).

`UploadData` gana el `case models.SourceTXT` junto a los existentes.

### 5. Formulario de creación (frontend)

`frontend/src/app/(dashboard)/monitors/page.tsx` — el diálogo de creación gana una sección
condicional según `sourceType`, mismo patrón visual que ya usa Configuración para los
campos de IA (aparecen/desaparecen según el `Select` elegido, sin pasos de wizard):

- `csv` / `txt`: `delimiter` (input de un carácter, default `,` / tab), `hasHeaderRow`
  (switch, default on).
- `excel`: `sheetName` (input opcional, placeholder "primera hoja").
- `json`: `rootPath` (input opcional, placeholder "raíz del JSON").
- `api`: `RadioGroup` push/pull.
  - `push`: sin campos previos — el token se genera al crear.
  - `pull`: `pullUrl`, `pullMethod` (Select GET/POST), `pullAuthType` (Select), campos
    condicionales de auth (`pullAuthHeaderName` + `pullAuthValue`, o solo `pullAuthValue`
    para bearer), `pullIntervalMinutes` (number input, mínimo razonable ej. 5).

Tras crear un monitor `api`+`push`, se abre el modal de URL+token descrito en la sección 2.

`frontend/src/lib/api/monitors.ts` — `create()` gana el campo `sourceConfig` opcional;
`frontend/src/lib/types.ts` — `Monitor`/`SourceType`/nuevo `SourceConfig` type.

### 6. Compatibilidad

Ningún monitor existente tiene `sourceConfig` — todos los campos son `omitempty`/opcionales
y los parsers (`IngestCSV`, etc.) ya usan defaults idénticos al comportamiento actual
cuando `SourceConfig` es `nil` o sus campos están vacíos. No se requiere migración de datos.

## Testing

El backend no tiene tests todavía (ver `CLAUDE.md`) — no se introducen en este cambio salvo
que se pida explícitamente. Verificación manual planeada: crear un monitor de cada tipo
(incluyendo `api` push y pull contra un endpoint de prueba), confirmar ingesta real y que
`IngestionHistory` refleja los resultados.

## Fuera de alcance

- Basic Auth para pull (queda `APIAuthType` extensible, se puede agregar después).
- Paginación automática en pull (si la API externa pagina, hoy se trae solo la primera
  página — hay que decidir un patrón de paginación cuando aparezca el caso real).
- Reintentos con backoff para push/pull fallidos más allá de lo que ya hace el flujo
  actual de reglas (dead queue) — el pull simplemente reintenta en el siguiente ciclo.
- Bitácora granular de cada intento de pull (éxito y error, con timestamp e historial
  completo) — v1 solo guarda el último resultado (`LastPullAt`/`LastPullStatus`/
  `LastPullError`) en el propio monitor, no un log separado de eventos.
- Cifrado en reposo real para secretos (`PushToken`, `PullAuthValue`, y la API key de IA
  existente) — hoy ninguno de los tres está cifrado en MongoDB, solo ocultos de las
  respuestas JSON. Es una mejora de seguridad transversal, no específica de este cambio.
