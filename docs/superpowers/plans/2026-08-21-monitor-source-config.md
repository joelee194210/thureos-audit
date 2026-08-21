# Parámetros de carga configurables por tipo de fuente — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Que los monitores tipo API reciban datos de verdad (push y pull, configurables), que TXT sea un tipo de fuente soportado, y que el formulario de creación pida los parámetros correctos según el tipo elegido — en vez de solo guardar `name`/`description`/`sourceType` y no hacer nada más.

**Architecture:** Se extiende `Monitor` con un `SourceConfig` tipado (un subdocumento cuya forma depende de `sourceType`), manteniendo la relación 1:1 monitor↔fuente existente. El pull de API reutiliza el patrón de `Scheduler` (cron cada minuto, revisa qué está vencido). El push de API es un endpoint público autenticado por token, fuera del grupo `protected`, igual que `/auth/login`. TXT reutiliza el parser de CSV parametrizando el delimitador.

**Tech Stack:** Go 1.22+ / Fiber v2 / MongoDB (backend), Next.js 15 / React 19 / shadcn (frontend). Sin frameworks de test nuevos — ver Global Constraints.

**Spec:** `docs/superpowers/specs/2026-08-21-monitor-source-config-design.md`

## Global Constraints

- **Sin tests automatizados.** El backend no tiene tests (`CLAUDE.md`), y el spec dice explícitamente que no se introducen en este cambio. Cada tarea verifica con comandos concretos (`go build`, `curl`, `mongosh`, `npm run build`) en vez de "escribir el test que falla". Sigue el mismo rigor (comando exacto, salida esperada), solo que sin framework de test.
- **`crypto/rand`, nunca `math/rand`**, para generar `PushToken`.
- **Comparación de tokens en tiempo constante** (`crypto/subtle.ConstantTimeCompare`) para el endpoint de push — nunca `==` en un secreto.
- **`PushToken` y `PullAuthValue` nunca se serializan en respuestas** (`json:"-"` en `models.SourceConfig`). El valor de entrada al crear usa un struct de request separado (`SourceConfigInput`) que sí puede leer `pullAuthValue` del body — mismo patrón que ya usa `SystemConfig.APIKey` vs. el `struct` anónimo de `settings.Put("/ai")` en `router.go`.
- **Sin cifrado en reposo nuevo.** `PullAuthValue`/`PushToken` se guardan en texto plano en MongoDB, ocultos solo de las respuestas JSON — mismo nivel de protección que la API key de IA existente. No se introduce cifrado (ver spec, sección "Fuera de alcance").
- **Compatibilidad total.** Todo campo nuevo es opcional (`omitempty`/puntero). Monitores existentes sin `sourceConfig` siguen funcionando exactamente igual que hoy.
- **Identificadores en inglés, texto de UI en español** — mismo patrón que el resto del código (`/rules`, `/countries`, etc.).
- Después de cada tarea de backend: `cd backend && gofmt -w <archivos tocados> && go build ./... && go vet ./...` debe pasar limpio antes de continuar.
- Después de cada tarea de frontend: `cd frontend && npm run build` debe pasar limpio antes de continuar.
- El backend corre con `go run cmd/server/main.go` (sin hot-reload). Antes de probar manualmente una tarea de backend: `lsof -i :8080 | grep LISTEN` y `kill <PID>` si hay uno corriendo con código viejo, luego relanzar.

---

## Task 1: Modelo de datos — `SourceConfig`, `SourceTXT`, tipos de API

**Files:**
- Modify: `backend/internal/models/monitor.go`

**Interfaces:**
- Produces: `models.SourceTXT` (nuevo `SourceType`), `models.SourceConfig` (struct, campos ver abajo), `models.SourceConfigInput` (struct de request, con `ToSourceConfig() *SourceConfig`), `models.APIMode` (`APIModePush`/`APIModePull`), `models.APIAuthType` (`APIAuthNone`/`APIAuthAPIKey`/`APIAuthBearer`).

- [ ] **Step 1: Reemplazar el contenido de `monitor.go`**

Reemplaza el archivo completo (el actual tiene `SourceType`, `FieldType`, `SchemaField`, `Monitor`, `CreateMonitorRequest` — se agregan los tipos nuevos y se eliminan `APIEndpoint`/`Schedule` de `Monitor`, que están declarados pero nunca se leen en ningún otro lugar del código, solo se escribían en `Create` sin efecto):

```go
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SourceType string

const (
	SourceCSV   SourceType = "csv"
	SourceExcel SourceType = "excel"
	SourceJSON  SourceType = "json"
	SourceTXT   SourceType = "txt"
	SourceAPI   SourceType = "api"
)

type FieldType string

const (
	FieldString  FieldType = "string"
	FieldNumber  FieldType = "number"
	FieldDate    FieldType = "date"
	FieldBoolean FieldType = "boolean"
)

type SchemaField struct {
	Name     string    `bson:"name" json:"name"`
	Type     FieldType `bson:"type" json:"type"`
	Required bool      `bson:"required" json:"required"`
	Sample   string    `bson:"sample" json:"sample"`
}

type APIMode string

const (
	APIModePush APIMode = "push"
	APIModePull APIMode = "pull"
)

type APIAuthType string

const (
	APIAuthNone   APIAuthType = "none"
	APIAuthAPIKey APIAuthType = "api_key_header"
	APIAuthBearer APIAuthType = "bearer"
)

// SourceConfig holds ingestion parameters specific to a monitor's SourceType.
// Which fields apply depends on SourceType — see the design spec. Every field
// is optional so existing monitors (created before this struct existed)
// behave exactly as before.
type SourceConfig struct {
	// csv / txt
	Delimiter    string `bson:"delimiter,omitempty" json:"delimiter,omitempty"`
	HasHeaderRow *bool  `bson:"has_header_row,omitempty" json:"hasHeaderRow,omitempty"`

	// excel
	SheetName string `bson:"sheet_name,omitempty" json:"sheetName,omitempty"`

	// json / api (nested array extraction)
	RootPath string `bson:"root_path,omitempty" json:"rootPath,omitempty"`

	// api
	Mode                APIMode     `bson:"mode,omitempty" json:"mode,omitempty"`
	PushToken           string      `bson:"push_token,omitempty" json:"-"`
	PullURL             string      `bson:"pull_url,omitempty" json:"pullUrl,omitempty"`
	PullMethod          string      `bson:"pull_method,omitempty" json:"pullMethod,omitempty"`
	PullAuthType        APIAuthType `bson:"pull_auth_type,omitempty" json:"pullAuthType,omitempty"`
	PullAuthHeaderName  string      `bson:"pull_auth_header_name,omitempty" json:"pullAuthHeaderName,omitempty"`
	PullAuthValue       string      `bson:"pull_auth_value,omitempty" json:"-"`
	PullIntervalMinutes int         `bson:"pull_interval_minutes,omitempty" json:"pullIntervalMinutes,omitempty"`
	NextPullAt          *time.Time  `bson:"next_pull_at,omitempty" json:"nextPullAt,omitempty"`
	LastPullAt          *time.Time  `bson:"last_pull_at,omitempty" json:"lastPullAt,omitempty"`
	LastPullStatus      string      `bson:"last_pull_status,omitempty" json:"lastPullStatus,omitempty"`
	LastPullError       string      `bson:"last_pull_error,omitempty" json:"lastPullError,omitempty"`
}

// SourceConfigInput is the client-facing shape for creating a monitor's
// source configuration. Unlike SourceConfig, PullAuthValue has no `json:"-"`
// here — this struct is only ever used to receive input, never to respond,
// so there is no leak risk. PushToken and the LastPull*/NextPullAt fields
// are server-controlled and intentionally absent: a client cannot set them.
type SourceConfigInput struct {
	Delimiter           string      `json:"delimiter,omitempty"`
	HasHeaderRow        *bool       `json:"hasHeaderRow,omitempty"`
	SheetName           string      `json:"sheetName,omitempty"`
	RootPath            string      `json:"rootPath,omitempty"`
	Mode                APIMode     `json:"mode,omitempty"`
	PullURL             string      `json:"pullUrl,omitempty"`
	PullMethod          string      `json:"pullMethod,omitempty"`
	PullAuthType        APIAuthType `json:"pullAuthType,omitempty"`
	PullAuthHeaderName  string      `json:"pullAuthHeaderName,omitempty"`
	PullAuthValue       string      `json:"pullAuthValue,omitempty"`
	PullIntervalMinutes int         `json:"pullIntervalMinutes,omitempty"`
}

// ToSourceConfig converts client input into the stored shape. Returns nil
// for a nil receiver so callers can do `monitor.SourceConfig = req.SourceConfig.ToSourceConfig()`
// unconditionally, even when the client sent no sourceConfig at all.
func (in *SourceConfigInput) ToSourceConfig() *SourceConfig {
	if in == nil {
		return nil
	}
	return &SourceConfig{
		Delimiter:           in.Delimiter,
		HasHeaderRow:        in.HasHeaderRow,
		SheetName:           in.SheetName,
		RootPath:            in.RootPath,
		Mode:                in.Mode,
		PullURL:             in.PullURL,
		PullMethod:          in.PullMethod,
		PullAuthType:        in.PullAuthType,
		PullAuthHeaderName:  in.PullAuthHeaderName,
		PullAuthValue:       in.PullAuthValue,
		PullIntervalMinutes: in.PullIntervalMinutes,
	}
}

type Monitor struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name         string             `bson:"name" json:"name"`
	Description  string             `bson:"description" json:"description"`
	SourceType   SourceType         `bson:"source_type" json:"sourceType"`
	SourceConfig *SourceConfig      `bson:"source_config,omitempty" json:"sourceConfig,omitempty"`
	Schema       []SchemaField      `bson:"schema" json:"schema"`
	CollectionID string             `bson:"collection_id" json:"collectionId"`
	OwnerID      primitive.ObjectID `bson:"owner_id" json:"ownerId"`
	RecordCount  int64              `bson:"record_count" json:"recordCount"`
	LastIngested *time.Time         `bson:"last_ingested,omitempty" json:"lastIngested,omitempty"`
	CreatedAt    time.Time          `bson:"created_at" json:"createdAt"`
	UpdatedAt    time.Time          `bson:"updated_at" json:"updatedAt"`
}

type CreateMonitorRequest struct {
	Name         string             `json:"name"`
	Description  string             `json:"description"`
	SourceType   SourceType         `json:"sourceType"`
	SourceConfig *SourceConfigInput `json:"sourceConfig,omitempty"`
}
```

- [ ] **Step 2: Verificar que compila**

Run: `cd backend && gofmt -w internal/models/monitor.go && go build ./... 2>&1`
Expected: dos errores de compilación en `backend/internal/handlers/monitor_handler.go` (líneas que usan `req.APIEndpoint`/`req.Schedule`, que ya no existen). Eso es esperado — se corrige en el Task 4. Confirma que el ÚNICO error es ese, no otro.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/models/monitor.go
git commit -m "feat(monitors): agregar SourceConfig tipado y SourceType txt al modelo"
```

---

## Task 2: Ingesta CSV/TXT con delimitador y encabezado configurables

**Files:**
- Modify: `backend/internal/services/ingestion_service.go`

**Interfaces:**
- Consumes: `models.SourceConfig.Delimiter`, `models.SourceConfig.HasHeaderRow` (Task 1).
- Produces: `IngestionService.IngestTXT(ctx, monitor, file) (int, error)` — conectado al switch de `UploadData` en la Task 4.

- [ ] **Step 1: Reemplazar `IngestCSV` por una versión parametrizada + agregar `IngestTXT`**

En `ingestion_service.go`, reemplaza la función `IngestCSV` completa (líneas ~29-75 del archivo actual, desde `func (s *IngestionService) IngestCSV` hasta su `}` de cierre) por:

```go
// IngestCSV parses a CSV file, detects schema, and stores data
func (s *IngestionService) IngestCSV(ctx context.Context, monitor *models.Monitor, file multipart.File) (int, error) {
	return s.ingestDelimited(ctx, monitor, file, ',')
}

// IngestTXT parses a delimited text file (default: tab-separated), detects
// schema, and stores data. Same parser as IngestCSV — only the default
// delimiter differs, and SourceConfig.Delimiter always overrides either default.
func (s *IngestionService) IngestTXT(ctx context.Context, monitor *models.Monitor, file multipart.File) (int, error) {
	return s.ingestDelimited(ctx, monitor, file, '\t')
}

func (s *IngestionService) ingestDelimited(ctx context.Context, monitor *models.Monitor, file multipart.File, defaultDelimiter rune) (int, error) {
	delimiter := defaultDelimiter
	hasHeaderRow := true
	if monitor.SourceConfig != nil {
		if monitor.SourceConfig.Delimiter != "" {
			delimiter = rune(monitor.SourceConfig.Delimiter[0])
		}
		if monitor.SourceConfig.HasHeaderRow != nil {
			hasHeaderRow = *monitor.SourceConfig.HasHeaderRow
		}
	}

	reader := csv.NewReader(file)
	reader.Comma = delimiter

	var headers []string
	var allRows [][]string

	if hasHeaderRow {
		h, err := reader.Read()
		if err != nil {
			return 0, fmt.Errorf("reading headers: %w", err)
		}
		headers = h
	}

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("reading row: %w", err)
		}
		if headers == nil {
			// No header row: synthesize column names from the first row's width.
			headers = make([]string, len(row))
			for i := range row {
				headers[i] = fmt.Sprintf("col_%d", i+1)
			}
		}
		allRows = append(allRows, row)
	}

	if len(monitor.Schema) == 0 {
		monitor.Schema = detectSchemaFromCSV(headers, allRows)
	}

	documents := make([]interface{}, 0, len(allRows))
	for _, row := range allRows {
		doc := bson.M{"_ingested_at": time.Now()}
		for i, header := range headers {
			if i < len(row) {
				doc[sanitizeFieldName(header)] = parseValue(row[i], monitor.Schema, header)
			}
		}
		documents = append(documents, doc)
	}

	count, err := s.monitorRepo.InsertData(ctx, monitor.CollectionID, documents)
	if err != nil {
		return 0, fmt.Errorf("inserting data: %w", err)
	}

	now := time.Now()
	s.monitorRepo.Update(ctx, monitor.ID, bson.M{
		"schema":        monitor.Schema,
		"record_count":  monitor.RecordCount + int64(count),
		"last_ingested": now,
	})

	return count, nil
}
```

No se necesitan imports nuevos — `csv`, `fmt`, `io`, `time`, `bson` ya están importados en el archivo.

- [ ] **Step 2: Verificar que compila**

Run: `cd backend && gofmt -w internal/services/ingestion_service.go && go build ./... 2>&1`
Expected: mismos dos errores del Task 1 (en `monitor_handler.go`, aún no corregidos) y ningún error nuevo en `ingestion_service.go`.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/services/ingestion_service.go
git commit -m "feat(monitors): parametrizar delimitador/encabezado en CSV, agregar IngestTXT"
```

---

## Task 3: Excel `sheetName`, JSON `rootPath`, e `IngestJSON` reutilizable por HTTP

**Files:**
- Modify: `backend/internal/services/ingestion_service.go`

**Interfaces:**
- Consumes: `models.SourceConfig.SheetName`, `models.SourceConfig.RootPath` (Task 1).
- Produces: `IngestionService.IngestJSON(ctx, monitor, file io.Reader) (int, error)` — nota el cambio de firma de `multipart.File` a `io.Reader`, usado por Task 5 (push, desde `bytes.Reader`) y Task 6 (pull, desde `http.Response.Body`). `extractRootPath` y `toRecordSlice` (helpers internos).

- [ ] **Step 1: Agregar `sheetName` a `IngestExcel`**

En la función `IngestExcel`, ubica estas dos líneas:

```go
	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
```

Reemplázalas por:

```go
	sheetName := f.GetSheetName(0)
	if monitor.SourceConfig != nil && monitor.SourceConfig.SheetName != "" {
		sheetName = monitor.SourceConfig.SheetName
	}
	rows, err := f.GetRows(sheetName)
```

- [ ] **Step 2: Reemplazar `IngestJSON` por una versión con `rootPath` y firma `io.Reader`**

Reemplaza la función `IngestJSON` completa por:

```go
// IngestJSON parses a JSON document, optionally drilling into a nested
// array via SourceConfig.RootPath, detects schema, and stores data.
// Takes io.Reader (not multipart.File) so it can be reused for HTTP
// request/response bodies from API push and pull ingestion, not just
// uploaded files — multipart.File already satisfies io.Reader, so existing
// callers (UploadData) need no changes.
func (s *IngestionService) IngestJSON(ctx context.Context, monitor *models.Monitor, file io.Reader) (int, error) {
	raw, err := io.ReadAll(file)
	if err != nil {
		return 0, fmt.Errorf("reading JSON: %w", err)
	}

	var parsed interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return 0, fmt.Errorf("parsing JSON: %w", err)
	}

	rootPath := ""
	if monitor.SourceConfig != nil {
		rootPath = monitor.SourceConfig.RootPath
	}

	extracted, err := extractRootPath(parsed, rootPath)
	if err != nil {
		return 0, err
	}

	records, err := toRecordSlice(extracted)
	if err != nil {
		return 0, err
	}

	if len(records) == 0 {
		return 0, nil
	}

	if len(monitor.Schema) == 0 {
		monitor.Schema = detectSchemaFromJSON(records)
	}

	documents := make([]interface{}, 0, len(records))
	for _, record := range records {
		record["_ingested_at"] = time.Now()
		documents = append(documents, record)
	}

	count, err := s.monitorRepo.InsertData(ctx, monitor.CollectionID, documents)
	if err != nil {
		return 0, fmt.Errorf("inserting data: %w", err)
	}

	now := time.Now()
	s.monitorRepo.Update(ctx, monitor.ID, bson.M{
		"schema":        monitor.Schema,
		"record_count":  monitor.RecordCount + int64(count),
		"last_ingested": now,
	})

	return count, nil
}

// extractRootPath navigates a parsed JSON value by dot-separated path
// (e.g. "data.records") and returns the value found there. An empty path
// returns v unchanged — this is the default, matching today's behavior of
// treating the JSON body itself as the records.
func extractRootPath(v interface{}, path string) (interface{}, error) {
	if path == "" {
		return v, nil
	}
	current := v
	for _, key := range strings.Split(path, ".") {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("rootPath %q: expected an object before key %q, got %T", path, key, current)
		}
		val, exists := m[key]
		if !exists {
			return nil, fmt.Errorf("rootPath %q: key %q not found", path, key)
		}
		current = val
	}
	return current, nil
}

// toRecordSlice coerces a parsed JSON value into a slice of records. Matches
// the pre-existing IngestJSON fallback: an array of objects ingests as-is,
// a single bare object ingests as one record.
func toRecordSlice(v interface{}) ([]map[string]interface{}, error) {
	switch val := v.(type) {
	case []interface{}:
		records := make([]map[string]interface{}, 0, len(val))
		for _, item := range val {
			m, ok := item.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("expected an array of objects, got an element of type %T", item)
			}
			records = append(records, m)
		}
		return records, nil
	case map[string]interface{}:
		return []map[string]interface{}{val}, nil
	default:
		return nil, fmt.Errorf("expected a JSON array or object at the root (or at rootPath), got %T", v)
	}
}
```

- [ ] **Step 3: Agregar el import `strings` si falta**

`ingestion_service.go` ya importa `"strings"` (usado por `sanitizeFieldName`/`inferType`) — confirma con `grep -n '"strings"' backend/internal/services/ingestion_service.go` que sigue ahí; no se necesita ningún import nuevo.

- [ ] **Step 4: Verificar que compila**

Run: `cd backend && gofmt -w internal/services/ingestion_service.go && go build ./... 2>&1`
Expected: mismos dos errores de `monitor_handler.go` (aún pendientes de Task 4), ninguno nuevo. Si aparece un error sobre `IngestJSON` esperando `multipart.File` en algún call site — no debería, porque `multipart.File` satisface `io.Reader`, pero si aparece, es en `monitor_handler.go` y se resuelve solo al terminar Task 4 (ese archivo se reescribe ahí).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/ingestion_service.go
git commit -m "feat(monitors): sheetName en Excel, rootPath en JSON, IngestJSON acepta io.Reader"
```

---

## Task 4: Creación de monitor — acepta `SourceConfig`, genera `PushToken`

**Files:**
- Modify: `backend/internal/handlers/monitor_handler.go`

**Interfaces:**
- Consumes: `models.SourceConfigInput.ToSourceConfig()` (Task 1).
- Produces: respuesta de `POST /monitors` — `Monitor` normal, o `{"monitor": Monitor, "pushToken": string}` cuando `sourceType=api` y `sourceConfig.mode=push`. Usado por Task 8 (frontend).

- [ ] **Step 1: Reemplazar el método `Create`**

Reemplaza la función `Create` completa (la que arma `monitor := &models.Monitor{...}` con `APIEndpoint`/`Schedule`, que ya no existen en el struct tras el Task 1) por:

```go
func (h *MonitorHandler) Create(c *fiber.Ctx) error {
	var req models.CreateMonitorRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	userID, _ := primitive.ObjectIDFromHex(c.Locals("userId").(string))

	monitor := &models.Monitor{
		Name:         req.Name,
		Description:  req.Description,
		SourceType:   req.SourceType,
		SourceConfig: req.SourceConfig.ToSourceConfig(),
		CollectionID: primitive.NewObjectID().Hex(),
		OwnerID:      userID,
	}

	var pushToken string
	if monitor.SourceType == models.SourceAPI && monitor.SourceConfig != nil && monitor.SourceConfig.Mode == models.APIModePush {
		token, err := generatePushToken()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "generating push token: " + err.Error()})
		}
		monitor.SourceConfig.PushToken = token
		pushToken = token
	}

	if err := h.monitorRepo.Create(c.Context(), monitor); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "creating monitor: " + err.Error()})
	}

	if pushToken != "" {
		// PushToken has json:"-" on Monitor, so it never round-trips through
		// the normal response. This is the one moment it's shown — the
		// frontend must display and let the user copy it now.
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"monitor": monitor, "pushToken": pushToken})
	}
	return c.Status(fiber.StatusCreated).JSON(monitor)
}

// generatePushToken returns a 64-character hex-encoded random token
// (32 bytes of entropy) for authenticating API push ingestion requests.
func generatePushToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
```

- [ ] **Step 2: Conectar `IngestTXT` (Task 2) al switch de `UploadData`**

En `UploadData`, ubica:

```go
	var count int
	switch monitor.SourceType {
	case models.SourceCSV:
		count, err = h.ingestionService.IngestCSV(c.Context(), monitor, f)
	case models.SourceJSON:
		count, err = h.ingestionService.IngestJSON(c.Context(), monitor, f)
	case models.SourceExcel:
		count, err = h.ingestionService.IngestExcel(c.Context(), monitor, f)
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unsupported source type"})
	}
```

Reemplázala por:

```go
	var count int
	switch monitor.SourceType {
	case models.SourceCSV:
		count, err = h.ingestionService.IngestCSV(c.Context(), monitor, f)
	case models.SourceJSON:
		count, err = h.ingestionService.IngestJSON(c.Context(), monitor, f)
	case models.SourceExcel:
		count, err = h.ingestionService.IngestExcel(c.Context(), monitor, f)
	case models.SourceTXT:
		count, err = h.ingestionService.IngestTXT(c.Context(), monitor, f)
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unsupported source type"})
	}
```

Sin esto, un monitor `txt` seguiría cayendo en el `default` — exactamente el bug original, solo que ahora para un tipo distinto.

- [ ] **Step 3: Agregar imports**

Al bloque `import` de `monitor_handler.go`, agrega:

```go
	"crypto/rand"
	"encoding/hex"
```

(Van en orden alfabético dentro del grupo de la librería estándar, junto a los imports existentes.)

- [ ] **Step 4: Verificar que compila**

Run: `cd backend && gofmt -w internal/handlers/monitor_handler.go && go build ./... && go vet ./... 2>&1`
Expected: compila sin ningún error.

- [ ] **Step 5: Verificar en ejecución — monitor csv (sin cambios de comportamiento)**

```bash
lsof -i :8080 | grep LISTEN   # anota el PID si hay uno, mátalo
cd backend && go run cmd/server/main.go &
sleep 3
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d '{"email":"<tu admin>","password":"<tu password>"}' | jq -r .token)
curl -s -X POST http://localhost:8080/api/v1/monitors -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"name":"Test CSV","description":"","sourceType":"csv"}' | jq .
```

Expected: JSON del monitor creado, `sourceConfig` ausente o `null` (no se envió), sin campo `pushToken` en el nivel raíz (porque no es `api`+`push`).

- [ ] **Step 6: Verificar en ejecución — monitor txt sube y se ingesta**

```bash
MONITOR_ID=$(curl -s -X POST http://localhost:8080/api/v1/monitors -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"name":"Test TXT","description":"","sourceType":"txt"}' | jq -r .id)
printf 'monto\tcliente\n100\tACME\n250\tBeta\n' > /tmp/test.txt
curl -s -X POST "http://localhost:8080/api/v1/monitors/$MONITOR_ID/upload" -H "Authorization: Bearer $TOKEN" -F "file=@/tmp/test.txt" | jq .
```

Expected: `{"recordsIngested":2,...}` — confirma que el delimitador tab por defecto funcionó sin configurar nada.

- [ ] **Step 7: Verificar en ejecución — monitor api+push genera token**

```bash
curl -s -X POST http://localhost:8080/api/v1/monitors -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"name":"Test Push","description":"","sourceType":"api","sourceConfig":{"mode":"push"}}' | jq .
```

Expected: `{"monitor": {..., "sourceConfig": {"mode": "push"} /* sin pushToken visible aquí */}, "pushToken": "<64 caracteres hex>"}`. Confirma con `mongosh` que si vuelves a pedir el monitor (`GET /monitors/:id`), el campo `pushToken` NO aparece en absoluto (por el `json:"-"`):

```bash
docker exec -i thureos-mongo mongosh "mongodb://localhost:27017/thureos_compliance" --quiet --eval 'db.monitors.findOne({name:"Test Push"}, {source_config:1})'
```

Expected: el documento SÍ tiene `push_token` guardado en Mongo (texto plano, es el diseño aceptado), pero la respuesta HTTP de `GET /monitors/:id` no lo incluye.

- [ ] **Step 8: Limpiar los monitores de prueba**

```bash
docker exec -i thureos-mongo mongosh "mongodb://localhost:27017/thureos_compliance" --quiet --eval '
db.monitors.find({name: {$in: ["Test CSV", "Test TXT", "Test Push"]}}).forEach(m => {
  db.getCollection("data_" + m.collection_id).drop();
});
db.monitors.deleteMany({name: {$in: ["Test CSV", "Test TXT", "Test Push"]}});
'
rm -f /tmp/test.txt
```

- [ ] **Step 9: Commit**

```bash
git add backend/internal/handlers/monitor_handler.go
git commit -m "feat(monitors): Create acepta sourceConfig y genera pushToken para api+push"
```

---

## Task 5: Endpoint público de ingesta push

**Files:**
- Modify: `backend/internal/handlers/monitor_handler.go`
- Modify: `backend/internal/router/router.go`

**Interfaces:**
- Consumes: `IngestionService.IngestJSON(ctx, monitor, io.Reader)` (Task 3), `models.SourceConfig.PushToken` (Task 1).
- Produces: `POST /api/v1/ingest/:monitorId` (público, header `X-Ingest-Token`).

- [ ] **Step 1: Agregar el handler `IngestPush`**

Al final de `monitor_handler.go`, agrega:

```go
// IngestPush receives data pushed by an external system for an API+push
// monitor. Public route (no JWT) — authenticated by a per-monitor secret
// token instead, since an external system has no user session.
func (h *MonitorHandler) IngestPush(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("monitorId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}

	monitor, err := h.monitorRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
	}

	if monitor.SourceType != models.SourceAPI || monitor.SourceConfig == nil || monitor.SourceConfig.Mode != models.APIModePush {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "monitor is not configured for API push"})
	}

	token := c.Get("X-Ingest-Token")
	if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(monitor.SourceConfig.PushToken)) != 1 {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or missing ingest token"})
	}

	count, err := h.ingestionService.IngestJSON(c.Context(), monitor, bytes.NewReader(c.Body()))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	queued := false
	if h.jobQueue != nil {
		err = h.jobQueue.Enqueue(c.Context(), services.EvalJob{MonitorID: id.Hex()})
		queued = err == nil
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"recordsIngested":  count,
		"evaluationQueued": queued,
	})
}
```

- [ ] **Step 2: Agregar imports**

Agrega al bloque `import` de `monitor_handler.go`:

```go
	"bytes"
	"crypto/subtle"
```

- [ ] **Step 3: Registrar la ruta pública en `router.go`**

En `router.go`, ubica el bloque de rutas públicas (donde están `auth.Post("/register", ...)` y `auth.Post("/login", ...)`, antes de `protected := api.Group(...)`). Justo después de ese bloque `auth`, agrega:

```go
	// Public ingest endpoint for API+push monitors — authenticated by a
	// per-monitor secret token (X-Ingest-Token header), not JWT. An
	// external system pushing data has no user session.
	api.Post("/ingest/:monitorId", h.Monitor.IngestPush)
```

(Debe quedar registrada sobre `api`, no sobre `protected` — igual que `auth`, para que `middleware.AuthRequired` no la intercepte.)

- [ ] **Step 4: Verificar que compila**

Run: `cd backend && gofmt -w internal/handlers/monitor_handler.go internal/router/router.go && go build ./... && go vet ./... 2>&1`
Expected: compila sin errores.

- [ ] **Step 5: Verificar en ejecución — token correcto vs. incorrecto**

Reinicia el backend (mata el proceso en :8080, vuelve a correr `go run cmd/server/main.go`), luego:

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d '{"email":"<admin>","password":"<password>"}' | jq -r .token)
RESP=$(curl -s -X POST http://localhost:8080/api/v1/monitors -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"name":"Test Push2","description":"","sourceType":"api","sourceConfig":{"mode":"push"}}')
MONITOR_ID=$(echo "$RESP" | jq -r .monitor.id)
PUSH_TOKEN=$(echo "$RESP" | jq -r .pushToken)

echo "=== sin token (debe fallar 401) ==="
curl -s -o /dev/null -w "%{http_code}\n" -X POST "http://localhost:8080/api/v1/ingest/$MONITOR_ID" -H "Content-Type: application/json" -d '[{"monto": 100}]'

echo "=== token incorrecto (debe fallar 401) ==="
curl -s -o /dev/null -w "%{http_code}\n" -X POST "http://localhost:8080/api/v1/ingest/$MONITOR_ID" -H "X-Ingest-Token: wrong" -H "Content-Type: application/json" -d '[{"monto": 100}]'

echo "=== token correcto (debe aceptar, 202) ==="
curl -s -w "\n%{http_code}\n" -X POST "http://localhost:8080/api/v1/ingest/$MONITOR_ID" -H "X-Ingest-Token: $PUSH_TOKEN" -H "Content-Type: application/json" -d '[{"monto": 100}, {"monto": 250}]'
```

Expected: 401, 401, luego `{"recordsIngested":2,"evaluationQueued":true} 202`. Verifica los datos insertados:

```bash
docker exec -i thureos-mongo mongosh "mongodb://localhost:27017/thureos_compliance" --quiet --eval "db.getCollection('data_' + db.monitors.findOne({name:'Test Push2'}).collection_id).find().toArray()"
```

Expected: 2 documentos con `monto: 100` y `monto: 250`.

- [ ] **Step 6: Limpiar**

```bash
docker exec -i thureos-mongo mongosh "mongodb://localhost:27017/thureos_compliance" --quiet --eval '
var m = db.monitors.findOne({name:"Test Push2"});
db.getCollection("data_" + m.collection_id).drop();
db.monitors.deleteOne({name:"Test Push2"});
'
```

- [ ] **Step 7: Commit**

```bash
git add backend/internal/handlers/monitor_handler.go backend/internal/router/router.go
git commit -m "feat(monitors): endpoint público de ingesta push para monitores api"
```

---

## Task 6: `MonitorPuller` — pull programado de API

**Files:**
- Create: `backend/internal/services/monitor_puller.go`
- Modify: `backend/internal/repository/monitor_repo.go`
- Modify: `backend/cmd/server/main.go`

**Interfaces:**
- Consumes: `MonitorRepository.Update` (existente), `IngestionService.IngestJSON(ctx, monitor, io.Reader)` (Task 3).
- Produces: `MonitorRepository.FindPullMonitors(ctx) ([]models.Monitor, error)`, `services.NewMonitorPuller(monitorRepo, ingestionService, jobQueue) *MonitorPuller`, `(*MonitorPuller).Start(ctx)`.

- [ ] **Step 1: Agregar `FindPullMonitors` al repositorio**

En `monitor_repo.go`, después de la función `Delete`, agrega:

```go
// FindPullMonitors returns all API-source monitors configured for pull mode.
// The caller filters by NextPullAt to decide which are actually due —
// same split as RuleRepository.FindScheduled + Scheduler.checkAndFireRules.
func (r *MonitorRepository) FindPullMonitors(ctx context.Context) ([]models.Monitor, error) {
	cursor, err := r.col.Find(ctx, bson.M{
		"source_type":        models.SourceAPI,
		"source_config.mode": models.APIModePull,
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var monitors []models.Monitor
	if err := cursor.All(ctx, &monitors); err != nil {
		return nil, err
	}
	return monitors, nil
}
```

- [ ] **Step 2: Crear `monitor_puller.go`**

```go
package services

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
	"go.mongodb.org/mongo-driver/bson"
)

// MonitorPuller periodically fetches data from external APIs for monitors
// configured with SourceType "api" and SourceConfig.Mode "pull". Same
// polling pattern as Scheduler: one cron entry checks every minute for
// monitors whose NextPullAt is due, instead of one cron entry per monitor.
type MonitorPuller struct {
	cron             *cron.Cron
	monitorRepo      *repository.MonitorRepository
	ingestionService *IngestionService
	jobQueue         *JobQueue
	httpClient       *http.Client
	mu               sync.RWMutex
	running          bool
}

func NewMonitorPuller(monitorRepo *repository.MonitorRepository, ingestionService *IngestionService, jobQueue *JobQueue) *MonitorPuller {
	return &MonitorPuller{
		monitorRepo:      monitorRepo,
		ingestionService: ingestionService,
		jobQueue:         jobQueue,
		httpClient:       &http.Client{Timeout: 30 * time.Second},
	}
}

// Start launches the puller. Blocks until ctx is cancelled — call with `go`.
func (p *MonitorPuller) Start(ctx context.Context) {
	p.cron = cron.New()
	p.cron.AddFunc("* * * * *", func() {
		p.checkAndPull(ctx)
	})

	p.mu.Lock()
	p.running = true
	p.mu.Unlock()

	log.Println("Monitor puller started — checking API pull monitors every minute")
	p.cron.Start()

	<-ctx.Done()

	log.Println("Monitor puller stopping...")
	stopCtx := p.cron.Stop()
	<-stopCtx.Done()

	p.mu.Lock()
	p.running = false
	p.mu.Unlock()
	log.Println("Monitor puller stopped")
}

func (p *MonitorPuller) checkAndPull(ctx context.Context) {
	monitors, err := p.monitorRepo.FindPullMonitors(ctx)
	if err != nil {
		log.Printf("Monitor puller: error finding pull monitors: %v", err)
		return
	}

	now := time.Now()
	for _, m := range monitors {
		if m.SourceConfig.NextPullAt != nil && now.Before(*m.SourceConfig.NextPullAt) {
			continue
		}
		go func(monitor models.Monitor) {
			p.pullOne(ctx, monitor)
		}(m)
	}
}

func (p *MonitorPuller) pullOne(ctx context.Context, monitor models.Monitor) {
	cfg := monitor.SourceConfig
	method := cfg.PullMethod
	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequestWithContext(ctx, method, cfg.PullURL, nil)
	if err != nil {
		p.recordResult(ctx, monitor, 0, fmt.Errorf("building request: %w", err))
		return
	}

	switch cfg.PullAuthType {
	case models.APIAuthAPIKey:
		req.Header.Set(cfg.PullAuthHeaderName, cfg.PullAuthValue)
	case models.APIAuthBearer:
		req.Header.Set("Authorization", "Bearer "+cfg.PullAuthValue)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		p.recordResult(ctx, monitor, 0, fmt.Errorf("requesting %s: %w", cfg.PullURL, err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		p.recordResult(ctx, monitor, 0, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body)))
		return
	}

	count, err := p.ingestionService.IngestJSON(ctx, &monitor, resp.Body)
	if err != nil {
		p.recordResult(ctx, monitor, 0, fmt.Errorf("ingesting response: %w", err))
		return
	}

	if p.jobQueue != nil {
		_ = p.jobQueue.Enqueue(ctx, EvalJob{MonitorID: monitor.ID.Hex()})
	}

	p.recordResult(ctx, monitor, count, nil)
}

func (p *MonitorPuller) nextPullTime(monitor models.Monitor) time.Time {
	interval := monitor.SourceConfig.PullIntervalMinutes
	if interval < 1 {
		interval = 60
	}
	return time.Now().Add(time.Duration(interval) * time.Minute)
}

// recordResult writes LastPullAt/LastPullStatus/LastPullError and recomputes
// NextPullAt — on both success and failure, so a failing external API
// doesn't get hammered every minute; it waits a full interval before retrying.
func (p *MonitorPuller) recordResult(ctx context.Context, monitor models.Monitor, count int, pullErr error) {
	now := time.Now()
	next := p.nextPullTime(monitor)
	status := "ok"
	errMsg := ""
	if pullErr != nil {
		status = "error"
		errMsg = pullErr.Error()
		log.Printf("Monitor puller: pull failed for monitor %s: %v", monitor.Name, pullErr)
	} else {
		log.Printf("Monitor puller: pulled %d records for monitor %s", count, monitor.Name)
	}

	err := p.monitorRepo.Update(ctx, monitor.ID, bson.M{
		"source_config.last_pull_at":     now,
		"source_config.last_pull_status": status,
		"source_config.last_pull_error":  errMsg,
		"source_config.next_pull_at":     next,
	})
	if err != nil {
		log.Printf("Monitor puller: failed to record pull result for monitor %s: %v", monitor.ID.Hex(), err)
	}
}

// Status reports whether the puller's cron loop is running — same shape as
// Scheduler.Status(), consumed by the /settings endpoint if it's ever added there.
func (p *MonitorPuller) Status() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return map[string]interface{}{"running": p.running}
}
```

- [ ] **Step 3: Wiring en `main.go`**

Después del bloque del scheduler (`go scheduler.Start(schedulerCtx)`), agrega:

```go
	// Initialize monitor puller (API pull-mode monitors)
	monitorPuller := services.NewMonitorPuller(monitorRepo, ingestionService, jobQueue)
	pullerCtx, pullerCancel := context.WithCancel(context.Background())
	defer pullerCancel()
	go monitorPuller.Start(pullerCtx)
```

- [ ] **Step 4: Verificar que compila**

Run: `cd backend && gofmt -w internal/services/monitor_puller.go internal/repository/monitor_repo.go cmd/server/main.go && go build ./... && go vet ./... 2>&1`
Expected: compila sin errores.

- [ ] **Step 5: Verificar en ejecución — pull real contra un servidor de prueba**

Levanta un servidor HTTP mínimo que sirva JSON estático (para no depender de una API externa real):

```bash
mkdir -p /tmp/pull-test && cat > /tmp/pull-test/data.json <<'EOF'
[{"monto": 500, "cliente": "ACME"}, {"monto": 1200, "cliente": "Beta"}]
EOF
(cd /tmp/pull-test && python3 -m http.server 9091 &)
sleep 1
```

Reinicia el backend (mata proceso :8080, relanza), luego crea el monitor pull con intervalo de 1 minuto:

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d '{"email":"<admin>","password":"<password>"}' | jq -r .token)
curl -s -X POST http://localhost:8080/api/v1/monitors -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{
  "name": "Test Pull",
  "description": "",
  "sourceType": "api",
  "sourceConfig": {"mode": "pull", "pullUrl": "http://localhost:9091/data.json", "pullMethod": "GET", "pullAuthType": "none", "pullIntervalMinutes": 1}
}' | jq .
```

Espera hasta 90 segundos (el cron corre cada minuto), luego:

```bash
docker exec -i thureos-mongo mongosh "mongodb://localhost:27017/thureos_compliance" --quiet --eval '
var m = db.monitors.findOne({name:"Test Pull"});
print("lastPullStatus: " + m.source_config.last_pull_status);
print("recordCount: " + m.record_count);
db.getCollection("data_" + m.collection_id).find().toArray();
'
```

Expected: `lastPullStatus: ok`, `recordCount: 2`, dos documentos con `monto`/`cliente` correctos.

- [ ] **Step 6: Verificar el caso de error (URL inválida)**

```bash
curl -s -X POST http://localhost:8080/api/v1/monitors -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{
  "name": "Test Pull Fail",
  "description": "",
  "sourceType": "api",
  "sourceConfig": {"mode": "pull", "pullUrl": "http://localhost:9999/nope", "pullMethod": "GET", "pullAuthType": "none", "pullIntervalMinutes": 1}
}' | jq .
```

Espera hasta 90 segundos, luego:

```bash
docker exec -i thureos-mongo mongosh "mongodb://localhost:27017/thureos_compliance" --quiet --eval 'db.monitors.findOne({name:"Test Pull Fail"}, {source_config:1})'
```

Expected: `last_pull_status: "error"`, `last_pull_error` con un mensaje de conexión rechazada — confirma que el fallo quedó registrado y visible, no silencioso.

- [ ] **Step 7: Limpiar**

```bash
kill %1 2>/dev/null  # el servidor python del step 5
docker exec -i thureos-mongo mongosh "mongodb://localhost:27017/thureos_compliance" --quiet --eval '
db.monitors.find({name: {$in: ["Test Pull", "Test Pull Fail"]}}).forEach(m => {
  db.getCollection("data_" + m.collection_id).drop();
});
db.monitors.deleteMany({name: {$in: ["Test Pull", "Test Pull Fail"]}});
'
rm -rf /tmp/pull-test
```

- [ ] **Step 8: Commit**

```bash
git add backend/internal/services/monitor_puller.go backend/internal/repository/monitor_repo.go backend/cmd/server/main.go
git commit -m "feat(monitors): MonitorPuller para ingesta pull programada de API"
```

---

## Task 7: Frontend — tipos y cliente API

**Files:**
- Modify: `frontend/src/lib/types.ts`
- Modify: `frontend/src/lib/api/monitors.ts`

**Interfaces:**
- Produces: `SourceConfig`, `CreateSourceConfig`, `APIMode`, `APIAuthType` (types), `monitorsApi.create()` con firma nueva.

- [ ] **Step 1: Actualizar `types.ts`**

Ubica esta línea:

```ts
export type SourceType = "csv" | "excel" | "json" | "api";
```

Reemplázala, y todo el bloque de `Monitor`, por:

```ts
export type SourceType = "csv" | "excel" | "json" | "txt" | "api";

export type APIMode = "push" | "pull";
export type APIAuthType = "none" | "api_key_header" | "bearer";

// Lo que devuelve el backend en Monitor.sourceConfig — sin pushToken ni
// pullAuthValue, que nunca se serializan (json:"-" en el backend).
export interface SourceConfig {
  delimiter?: string;
  hasHeaderRow?: boolean;
  sheetName?: string;
  rootPath?: string;
  mode?: APIMode;
  pullUrl?: string;
  pullMethod?: string;
  pullAuthType?: APIAuthType;
  pullAuthHeaderName?: string;
  pullIntervalMinutes?: number;
  nextPullAt?: string;
  lastPullAt?: string;
  lastPullStatus?: "ok" | "error";
  lastPullError?: string;
}

// Lo que se envía al crear un monitor — sí incluye pullAuthValue, que es
// de solo-escritura (el backend lo acepta pero nunca lo devuelve).
export interface CreateSourceConfig {
  delimiter?: string;
  hasHeaderRow?: boolean;
  sheetName?: string;
  rootPath?: string;
  mode?: APIMode;
  pullUrl?: string;
  pullMethod?: string;
  pullAuthType?: APIAuthType;
  pullAuthHeaderName?: string;
  pullAuthValue?: string;
  pullIntervalMinutes?: number;
}

export interface Monitor {
  id: string;
  name: string;
  description: string;
  sourceType: SourceType;
  sourceConfig?: SourceConfig;
  schema: SchemaField[];
  collectionId: string;
  ownerId: string;
  recordCount: number;
  lastIngested?: string;
  createdAt: string;
  updatedAt: string;
}
```

(Se eliminan `apiEndpoint?: string;` y `schedule?: string;` de `Monitor` — reemplazados por `sourceConfig`.)

- [ ] **Step 2: Actualizar `monitorsApi.create` en `monitors.ts`**

Ubica:

```ts
import type { Monitor, SchemaField } from "@/lib/types";
```

Reemplázala por:

```ts
import type { Monitor, SchemaField, SourceType, CreateSourceConfig } from "@/lib/types";
```

Ubica:

```ts
  create: (data: { name: string; description: string; sourceType: string }) =>
    api.post<Monitor>("/monitors", data),
```

Reemplázala por:

```ts
  create: (data: { name: string; description: string; sourceType: SourceType; sourceConfig?: CreateSourceConfig }) =>
    api.post<Monitor | { monitor: Monitor; pushToken: string }>("/monitors", data),
```

- [ ] **Step 3: Verificar que compila**

Run: `cd frontend && npm run build 2>&1 | tail -40`
Expected: falla — `frontend/src/app/(dashboard)/monitors/page.tsx` todavía llama `monitorsApi.create(newMonitor)` sin `sourceConfig` y trata la respuesta como `Monitor` directo, no como la unión nueva. Confirma que el ÚNICO archivo con error es `monitors/page.tsx` (se corrige en el Task 8) y no hay errores en `types.ts`/`monitors.ts`.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/types.ts frontend/src/lib/api/monitors.ts
git commit -m "feat(monitors): tipos SourceConfig/CreateSourceConfig y create() actualizado"
```

---

## Task 8: Frontend — formulario de creación condicional + modal de token

**Files:**
- Modify: `frontend/src/app/(dashboard)/monitors/page.tsx`

**Interfaces:**
- Consumes: `monitorsApi.create` (Task 7), `SourceConfig`/`CreateSourceConfig`/`APIMode`/`APIAuthType` (Task 7).

- [ ] **Step 1: Reemplazar imports y estado**

Reemplaza:

```ts
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Plus, Database, Upload, Trash2 } from "lucide-react";
import { monitorsApi } from "@/lib/api/monitors";
import { useToast } from "@/lib/use-toast";
import { formatDate } from "@/lib/utils";
import type { Monitor, SourceType } from "@/lib/types";
```

por:

```ts
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Switch } from "@/components/ui/switch";
import { Plus, Database, Upload, Trash2, Copy, Check } from "lucide-react";
import { monitorsApi } from "@/lib/api/monitors";
import { useToast } from "@/lib/use-toast";
import { formatDate } from "@/lib/utils";
import type { Monitor, SourceType, APIMode, APIAuthType } from "@/lib/types";
```

`@/components/ui/switch` no existe todavía en este repo — se crea en el Step 2. `Copy`/`Check` son íconos de `lucide-react`, ya es una dependencia instalada.

Reemplaza:

```ts
  const [newMonitor, setNewMonitor] = useState({ name: "", description: "", sourceType: "csv" as SourceType });
```

por:

```ts
  const [newMonitor, setNewMonitor] = useState({
    name: "",
    description: "",
    sourceType: "csv" as SourceType,
    delimiter: "",
    hasHeaderRow: true,
    sheetName: "",
    rootPath: "",
    apiMode: "push" as APIMode,
    pullUrl: "",
    pullMethod: "GET",
    pullAuthType: "none" as APIAuthType,
    pullAuthHeaderName: "",
    pullAuthValue: "",
    pullIntervalMinutes: "60",
  });
  const [pushTokenReveal, setPushTokenReveal] = useState<{ url: string; token: string } | null>(null);
  const [tokenCopied, setTokenCopied] = useState(false);
```

- [ ] **Step 2: Crear el componente `Switch`**

Este repo no tiene un `Switch` de shadcn todavía. Verifica primero si `@radix-ui/react-switch` está instalado:

Run: `cd frontend && grep -i "radix-ui/react-switch" package.json`

Si no aparece nada, instálalo: `npm install @radix-ui/react-switch`

Crea `frontend/src/components/ui/switch.tsx`:

```tsx
"use client";

import * as React from "react";
import * as SwitchPrimitives from "@radix-ui/react-switch";
import { cn } from "@/lib/utils";

const Switch = React.forwardRef<
  React.ComponentRef<typeof SwitchPrimitives.Root>,
  React.ComponentPropsWithoutRef<typeof SwitchPrimitives.Root>
>(({ className, ...props }, ref) => (
  <SwitchPrimitives.Root
    className={cn(
      "peer inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full border-2 border-transparent transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 data-[state=checked]:bg-primary data-[state=unchecked]:bg-input",
      className
    )}
    {...props}
    ref={ref}
  >
    <SwitchPrimitives.Thumb
      className={cn(
        "pointer-events-none block h-4 w-4 rounded-full bg-background shadow-lg ring-0 transition-transform data-[state=checked]:translate-x-4 data-[state=unchecked]:translate-x-0"
      )}
    />
  </SwitchPrimitives.Root>
));
Switch.displayName = SwitchPrimitives.Root.displayName;

export { Switch };
```

- [ ] **Step 3: Reemplazar `createMonitor`**

Reemplaza la función completa:

```ts
  async function createMonitor(e: React.FormEvent) {
    e.preventDefault();
    try {
      let sourceConfig: CreateSourceConfig | undefined;

      if (newMonitor.sourceType === "csv" || newMonitor.sourceType === "txt") {
        sourceConfig = {
          delimiter: newMonitor.delimiter || undefined,
          hasHeaderRow: newMonitor.hasHeaderRow,
        };
      } else if (newMonitor.sourceType === "excel") {
        sourceConfig = newMonitor.sheetName ? { sheetName: newMonitor.sheetName } : undefined;
      } else if (newMonitor.sourceType === "json") {
        sourceConfig = newMonitor.rootPath ? { rootPath: newMonitor.rootPath } : undefined;
      } else if (newMonitor.sourceType === "api") {
        sourceConfig =
          newMonitor.apiMode === "pull"
            ? {
                mode: "pull",
                pullUrl: newMonitor.pullUrl,
                pullMethod: newMonitor.pullMethod,
                pullAuthType: newMonitor.pullAuthType,
                pullAuthHeaderName: newMonitor.pullAuthHeaderName || undefined,
                pullAuthValue: newMonitor.pullAuthValue || undefined,
                pullIntervalMinutes: Number(newMonitor.pullIntervalMinutes) || 60,
              }
            : { mode: "push" };
      }

      const result = await monitorsApi.create({
        name: newMonitor.name,
        description: newMonitor.description,
        sourceType: newMonitor.sourceType,
        sourceConfig,
      });

      setIsCreateOpen(false);
      if ("pushToken" in result) {
        const apiUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";
        setPushTokenReveal({ url: `${apiUrl}/ingest/${result.monitor.id}`, token: result.pushToken });
        setTokenCopied(false);
      }
      setNewMonitor({
        name: "", description: "", sourceType: "csv",
        delimiter: "", hasHeaderRow: true, sheetName: "", rootPath: "",
        apiMode: "push", pullUrl: "", pullMethod: "GET", pullAuthType: "none",
        pullAuthHeaderName: "", pullAuthValue: "", pullIntervalMinutes: "60",
      });
      loadMonitors();
    } catch { toastError("Error al crear monitor"); }
  }

  async function copyPushToken() {
    if (!pushTokenReveal) return;
    await navigator.clipboard.writeText(pushTokenReveal.token);
    setTokenCopied(true);
  }
```

- [ ] **Step 4: Agregar `"txt"` al `Select` de tipo de fuente**

Ubica:

```tsx
                    <SelectContent>
                      <SelectItem value="csv">CSV</SelectItem>
                      <SelectItem value="excel">Excel</SelectItem>
                      <SelectItem value="json">JSON</SelectItem>
                      <SelectItem value="api">API</SelectItem>
                    </SelectContent>
```

Reemplázala por:

```tsx
                    <SelectContent>
                      <SelectItem value="csv">CSV</SelectItem>
                      <SelectItem value="excel">Excel</SelectItem>
                      <SelectItem value="json">JSON</SelectItem>
                      <SelectItem value="txt">TXT</SelectItem>
                      <SelectItem value="api">API</SelectItem>
                    </SelectContent>
```

- [ ] **Step 5: Agregar la sección condicional al formulario**

Justo después del bloque `<div className="space-y-2">...Tipo de fuente...</div>` y antes de `<Button type="submit" className="w-full">Crear</Button>`, agrega:

```tsx
                {(newMonitor.sourceType === "csv" || newMonitor.sourceType === "txt") && (
                  <div className="space-y-3 rounded-md border p-3">
                    <div className="space-y-2">
                      <Label>Delimitador</Label>
                      <Input
                        value={newMonitor.delimiter}
                        onChange={(e) => setNewMonitor({ ...newMonitor, delimiter: e.target.value.slice(0, 1) })}
                        placeholder={newMonitor.sourceType === "txt" ? "Tab (por defecto)" : "coma (por defecto)"}
                        maxLength={1}
                      />
                    </div>
                    <div className="flex items-center justify-between">
                      <Label>Primera fila es encabezado</Label>
                      <Switch
                        checked={newMonitor.hasHeaderRow}
                        onCheckedChange={(v) => setNewMonitor({ ...newMonitor, hasHeaderRow: v })}
                      />
                    </div>
                  </div>
                )}

                {newMonitor.sourceType === "excel" && (
                  <div className="space-y-2 rounded-md border p-3">
                    <Label>Nombre de la hoja</Label>
                    <Input
                      value={newMonitor.sheetName}
                      onChange={(e) => setNewMonitor({ ...newMonitor, sheetName: e.target.value })}
                      placeholder="Primera hoja (por defecto)"
                    />
                  </div>
                )}

                {newMonitor.sourceType === "json" && (
                  <div className="space-y-2 rounded-md border p-3">
                    <Label>Ruta raíz (opcional)</Label>
                    <Input
                      value={newMonitor.rootPath}
                      onChange={(e) => setNewMonitor({ ...newMonitor, rootPath: e.target.value })}
                      placeholder="ej. data.records — vacío usa la raíz del JSON"
                    />
                  </div>
                )}

                {newMonitor.sourceType === "api" && (
                  <div className="space-y-3 rounded-md border p-3">
                    <div className="space-y-2">
                      <Label>Modo</Label>
                      <Select
                        value={newMonitor.apiMode}
                        onValueChange={(v) => setNewMonitor({ ...newMonitor, apiMode: v as APIMode })}
                      >
                        <SelectTrigger><SelectValue /></SelectTrigger>
                        <SelectContent>
                          <SelectItem value="push">Push — el sistema externo nos envía datos</SelectItem>
                          <SelectItem value="pull">Pull — consultamos una API externa por horario</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>

                    {newMonitor.apiMode === "push" ? (
                      <p className="text-xs text-muted-foreground">
                        Al crear el monitor se genera una URL y un token — se muestran una sola vez.
                      </p>
                    ) : (
                      <>
                        <div className="space-y-2">
                          <Label>URL a consultar</Label>
                          <Input
                            value={newMonitor.pullUrl}
                            onChange={(e) => setNewMonitor({ ...newMonitor, pullUrl: e.target.value })}
                            placeholder="https://api.ejemplo.com/transacciones"
                            required
                          />
                        </div>
                        <div className="grid grid-cols-2 gap-3">
                          <div className="space-y-2">
                            <Label>Método</Label>
                            <Select
                              value={newMonitor.pullMethod}
                              onValueChange={(v) => setNewMonitor({ ...newMonitor, pullMethod: v })}
                            >
                              <SelectTrigger><SelectValue /></SelectTrigger>
                              <SelectContent>
                                <SelectItem value="GET">GET</SelectItem>
                                <SelectItem value="POST">POST</SelectItem>
                              </SelectContent>
                            </Select>
                          </div>
                          <div className="space-y-2">
                            <Label>Cada (minutos)</Label>
                            <Input
                              type="number"
                              min={1}
                              value={newMonitor.pullIntervalMinutes}
                              onChange={(e) => setNewMonitor({ ...newMonitor, pullIntervalMinutes: e.target.value })}
                            />
                          </div>
                        </div>
                        <div className="space-y-2">
                          <Label>Autenticación</Label>
                          <Select
                            value={newMonitor.pullAuthType}
                            onValueChange={(v) => setNewMonitor({ ...newMonitor, pullAuthType: v as APIAuthType })}
                          >
                            <SelectTrigger><SelectValue /></SelectTrigger>
                            <SelectContent>
                              <SelectItem value="none">Ninguna</SelectItem>
                              <SelectItem value="api_key_header">API key (header)</SelectItem>
                              <SelectItem value="bearer">Bearer token</SelectItem>
                            </SelectContent>
                          </Select>
                        </div>
                        {newMonitor.pullAuthType === "api_key_header" && (
                          <div className="space-y-2">
                            <Label>Nombre del header</Label>
                            <Input
                              value={newMonitor.pullAuthHeaderName}
                              onChange={(e) => setNewMonitor({ ...newMonitor, pullAuthHeaderName: e.target.value })}
                              placeholder="ej. X-API-Key"
                            />
                          </div>
                        )}
                        {newMonitor.pullAuthType !== "none" && (
                          <div className="space-y-2">
                            <Label>{newMonitor.pullAuthType === "bearer" ? "Token" : "Valor de la API key"}</Label>
                            <Input
                              type="password"
                              value={newMonitor.pullAuthValue}
                              onChange={(e) => setNewMonitor({ ...newMonitor, pullAuthValue: e.target.value })}
                            />
                          </div>
                        )}
                      </>
                    )}
                  </div>
                )}
```

- [ ] **Step 6: Agregar el modal de revelación del push token**

Justo después del `</Dialog>` de creación (antes del cierre de `<div className="mb-6 flex items-center justify-between">`), agrega un segundo `Dialog`:

```tsx
          <Dialog open={!!pushTokenReveal} onOpenChange={(open) => !open && setPushTokenReveal(null)}>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Credenciales de ingesta</DialogTitle>
              </DialogHeader>
              <div className="space-y-4">
                <p className="text-sm text-muted-foreground">
                  Guarda esta URL y este token ahora — no se van a volver a mostrar.
                </p>
                <div className="space-y-2">
                  <Label>URL</Label>
                  <Input readOnly value={pushTokenReveal?.url ?? ""} className="font-mono text-xs" />
                </div>
                <div className="space-y-2">
                  <Label>Header: X-Ingest-Token</Label>
                  <Input readOnly value={pushTokenReveal?.token ?? ""} className="font-mono text-xs" />
                </div>
                <Button onClick={copyPushToken} className="w-full" variant="outline">
                  {tokenCopied ? <Check className="mr-2 h-4 w-4" /> : <Copy className="mr-2 h-4 w-4" />}
                  {tokenCopied ? "Token copiado" : "Copiar token"}
                </Button>
              </div>
            </DialogContent>
          </Dialog>
```

- [ ] **Step 7: Verificar que compila**

Run: `cd frontend && npm run build 2>&1 | tail -60`
Expected: compila sin errores de tipos.

- [ ] **Step 8: Verificar en el navegador**

Levanta `npm run dev` (backend también corriendo), entra a `/monitors`, abre "Nuevo monitor":
1. Cambia el tipo de fuente entre `csv`/`excel`/`json`/`txt`/`api` y confirma que los campos condicionales aparecen/desaparecen correctamente.
2. Con `api` + `push`, crea el monitor y confirma que aparece el modal con URL + token, y que "Copiar token" funciona.
3. Con `api` + `pull`, llena URL/método/intervalo y crea — confirma que no hay errores en consola.

- [ ] **Step 9: Commit**

```bash
git add frontend/src/app/\(dashboard\)/monitors/page.tsx frontend/src/components/ui/switch.tsx frontend/package.json frontend/package-lock.json
git commit -m "feat(monitors): formulario de creación con parámetros por tipo de fuente"
```

---

## Task 9: Frontend — detalle de monitor (TXT + estado de conexión API)

**Files:**
- Modify: `frontend/src/app/(dashboard)/monitors/[id]/page.tsx`

**Interfaces:**
- Consumes: `Monitor.sourceConfig` (Task 7).

- [ ] **Step 1: Agregar `.txt` al dropzone**

Ubica:

```ts
    accept: {
      "text/csv": [".csv"],
      "application/json": [".json"],
      "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": [".xlsx"],
      "application/vnd.ms-excel": [".xls"],
    },
```

Reemplázala por:

```ts
    accept: {
      "text/csv": [".csv"],
      "application/json": [".json"],
      "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": [".xlsx"],
      "application/vnd.ms-excel": [".xls"],
      "text/plain": [".txt"],
    },
```

No toques el párrafo de ayuda ("CSV, Excel (.xlsx), JSON") todavía — el Step 2 reemplaza ese bloque completo, ya con el texto correcto incluido; editarlo aquí sería trabajo que el Step 2 pisa de inmediato.

- [ ] **Step 2: Reemplazar el contenido del tab "Cargar datos" para monitores `api`**

Un monitor `api` no tiene archivo que arrastrar — el dropzone actual (`getRootProps()`) queda sin sentido para ese caso y el usuario terminaría con el mismo error "unsupported source type" que motivó este cambio si el backend no lo maneja (ya no debería pasar, pero la UI también debe dejar de ofrecer un flujo que no aplica).

Ubica el `<TabsContent value="upload">` completo (contiene el `<div {...getRootProps()}>`). Envuélvelo con una condición: si `monitor.sourceType === "api"`, muestra el estado de conexión en vez del dropzone. Reemplaza el contenido de `<TabsContent value="upload">` por:

```tsx
          <TabsContent value="upload">
            <Card>
              <CardContent className="pt-6">
                {monitor.sourceType === "api" ? (
                  monitor.sourceConfig?.mode === "pull" ? (
                    <div className="space-y-3">
                      <div className="flex items-center justify-between rounded-md border p-3">
                        <span className="text-sm text-muted-foreground">URL</span>
                        <span className="max-w-[60%] truncate text-sm font-mono">{monitor.sourceConfig?.pullUrl}</span>
                      </div>
                      <div className="flex items-center justify-between rounded-md border p-3">
                        <span className="text-sm text-muted-foreground">Cada</span>
                        <span className="text-sm">{monitor.sourceConfig?.pullIntervalMinutes ?? 60} minutos</span>
                      </div>
                      <div className="flex items-center justify-between rounded-md border p-3">
                        <span className="text-sm text-muted-foreground">Último intento</span>
                        <span className="text-sm">
                          {monitor.sourceConfig?.lastPullAt ? formatDate(monitor.sourceConfig.lastPullAt) : "Aún no se ha ejecutado"}
                        </span>
                      </div>
                      {monitor.sourceConfig?.lastPullStatus === "error" && (
                        <div className="rounded-md bg-danger-bg p-3 text-sm text-danger-fg">
                          {monitor.sourceConfig.lastPullError}
                        </div>
                      )}
                      {monitor.sourceConfig?.lastPullStatus === "ok" && (
                        <div className="rounded-md bg-success-bg p-3 text-sm text-success-fg">
                          Última consulta exitosa
                        </div>
                      )}
                    </div>
                  ) : (
                    <p className="text-sm text-muted-foreground">
                      Este monitor recibe datos por push. La URL y el token se mostraron una sola vez al crearlo —
                      si los perdiste, elimina este monitor y crea uno nuevo (no se pueden volver a mostrar).
                    </p>
                  )
                ) : (
                  <>
                    <div
                      {...getRootProps()}
                      className={`flex cursor-pointer flex-col items-center justify-center rounded-lg border-2 border-dashed p-12 transition-colors ${
                        isDragActive ? "border-primary bg-primary/5" : "border-muted-foreground/25 hover:border-primary/50"
                      }`}
                    >
                      <input {...getInputProps()} />
                      <Upload className="mb-4 h-10 w-10 text-muted-foreground" />
                      {uploading ? (
                        <p className="text-sm text-muted-foreground">Procesando archivo...</p>
                      ) : isDragActive ? (
                        <p className="text-sm">Suelta el archivo aqui</p>
                      ) : (
                        <>
                          <p className="text-sm font-medium">Arrastra un archivo o haz clic para seleccionar</p>
                          <p className="mt-1 text-xs text-muted-foreground">CSV, Excel (.xlsx), JSON, TXT</p>
                        </>
                      )}
                    </div>

                    {uploadResult && (
                      <div className="mt-4 rounded-md bg-success-bg p-4">
                        <p className="text-sm font-medium text-success-fg">
                          {uploadResult.recordsIngested} registros cargados
                          {uploadResult.evaluationQueued && " — reglas en evaluacion"}
                        </p>
                      </div>
                    )}
                  </>
                )}
              </CardContent>
            </Card>
          </TabsContent>
```

- [ ] **Step 3: Verificar que compila**

Run: `cd frontend && npm run build 2>&1 | tail -40`
Expected: compila sin errores.

- [ ] **Step 4: Verificar en el navegador**

1. Crea un monitor `csv`, sube un `.txt` delimitado por tabs — confirma que el dropzone lo acepta y que los datos se cargan (usa el flujo de creación con `sourceType: "txt"` en su lugar y confirma el mismo resultado).
2. Abre el detalle de un monitor `api`+`pull` creado antes — confirma que se ve la URL, intervalo, y último estado en vez del dropzone.
3. Abre el detalle de un monitor `api`+`push` — confirma el mensaje de que la URL/token no se puede volver a mostrar.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/app/\(dashboard\)/monitors/\[id\]/page.tsx
git commit -m "feat(monitors): detalle de monitor refleja TXT y estado de conexión API"
```

---

## Self-Review (completado durante la redacción de este plan)

- **Cobertura del spec:** cada sección del spec tiene tarea — modelo (Task 1), TXT (Task 2), Excel/JSON/refactor io.Reader (Task 3), creación+token (Task 4), push (Task 5), pull (Task 6), frontend tipos (Task 7), formulario (Task 8), detalle (Task 9).
- **Correcciones encontradas al escribir el plan (no estaban en el spec):**
  - `PullAuthValue` con `json:"-"` en `SourceConfig` habría bloqueado también la ENTRADA del dato, no solo la salida — el spec no lo mencionaba. Se resolvió con `SourceConfigInput`, un struct de request separado sin esa restricción (Task 1).
  - `IngestJSON` necesitaba cambiar su firma de `multipart.File` a `io.Reader` para ser reutilizable por push (body HTTP) y pull (respuesta HTTP) — el spec lo daba por hecho pero no lo especificaba; confirmado que es un cambio compatible porque `multipart.File` ya satisface `io.Reader` (Task 3).
- **Consistencia de tipos:** `IngestPush`/`MonitorPuller` ambos llaman `IngestionService.IngestJSON(ctx, monitor, io.Reader)` con la misma firma definida en Task 3. `SourceConfigInput.ToSourceConfig()` se usa igual en Task 4. Nombres de campos bson (`source_config.last_pull_at`, etc.) coinciden entre el struct de Task 1 y las actualizaciones de Task 6.
