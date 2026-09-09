# Fase 1 — Timestamp Derivado y Condición de Velocidad — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Hacer evaluable el caso "transacciones > USD 5.000 de casino con menos de 35 segundos entre una y otra en la misma tarjeta", construyendo un timestamp real a partir de los dos campos numéricos separados del origen (`fechapoliza` + `horapoliza`) y agregando un tipo de condición nuevo que mide la distancia temporal entre eventos consecutivos de una misma entidad.

**Architecture:** Dos piezas independientes que se encuentran al final. (1) `DerivedTimestampConfig` en el monitor: una función pura combina fecha y hora numéricas en un `time.Time`, la ingesta lo materializa en cada documento, un endpoint de backfill lo calcula para los datos ya guardados, y el campo se expone en el esquema como tipo `date`. (2) `VelocityCondition` en la regla: se evalúa con un pipeline de `$setWindowFields` + `$shift` + `$dateDiff` (validado contra el Mongo 7.0.40 de producción antes de escribir el spec), emitiendo una bandera roja por cada par de transacciones consecutivas demasiado próximas.

**Tech Stack:** Go/Fiber/MongoDB 7 — sin dependencias nuevas.

**Spec:** `docs/superpowers/specs/2026-09-09-reglas-de-velocidad-y-timestamp-derivado-design.md`

**Alcance:** Este plan cubre **solo la Fase 1** del spec (backend: modelo, ingesta, backfill, motor). La Fase 2 (UI) y la Fase 3 (IA) tienen sus propios planes, que se escriben una vez que esta fase esté mergeada — así se apoyan en los nombres y firmas reales que produzca, no en los previstos. Al terminar esta fase el caso ya es expresable y evaluable vía API.

## Global Constraints

- **Todo aditivo.** Un monitor sin `DerivedTimestamp` y una regla sin `VelocityConditions` se comportan exactamente como hoy. Ningún test ni comportamiento existente puede cambiar.
- **El relleno de ceros es obligatorio.** `horapoliza` se guarda como número: `021517` llega como `21517` y hay que rellenar a la izquierda hasta el ancho del formato antes de parsear. Sin eso `21517` se leería como `21:51:07` en vez de `02:15:17` — 19 horas de error silencioso. Vale igual para el campo de fecha.
- **Un timestamp inconstruible no invalida la fila.** Si falta un campo o no parsea, el documento se ingiere sin el campo derivado; no se rechaza la fila.
- **Configuración inválida se rechaza al guardar, nunca en silencio al evaluar.** Este es exactamente el modo de falla que dejó la ventana temporal rota en producción sin que nadie se enterara.
- **La rama nueva del motor va con tests activados.** El bug de la ventana temporal sobrevivió porque `baseAggCondition()` nunca seteaba `TimeField`/`TimeWindow` y ningún test ejercitaba esa rama.
- Sin tests de UI/componentes — esta fase no toca frontend.

---

### Task 1: `DerivedTimestampConfig` y la función pura de derivación

**Files:**
- Modify: `backend/internal/models/monitor.go`
- Create: `backend/internal/services/derived_timestamp.go`
- Test: `backend/internal/services/derived_timestamp_test.go`

**Interfaces:**
- Produces: `models.DerivedTimestampConfig` y `models.Monitor.DerivedTimestamp *DerivedTimestampConfig`; `services.BuildDerivedTimestamp(cfg models.DerivedTimestampConfig, doc bson.M) (time.Time, bool)` — devuelve `ok=false` cuando el timestamp no se puede construir. Tareas 2, 3 y 4 la consumen.

- [ ] **Step 1: Agregar el modelo**

En `backend/internal/models/monitor.go`, junto a `SchemaField`:

```go
// DerivedTimestampConfig construye un campo de fecha real a partir de dos
// columnas numéricas separadas — patrón habitual en exports bancarios, donde
// fecha y hora vienen en campos distintos. Nil en el monitor significa que no
// hay timestamp derivado y todo se comporta como siempre.
type DerivedTimestampConfig struct {
	DateField  string `bson:"date_field" json:"dateField"`
	DateFormat string `bson:"date_format" json:"dateFormat"` // "YYYYMMDD"
	TimeField  string `bson:"time_field" json:"timeField"`
	TimeFormat string `bson:"time_format" json:"timeFormat"` // "HHMMSS" | "HHMM"
	TargetName string `bson:"target_name" json:"targetName"`
}
```

Y en el struct `Monitor`, después de `Schema`:

```go
	DerivedTimestamp *DerivedTimestampConfig `bson:"derived_timestamp,omitempty" json:"derivedTimestamp,omitempty"`
```

- [ ] **Step 2: Escribir los tests (deben fallar — la función no existe)**

Crear `backend/internal/services/derived_timestamp_test.go`:

```go
package services

import (
	"testing"
	"time"

	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson"
)

func baseTimestampConfig() models.DerivedTimestampConfig {
	return models.DerivedTimestampConfig{
		DateField:  "fechapoliza",
		DateFormat: "YYYYMMDD",
		TimeField:  "horapoliza",
		TimeFormat: "HHMMSS",
		TargetName: "timestamp",
	}
}

// El caso que motiva todo: horapoliza se guarda como número, así que
// "021517" llega como 21517. Sin relleno de ceros se leería 21:51:07 —
// 19 horas de diferencia sobre el valor real, en silencio.
func TestBuildDerivedTimestamp_RellenaCerosALaIzquierda(t *testing.T) {
	doc := bson.M{"fechapoliza": int32(20260907), "horapoliza": int32(21517)}
	got, ok := BuildDerivedTimestamp(baseTimestampConfig(), doc)
	if !ok {
		t.Fatal("esperaba poder construir el timestamp")
	}
	want := time.Date(2026, 9, 7, 2, 15, 17, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestBuildDerivedTimestamp_MedianocheEsCero(t *testing.T) {
	doc := bson.M{"fechapoliza": int32(20260907), "horapoliza": int32(0)}
	got, ok := BuildDerivedTimestamp(baseTimestampConfig(), doc)
	if !ok {
		t.Fatal("esperaba poder construir el timestamp")
	}
	want := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v (medianoche)", got, want)
	}
}

func TestBuildDerivedTimestamp_FormatoHHMM(t *testing.T) {
	cfg := baseTimestampConfig()
	cfg.TimeFormat = "HHMM"
	doc := bson.M{"fechapoliza": int32(20260907), "horapoliza": int32(215)}
	got, ok := BuildDerivedTimestamp(cfg, doc)
	if !ok {
		t.Fatal("esperaba poder construir el timestamp")
	}
	want := time.Date(2026, 9, 7, 2, 15, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestBuildDerivedTimestamp_ValoresFloatYString(t *testing.T) {
	// La ingesta puede dejar números como float64 (JSON) o el valor crudo
	// como string si el campo no se tipó como number.
	casos := []bson.M{
		{"fechapoliza": float64(20260907), "horapoliza": float64(21517)},
		{"fechapoliza": "20260907", "horapoliza": "021517"},
	}
	want := time.Date(2026, 9, 7, 2, 15, 17, 0, time.UTC)
	for i, doc := range casos {
		got, ok := BuildDerivedTimestamp(baseTimestampConfig(), doc)
		if !ok {
			t.Fatalf("caso %d: esperaba poder construir el timestamp", i)
		}
		if !got.Equal(want) {
			t.Errorf("caso %d: got %v, want %v", i, got, want)
		}
	}
}

func TestBuildDerivedTimestamp_CampoFaltanteNoConstruye(t *testing.T) {
	doc := bson.M{"fechapoliza": int32(20260907)}
	if _, ok := BuildDerivedTimestamp(baseTimestampConfig(), doc); ok {
		t.Error("sin el campo de hora no debería construir un timestamp")
	}
}

func TestBuildDerivedTimestamp_ValorNoParseableNoConstruye(t *testing.T) {
	doc := bson.M{"fechapoliza": "no-es-fecha", "horapoliza": int32(21517)}
	if _, ok := BuildDerivedTimestamp(baseTimestampConfig(), doc); ok {
		t.Error("un valor no parseable no debería construir un timestamp")
	}
}

func TestBuildDerivedTimestamp_FechaInvalidaNoConstruye(t *testing.T) {
	doc := bson.M{"fechapoliza": int32(20261345), "horapoliza": int32(21517)}
	if _, ok := BuildDerivedTimestamp(baseTimestampConfig(), doc); ok {
		t.Error("mes 13 / día 45 no debería construir un timestamp")
	}
}
```

- [ ] **Step 3: Correr los tests, confirmar que fallan**

Run: `cd backend && go test ./internal/services/... -run TestBuildDerivedTimestamp -v`
Expected: FAIL — `undefined: BuildDerivedTimestamp` (no compila).

- [ ] **Step 4: Implementar la función**

Crear `backend/internal/services/derived_timestamp.go`:

```go
package services

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson"
)

// timestampLayouts mapea los formatos que expone la configuración al layout
// de Go equivalente, junto al ancho al que hay que rellenar con ceros.
var timestampLayouts = map[string]struct {
	layout string
	width  int
}{
	"YYYYMMDD": {"20060102", 8},
	"HHMMSS":   {"150405", 6},
	"HHMM":     {"1504", 4},
}

// BuildDerivedTimestamp combina el campo de fecha y el de hora del documento
// en un time.Time UTC. Devuelve ok=false si algún campo falta, no parsea, o
// la fecha resultante no existe — el llamador ingiere el documento igual,
// sin el campo derivado: un timestamp inconstruible es un dato incompleto,
// no un dato inválido.
func BuildDerivedTimestamp(cfg models.DerivedTimestampConfig, doc bson.M) (time.Time, bool) {
	dateSpec, ok := timestampLayouts[cfg.DateFormat]
	if !ok {
		return time.Time{}, false
	}
	timeSpec, ok := timestampLayouts[cfg.TimeFormat]
	if !ok {
		return time.Time{}, false
	}

	datePart, ok := padNumericField(doc[cfg.DateField], dateSpec.width)
	if !ok {
		return time.Time{}, false
	}
	timePart, ok := padNumericField(doc[cfg.TimeField], timeSpec.width)
	if !ok {
		return time.Time{}, false
	}

	parsed, err := time.ParseInLocation(dateSpec.layout+timeSpec.layout, datePart+timePart, time.UTC)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

// padNumericField normaliza el valor a un string de exactamente width
// dígitos. El relleno de ceros es la razón de ser de esta función: la hora
// se guarda como número, así que "021517" vuelve como 21517 y sin rellenar
// se interpretaría como 21:51:07 en vez de 02:15:17.
func padNumericField(raw interface{}, width int) (string, bool) {
	var s string
	switch v := raw.(type) {
	case nil:
		return "", false
	case string:
		s = strings.TrimSpace(v)
	case int32:
		s = strconv.FormatInt(int64(v), 10)
	case int64:
		s = strconv.FormatInt(v, 10)
	case int:
		s = strconv.Itoa(v)
	case float64:
		s = strconv.FormatInt(int64(v), 10)
	default:
		return "", false
	}
	if s == "" {
		return "", false
	}
	if _, err := strconv.ParseInt(s, 10, 64); err != nil {
		return "", false
	}
	if len(s) > width {
		return "", false
	}
	return fmt.Sprintf("%0*s", width, s), true
}
```

- [ ] **Step 5: Correr los tests, confirmar que pasan**

Run: `cd backend && go test ./internal/services/... -run TestBuildDerivedTimestamp -v`
Expected: PASS (7 tests).

- [ ] **Step 6: Commit**

```bash
git add backend/internal/models/monitor.go backend/internal/services/derived_timestamp.go backend/internal/services/derived_timestamp_test.go
git commit -m "feat(timestamp): configuración y derivación de timestamp desde fecha+hora numéricas"
```

---

### Task 2: Materializar el timestamp en la ingesta

**Files:**
- Modify: `backend/internal/services/ingestion_service.go`
- Test: `backend/internal/services/ingestion_service_test.go`

**Interfaces:**
- Consumes: `services.BuildDerivedTimestamp`, `models.Monitor.DerivedTimestamp` (Task 1).
- Produces: los documentos ingeridos llevan el campo derivado como fecha BSON — Task 4 (backfill) usa la misma función para los ya guardados, y Task 6 (velocidad) lo consume como `TimeField`.

- [ ] **Step 1: Escribir el test (debe fallar)**

Agregar en `backend/internal/services/ingestion_service_test.go`:

```go
// El documento ingerido debe llevar el campo derivado como time.Time cuando
// el monitor lo tiene configurado, y no llevarlo cuando no.
func TestApplyDerivedTimestamp_AgregaCampoCuandoHayConfig(t *testing.T) {
	cfg := &models.DerivedTimestampConfig{
		DateField: "fechapoliza", DateFormat: "YYYYMMDD",
		TimeField: "horapoliza", TimeFormat: "HHMMSS",
		TargetName: "timestamp",
	}
	doc := bson.M{"fechapoliza": int32(20260907), "horapoliza": int32(21517)}

	applyDerivedTimestamp(doc, cfg)

	got, ok := doc["timestamp"].(time.Time)
	if !ok {
		t.Fatalf("esperaba un time.Time en 'timestamp', got %T", doc["timestamp"])
	}
	if want := time.Date(2026, 9, 7, 2, 15, 17, 0, time.UTC); !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestApplyDerivedTimestamp_SinConfigNoTocaElDocumento(t *testing.T) {
	doc := bson.M{"fechapoliza": int32(20260907), "horapoliza": int32(21517)}
	applyDerivedTimestamp(doc, nil)
	if _, existe := doc["timestamp"]; existe {
		t.Error("sin configuración no debería agregarse ningún campo derivado")
	}
	if len(doc) != 2 {
		t.Errorf("el documento no debería cambiar, got %d campos", len(doc))
	}
}

// Una fila con hora ilegible se ingiere igual, sin el campo derivado: un
// timestamp inconstruible es dato incompleto, no dato inválido.
func TestApplyDerivedTimestamp_InconstruibleNoAgregaCampo(t *testing.T) {
	cfg := &models.DerivedTimestampConfig{
		DateField: "fechapoliza", DateFormat: "YYYYMMDD",
		TimeField: "horapoliza", TimeFormat: "HHMMSS",
		TargetName: "timestamp",
	}
	doc := bson.M{"fechapoliza": int32(20260907)}
	applyDerivedTimestamp(doc, cfg)
	if _, existe := doc["timestamp"]; existe {
		t.Error("no debería agregarse el campo si no se puede construir")
	}
}
```

Confirmar que el bloque `import` del archivo incluye `"time"` y `"go.mongodb.org/mongo-driver/bson"` (`time` se agregó en el trabajo de formato de fecha; `bson` puede faltar — agregarlo si es así).

- [ ] **Step 2: Correr el test, confirmar que falla**

Run: `cd backend && go test ./internal/services/... -run TestApplyDerivedTimestamp -v`
Expected: FAIL — `undefined: applyDerivedTimestamp`.

- [ ] **Step 3: Implementar el helper y llamarlo desde los tres caminos de ingesta**

En `backend/internal/services/ingestion_service.go`, agregar el helper junto a `parseValue`:

```go
// applyDerivedTimestamp escribe el campo de fecha derivado en el documento
// cuando el monitor lo tiene configurado. Sin configuración, o cuando el
// timestamp no se puede construir, el documento queda intacto.
func applyDerivedTimestamp(doc bson.M, cfg *models.DerivedTimestampConfig) {
	if cfg == nil || cfg.TargetName == "" {
		return
	}
	if ts, ok := BuildDerivedTimestamp(*cfg, doc); ok {
		doc[cfg.TargetName] = ts
	}
}
```

Y llamarlo en los tres bucles que arman documentos, inmediatamente después de completar los campos del documento y antes de agregarlo a `documents`:

1. `ingestDelimited` — después del `for j, name := range fieldNames { ... doc[name] = parseValue(...) }`:
```go
		applyDerivedTimestamp(doc, monitor.DerivedTimestamp)
		documents = append(documents, doc)
```
2. `IngestExcel` — mismo patrón, mismo lugar.
3. `IngestJSON` — dentro del `for _, record := range records`, después de `record["_ingested_at"] = time.Now()`:
```go
		applyDerivedTimestamp(record, monitor.DerivedTimestamp)
		documents = append(documents, record)
```

- [ ] **Step 4: Correr los tests, confirmar que pasan**

Run: `cd backend && go test ./internal/services/... -v`
Expected: PASS — los 3 nuevos y todos los existentes de ingesta sin cambios (regresión: un monitor sin `DerivedTimestamp` ingiere exactamente igual que antes).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/ingestion_service.go backend/internal/services/ingestion_service_test.go
git commit -m "feat(timestamp): materializar el timestamp derivado en la ingesta"
```

---

### Task 3: Configurar el timestamp derivado por API y exponerlo en el esquema

**Files:**
- Modify: `backend/internal/handlers/monitor_handler.go`
- Modify: `backend/internal/router/router.go`
- Create: `backend/internal/services/derived_timestamp_validate.go`
- Test: `backend/internal/services/derived_timestamp_validate_test.go`

**Interfaces:**
- Consumes: `models.DerivedTimestampConfig` (Task 1).
- Produces: `PUT /monitors/:id/derived-timestamp`; `services.ValidateDerivedTimestamp(cfg models.DerivedTimestampConfig, schema []models.SchemaField) error`. Task 4 (backfill) asume que un monitor con configuración guardada ya pasó esta validación.

- [ ] **Step 1: Escribir los tests de validación (deben fallar)**

Crear `backend/internal/services/derived_timestamp_validate_test.go`:

```go
package services

import (
	"testing"

	"github.com/thureos/compliance/internal/models"
)

func schemaConFechaYHora() []models.SchemaField {
	return []models.SchemaField{
		{Name: "fechapoliza", Type: models.FieldNumber},
		{Name: "horapoliza", Type: models.FieldNumber},
		{Name: "importe", Type: models.FieldNumber},
	}
}

func TestValidateDerivedTimestamp_ConfigValida(t *testing.T) {
	cfg := models.DerivedTimestampConfig{
		DateField: "fechapoliza", DateFormat: "YYYYMMDD",
		TimeField: "horapoliza", TimeFormat: "HHMMSS",
		TargetName: "timestamp",
	}
	if err := ValidateDerivedTimestamp(cfg, schemaConFechaYHora()); err != nil {
		t.Errorf("esperaba configuración válida, got %v", err)
	}
}

func TestValidateDerivedTimestamp_CampoInexistente(t *testing.T) {
	cfg := models.DerivedTimestampConfig{
		DateField: "no_existe", DateFormat: "YYYYMMDD",
		TimeField: "horapoliza", TimeFormat: "HHMMSS",
		TargetName: "timestamp",
	}
	if err := ValidateDerivedTimestamp(cfg, schemaConFechaYHora()); err == nil {
		t.Error("un campo fuera del esquema debería rechazarse")
	}
}

func TestValidateDerivedTimestamp_FormatoNoSoportado(t *testing.T) {
	cfg := models.DerivedTimestampConfig{
		DateField: "fechapoliza", DateFormat: "DD/MM/YYYY",
		TimeField: "horapoliza", TimeFormat: "HHMMSS",
		TargetName: "timestamp",
	}
	if err := ValidateDerivedTimestamp(cfg, schemaConFechaYHora()); err == nil {
		t.Error("un formato fuera de los soportados debería rechazarse")
	}
}

// El campo derivado no puede pisar una columna real del archivo.
func TestValidateDerivedTimestamp_TargetColisionaConCampoExistente(t *testing.T) {
	cfg := models.DerivedTimestampConfig{
		DateField: "fechapoliza", DateFormat: "YYYYMMDD",
		TimeField: "horapoliza", TimeFormat: "HHMMSS",
		TargetName: "importe",
	}
	if err := ValidateDerivedTimestamp(cfg, schemaConFechaYHora()); err == nil {
		t.Error("un TargetName que pisa un campo existente debería rechazarse")
	}
}

func TestValidateDerivedTimestamp_TargetVacio(t *testing.T) {
	cfg := models.DerivedTimestampConfig{
		DateField: "fechapoliza", DateFormat: "YYYYMMDD",
		TimeField: "horapoliza", TimeFormat: "HHMMSS",
		TargetName: "",
	}
	if err := ValidateDerivedTimestamp(cfg, schemaConFechaYHora()); err == nil {
		t.Error("un TargetName vacío debería rechazarse")
	}
}
```

- [ ] **Step 2: Correr los tests, confirmar que fallan**

Run: `cd backend && go test ./internal/services/... -run TestValidateDerivedTimestamp -v`
Expected: FAIL — `undefined: ValidateDerivedTimestamp`.

- [ ] **Step 3: Implementar la validación**

Crear `backend/internal/services/derived_timestamp_validate.go`:

```go
package services

import (
	"fmt"

	"github.com/thureos/compliance/internal/models"
)

// ValidateDerivedTimestamp rechaza configuraciones que no podrían funcionar,
// en el momento de guardarlas. Una configuración inválida que se acepta y
// falla en silencio al ingerir es exactamente el modo de falla que dejó la
// ventana temporal rota en producción sin que nadie se enterara.
func ValidateDerivedTimestamp(cfg models.DerivedTimestampConfig, schema []models.SchemaField) error {
	names := make(map[string]bool, len(schema))
	for _, f := range schema {
		names[f.Name] = true
	}

	if !names[cfg.DateField] {
		return fmt.Errorf("el campo de fecha %q no existe en el esquema del monitor", cfg.DateField)
	}
	if !names[cfg.TimeField] {
		return fmt.Errorf("el campo de hora %q no existe en el esquema del monitor", cfg.TimeField)
	}
	if _, ok := timestampLayouts[cfg.DateFormat]; !ok {
		return fmt.Errorf("formato de fecha no soportado: %q", cfg.DateFormat)
	}
	if _, ok := timestampLayouts[cfg.TimeFormat]; !ok {
		return fmt.Errorf("formato de hora no soportado: %q", cfg.TimeFormat)
	}
	if cfg.TargetName == "" {
		return fmt.Errorf("el nombre del campo derivado no puede estar vacío")
	}
	if names[cfg.TargetName] {
		return fmt.Errorf("el nombre %q ya es una columna del monitor", cfg.TargetName)
	}
	return nil
}
```

- [ ] **Step 4: Correr los tests, confirmar que pasan**

Run: `cd backend && go test ./internal/services/... -run TestValidateDerivedTimestamp -v`
Expected: PASS (5 tests).

- [ ] **Step 5: Agregar el handler**

En `backend/internal/handlers/monitor_handler.go`, junto a `UpdateSchema`:

```go
// UpdateDerivedTimestamp configura (o limpia, mandando null) el timestamp
// derivado del monitor. Al guardarlo, el campo derivado se agrega al schema
// como tipo date para que aparezca en los selectores de campo de reglas,
// chat y dashboards sin que ninguno tenga que saber que es derivado.
func (h *MonitorHandler) UpdateDerivedTimestamp(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}

	monitor, err := h.monitorRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
	}

	var body struct {
		DerivedTimestamp *models.DerivedTimestampConfig `json:"derivedTimestamp"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	update := bson.M{}
	if body.DerivedTimestamp == nil {
		// Limpiar la configuración: se quita también el campo del schema.
		schema := make([]models.SchemaField, 0, len(monitor.Schema))
		for _, f := range monitor.Schema {
			if monitor.DerivedTimestamp != nil && f.Name == monitor.DerivedTimestamp.TargetName {
				continue
			}
			schema = append(schema, f)
		}
		update["derived_timestamp"] = nil
		update["schema"] = schema
	} else {
		// El schema contra el que se valida excluye el campo derivado
		// anterior: de lo contrario reconfigurar con el mismo TargetName
		// chocaría consigo mismo.
		base := make([]models.SchemaField, 0, len(monitor.Schema))
		for _, f := range monitor.Schema {
			if monitor.DerivedTimestamp != nil && f.Name == monitor.DerivedTimestamp.TargetName {
				continue
			}
			base = append(base, f)
		}
		if err := services.ValidateDerivedTimestamp(*body.DerivedTimestamp, base); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		update["derived_timestamp"] = body.DerivedTimestamp
		update["schema"] = append(base, models.SchemaField{
			Name:     body.DerivedTimestamp.TargetName,
			Type:     models.FieldDate,
			Required: false,
			Sample:   "",
		})
	}

	if err := h.monitorRepo.Update(c.Context(), monitor.ID, update); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	updated, err := h.monitorRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(updated)
}
```

- [ ] **Step 6: Registrar la ruta**

En `backend/internal/router/router.go`, junto a la ruta de `PUT /monitors/:id/schema`:

```go
	monitors.Put("/:id/derived-timestamp", middleware.RequireComplianceOrAbove(), h.Monitor.UpdateDerivedTimestamp)
```

- [ ] **Step 7: Verificar que compila y el gate pasa**

Run: `cd backend && go build ./... && go vet ./... && gofmt -l . && go test ./...`
Expected: sin errores, sin archivos sin formatear, todos los tests en verde.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/handlers/monitor_handler.go backend/internal/router/router.go backend/internal/services/derived_timestamp_validate.go backend/internal/services/derived_timestamp_validate_test.go
git commit -m "feat(timestamp): endpoint para configurar el timestamp derivado y exponerlo en el schema"
```

---

### Task 4: Backfill del timestamp para los datos ya guardados

**Files:**
- Modify: `backend/internal/repository/monitor_repo.go`
- Modify: `backend/internal/handlers/monitor_handler.go`
- Modify: `backend/internal/router/router.go`

**Interfaces:**
- Consumes: `services.BuildDerivedTimestamp` (Task 1), configuración guardada (Task 3).
- Produces: `POST /monitors/:id/backfill-timestamp` → `{"updated": N, "skipped": M}`.

- [ ] **Step 1: Agregar el método de repositorio**

En `backend/internal/repository/monitor_repo.go`, usando el helper `r.GetDataCollection(collectionID)` que ya usan los demás métodos sobre colecciones dinámicas (ver `AggregateData`, línea 240-241):

```go
// IterateData recorre todos los documentos de la colección de datos del
// monitor, aplicando fn a cada uno. Se usa para el backfill del timestamp
// derivado: cargar toda la colección en memoria no escala.
func (r *MonitorRepository) IterateData(ctx context.Context, collectionID string, fn func(bson.M) error) error {
	col := r.GetDataCollection(collectionID)
	cursor, err := col.Find(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("finding data for backfill: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			return fmt.Errorf("decoding document: %w", err)
		}
		if err := fn(doc); err != nil {
			return err
		}
	}
	return cursor.Err()
}

// SetDataField escribe un solo campo en un documento de la colección de datos.
func (r *MonitorRepository) SetDataField(ctx context.Context, collectionID string, docID interface{}, field string, value interface{}) error {
	col := r.GetDataCollection(collectionID)
	_, err := col.UpdateByID(ctx, docID, bson.M{"$set": bson.M{field: value}})
	if err != nil {
		return fmt.Errorf("setting %s: %w", field, err)
	}
	return nil
}
```

- [ ] **Step 2: Agregar el handler**

En `backend/internal/handlers/monitor_handler.go`:

```go
// BackfillDerivedTimestamp calcula el timestamp derivado para los documentos
// ya ingeridos. Es idempotente: los documentos que ya lo tienen se saltean,
// así que correrlo dos veces no cambia nada la segunda vez.
func (h *MonitorHandler) BackfillDerivedTimestamp(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}

	monitor, err := h.monitorRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
	}
	if monitor.DerivedTimestamp == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "el monitor no tiene timestamp derivado configurado"})
	}

	cfg := *monitor.DerivedTimestamp
	updated, skipped := 0, 0
	err = h.monitorRepo.IterateData(c.Context(), monitor.CollectionID, func(doc bson.M) error {
		if _, ya := doc[cfg.TargetName]; ya {
			return nil
		}
		ts, ok := services.BuildDerivedTimestamp(cfg, doc)
		if !ok {
			skipped++
			return nil
		}
		if err := h.monitorRepo.SetDataField(c.Context(), monitor.CollectionID, doc["_id"], cfg.TargetName, ts); err != nil {
			return err
		}
		updated++
		return nil
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"updated": updated, "skipped": skipped})
}
```

- [ ] **Step 3: Registrar la ruta**

```go
	monitors.Post("/:id/backfill-timestamp", middleware.RequireComplianceOrAbove(), h.Monitor.BackfillDerivedTimestamp)
```

- [ ] **Step 4: Verificar que compila y el gate pasa**

Run: `cd backend && go build ./... && go vet ./... && gofmt -l . && go test ./...`
Expected: sin errores.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/repository/monitor_repo.go backend/internal/handlers/monitor_handler.go backend/internal/router/router.go
git commit -m "feat(timestamp): backfill del timestamp derivado para datos ya ingeridos"
```

---

### Task 5: Modelo y validación de `VelocityCondition`

**Files:**
- Modify: `backend/internal/models/rule.go`
- Create: `backend/internal/services/velocity_validate.go`
- Test: `backend/internal/services/velocity_validate_test.go`

**Interfaces:**
- Produces: `models.VelocityCondition`, `models.Rule.VelocityConditions []VelocityCondition`, `services.ValidateVelocityCondition(cond models.VelocityCondition, schema []models.SchemaField) error`. Task 6 consume el modelo; el handler de creación de reglas consume la validación.

- [ ] **Step 1: Agregar el modelo**

En `backend/internal/models/rule.go`, después de `AggregateCondition`:

```go
// VelocityCondition detecta eventos consecutivos demasiado próximos en el
// tiempo dentro de una misma entidad (tarjeta, cliente, comercio).
// Es distinta de AggregateCondition: allí la ventana está anclada a "ahora"
// y se cuenta cuántos eventos caen dentro; acá se mide la distancia entre un
// evento y el inmediatamente anterior de la misma partición.
type VelocityCondition struct {
	TimeField string      `bson:"time_field" json:"timeField"` // campo date (típicamente el derivado)
	MaxGap    string      `bson:"max_gap" json:"maxGap"`       // "35s", "5min" — mismo parser que TimeWindow
	GroupBy   string      `bson:"group_by" json:"groupBy"`     // ej. "tarjeta"
	MinEvents int         `bson:"min_events" json:"minEvents"` // 2 = un par (default)
	Filter    []Condition `bson:"filter,omitempty" json:"filter,omitempty"`
}
```

Y en el struct `Rule`, después de `AggregateConditions`:

```go
	VelocityConditions []VelocityCondition `bson:"velocity_conditions,omitempty" json:"velocityConditions,omitempty"`
```

Agregar el mismo campo a `CreateRuleRequest` (mirar cómo está declarado `AggregateConditions` ahí y replicarlo).

- [ ] **Step 2: Escribir los tests de validación (deben fallar)**

Crear `backend/internal/services/velocity_validate_test.go`:

```go
package services

import (
	"testing"

	"github.com/thureos/compliance/internal/models"
)

func schemaConTimestamp() []models.SchemaField {
	return []models.SchemaField{
		{Name: "timestamp", Type: models.FieldDate},
		{Name: "tarjeta", Type: models.FieldString},
		{Name: "importe", Type: models.FieldNumber},
	}
}

func baseVelocityCondition() models.VelocityCondition {
	return models.VelocityCondition{
		TimeField: "timestamp",
		MaxGap:    "35s",
		GroupBy:   "tarjeta",
		MinEvents: 2,
	}
}

func TestValidateVelocityCondition_Valida(t *testing.T) {
	if err := ValidateVelocityCondition(baseVelocityCondition(), schemaConTimestamp()); err != nil {
		t.Errorf("esperaba condición válida, got %v", err)
	}
}

func TestValidateVelocityCondition_TimeFieldVacio(t *testing.T) {
	cond := baseVelocityCondition()
	cond.TimeField = ""
	if err := ValidateVelocityCondition(cond, schemaConTimestamp()); err == nil {
		t.Error("sin TimeField debería rechazarse")
	}
}

// El motor mide diferencias de tiempo: un campo que no es date no sirve, y
// aceptarlo en silencio produciría una regla que nunca dispara.
func TestValidateVelocityCondition_TimeFieldNoEsDate(t *testing.T) {
	cond := baseVelocityCondition()
	cond.TimeField = "importe"
	if err := ValidateVelocityCondition(cond, schemaConTimestamp()); err == nil {
		t.Error("un TimeField que no es de tipo date debería rechazarse")
	}
}

func TestValidateVelocityCondition_MaxGapInvalido(t *testing.T) {
	cond := baseVelocityCondition()
	cond.MaxGap = "35x"
	if err := ValidateVelocityCondition(cond, schemaConTimestamp()); err == nil {
		t.Error("un MaxGap que no parsea debería rechazarse")
	}
}

func TestValidateVelocityCondition_GroupByInexistente(t *testing.T) {
	cond := baseVelocityCondition()
	cond.GroupBy = "no_existe"
	if err := ValidateVelocityCondition(cond, schemaConTimestamp()); err == nil {
		t.Error("un GroupBy fuera del esquema debería rechazarse")
	}
}

func TestValidateVelocityCondition_MinEventsMenorADos(t *testing.T) {
	cond := baseVelocityCondition()
	cond.MinEvents = 1
	if err := ValidateVelocityCondition(cond, schemaConTimestamp()); err == nil {
		t.Error("MinEvents < 2 no tiene sentido: hace falta un par para medir un gap")
	}
}
```

- [ ] **Step 3: Correr los tests, confirmar que fallan**

Run: `cd backend && go test ./internal/services/... -run TestValidateVelocityCondition -v`
Expected: FAIL — `undefined: ValidateVelocityCondition`.

- [ ] **Step 4: Implementar la validación**

Crear `backend/internal/services/velocity_validate.go`:

```go
package services

import (
	"fmt"

	"github.com/thureos/compliance/internal/models"
)

// ValidateVelocityCondition rechaza al guardar las condiciones que no
// podrían disparar nunca. El precedente que justifica esta severidad: la
// ventana temporal rota vivió meses en producción porque fallaba en silencio
// al evaluar en vez de rechazarse al configurarse.
func ValidateVelocityCondition(cond models.VelocityCondition, schema []models.SchemaField) error {
	byName := make(map[string]models.SchemaField, len(schema))
	for _, f := range schema {
		byName[f.Name] = f
	}

	if cond.TimeField == "" {
		return fmt.Errorf("la condición de velocidad necesita un campo de tiempo")
	}
	timeField, ok := byName[cond.TimeField]
	if !ok {
		return fmt.Errorf("el campo de tiempo %q no existe en el esquema del monitor", cond.TimeField)
	}
	if timeField.Type != models.FieldDate {
		return fmt.Errorf("el campo de tiempo %q es de tipo %s: se necesita un campo date", cond.TimeField, timeField.Type)
	}
	if parseTimeWindow(cond.MaxGap) <= 0 {
		return fmt.Errorf("gap máximo inválido: %q (formatos válidos: 35s, 5min, 24h, 7d)", cond.MaxGap)
	}
	if cond.GroupBy != "" {
		if _, ok := byName[cond.GroupBy]; !ok {
			return fmt.Errorf("el campo de agrupación %q no existe en el esquema del monitor", cond.GroupBy)
		}
	}
	if cond.MinEvents < 2 {
		return fmt.Errorf("el mínimo de eventos debe ser al menos 2: hace falta un par para medir un gap")
	}
	for _, f := range cond.Filter {
		if _, ok := byName[f.Field]; !ok {
			return fmt.Errorf("el filtro referencia un campo inexistente: %q", f.Field)
		}
	}
	return nil
}
```

- [ ] **Step 5: Correr los tests, confirmar que pasan**

Run: `cd backend && go test ./internal/services/... -run TestValidateVelocityCondition -v`
Expected: PASS (6 tests).

- [ ] **Step 6: Enganchar la validación al guardar reglas**

En `backend/internal/handlers/rule_handler.go`, en el handler de creación de reglas: la `models.Rule` se arma en la línea 134-145, justo después de resolver `monitorID` (línea 127-130) y `userID` (línea 132). Insertar la validación **antes** de armar la regla, entre la línea 132 y la 134:

```go
	// Las condiciones de velocidad se validan contra el schema del monitor
	// al guardar: una condición que apunta a un campo que no es date jamás
	// dispararía, y aceptarla en silencio repite el modo de falla que dejó
	// la ventana temporal rota en producción.
	if len(req.VelocityConditions) > 0 {
		monitor, err := h.monitorRepo.FindByID(c.Context(), monitorID)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
		}
		for _, vc := range req.VelocityConditions {
			if err := services.ValidateVelocityCondition(vc, monitor.Schema); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
			}
		}
	}
```

Y agregar `VelocityConditions: req.VelocityConditions,` al literal `models.Rule` (después de `AggregateConditions`, línea 139).

`RuleHandler` ya tiene `monitorRepo` en su struct (`rule_handler.go:16`) y `services` ya está importado en el archivo, así que no hace falta tocar el constructor ni `main.go`.

- [ ] **Step 7: Verificar el gate**

Run: `cd backend && go build ./... && go vet ./... && gofmt -l . && go test ./...`
Expected: sin errores.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/models/rule.go backend/internal/services/velocity_validate.go backend/internal/services/velocity_validate_test.go backend/internal/handlers/rule_handler.go
git commit -m "feat(velocidad): modelo VelocityCondition y validación al guardar la regla"
```

---

### Task 6: Pipeline y evaluación de la condición de velocidad

**Files:**
- Modify: `backend/internal/services/rule_engine.go`
- Test: `backend/internal/services/rule_engine_test.go`

**Interfaces:**
- Consumes: `models.VelocityCondition` (Task 5), el campo derivado como `TimeField` (Tasks 1-4).
- Produces: `buildVelocityPipeline(cond models.VelocityCondition) mongo.Pipeline` y `(*RuleEngine).evaluateVelocityCondition(...) ([]models.RedFlag, error)`; banderas rojas de velocidad emitidas en el flujo normal de evaluación.

- [ ] **Step 1: Escribir los tests del pipeline (deben fallar)**

Agregar en `backend/internal/services/rule_engine_test.go`:

```go
func baseVelocityCond() models.VelocityCondition {
	return models.VelocityCondition{
		TimeField: "timestamp",
		MaxGap:    "35s",
		GroupBy:   "tarjeta",
		MinEvents: 2,
	}
}

// La forma del pipeline es el contrato: ordenar, calcular el evento
// anterior por partición, medir la diferencia y quedarse con los pares
// demasiado próximos.
func TestBuildVelocityPipeline_Forma(t *testing.T) {
	pipeline := buildVelocityPipeline(baseVelocityCond())
	assertStages(t, pipelineStageKeys(pipeline), []string{"$sort", "$setWindowFields", "$addFields", "$match"})
}

func TestBuildVelocityPipeline_ConFiltroAgregaMatchAlPrincipio(t *testing.T) {
	cond := baseVelocityCond()
	cond.Filter = []models.Condition{
		{Field: "importe", Operator: models.OpGreaterThan, Value: 5000.0},
		{Field: "mcc", Operator: models.OpEquals, Value: 7995.0},
	}
	pipeline := buildVelocityPipeline(cond)
	assertStages(t, pipelineStageKeys(pipeline), []string{"$match", "$sort", "$setWindowFields", "$addFields", "$match"})

	wantFilter := BuildMongoFilter(models.ConditionGroup{Logic: models.LogicAND, Conditions: cond.Filter})
	gotFilter := stageOperand(t, pipeline[0], "$match")
	if fmt.Sprintf("%v", gotFilter) != fmt.Sprintf("%v", wantFilter) {
		t.Errorf("el primer $match debería ser el filtro previo\ngot:  %v\nwant: %v", gotFilter, wantFilter)
	}
}

// El $match final compara contra un número de segundos, no contra un string
// ni una fecha: es la diferencia ya calculada por $dateDiff.
func TestBuildVelocityPipeline_ComparaGapEnSegundos(t *testing.T) {
	pipeline := buildVelocityPipeline(baseVelocityCond())
	final := stageOperand(t, pipeline[len(pipeline)-1], "$match")

	gapCond, ok := final["gapSeconds"].(bson.M)
	if !ok {
		t.Fatalf("esperaba una condición sobre gapSeconds, got %#v", final["gapSeconds"])
	}
	lte, ok := gapCond["$lte"]
	if !ok {
		t.Fatalf("esperaba $lte sobre gapSeconds, got %#v", gapCond)
	}
	if fmt.Sprintf("%v", lte) != "35" {
		t.Errorf("$lte = %v, want 35 (segundos de '35s')", lte)
	}
}

// Sin evento anterior no hay gap que medir: la primera transacción de cada
// partición no puede disparar por sí sola.
func TestBuildVelocityPipeline_DescartaElPrimeroDeCadaParticion(t *testing.T) {
	pipeline := buildVelocityPipeline(baseVelocityCond())
	final := stageOperand(t, pipeline[len(pipeline)-1], "$match")
	if _, ok := final["prevTime"]; !ok {
		t.Error("el $match final debería descartar los documentos sin evento anterior (prevTime null)")
	}
}
```

Confirmar que el bloque `import` del archivo de tests incluye `"fmt"` — si falta, agregarlo.

- [ ] **Step 2: Correr los tests, confirmar que fallan**

Run: `cd backend && go test ./internal/services/... -run TestBuildVelocityPipeline -v`
Expected: FAIL — `undefined: buildVelocityPipeline`.

- [ ] **Step 3: Implementar el pipeline**

En `backend/internal/services/rule_engine.go`, junto a `buildAggregatePipeline`:

```go
// buildVelocityPipeline arma el pipeline que detecta eventos consecutivos
// demasiado próximos dentro de una misma partición.
// Pipeline: [$match filtro] → $sort → $setWindowFields (evento anterior) →
// $addFields (diferencia en segundos) → $match (pares por debajo del gap).
// Validado contra MongoDB 7.0.40: $setWindowFields, $shift y $dateDiff
// ejecutan correctamente sobre la colección real del monitor.
func buildVelocityPipeline(cond models.VelocityCondition) mongo.Pipeline {
	pipeline := mongo.Pipeline{}

	if len(cond.Filter) > 0 {
		pipeline = append(pipeline, bson.D{
			{Key: "$match", Value: BuildMongoFilter(models.ConditionGroup{
				Logic: models.LogicAND, Conditions: cond.Filter,
			})},
		})
	}

	sortKeys := bson.D{}
	if cond.GroupBy != "" {
		sortKeys = append(sortKeys, bson.E{Key: cond.GroupBy, Value: 1})
	}
	sortKeys = append(sortKeys, bson.E{Key: cond.TimeField, Value: 1})
	pipeline = append(pipeline, bson.D{{Key: "$sort", Value: sortKeys}})

	var partitionBy interface{}
	if cond.GroupBy != "" {
		partitionBy = "$" + cond.GroupBy
	}
	pipeline = append(pipeline, bson.D{
		{Key: "$setWindowFields", Value: bson.D{
			{Key: "partitionBy", Value: partitionBy},
			{Key: "sortBy", Value: bson.D{{Key: cond.TimeField, Value: 1}}},
			{Key: "output", Value: bson.D{
				{Key: "prevTime", Value: bson.D{
					{Key: "$shift", Value: bson.D{
						{Key: "output", Value: "$" + cond.TimeField},
						{Key: "by", Value: -1},
					}},
				}},
			}},
		}},
	})

	pipeline = append(pipeline, bson.D{
		{Key: "$addFields", Value: bson.D{
			{Key: "gapSeconds", Value: bson.D{
				{Key: "$dateDiff", Value: bson.D{
					{Key: "startDate", Value: "$prevTime"},
					{Key: "endDate", Value: "$" + cond.TimeField},
					{Key: "unit", Value: "second"},
				}},
			}},
		}},
	})

	maxGapSeconds := int64(parseTimeWindow(cond.MaxGap) / time.Second)
	pipeline = append(pipeline, bson.D{
		{Key: "$match", Value: bson.D{
			{Key: "prevTime", Value: bson.D{{Key: "$ne", Value: nil}}},
			{Key: "gapSeconds", Value: bson.D{{Key: "$lte", Value: maxGapSeconds}}},
		}},
	})

	return pipeline
}
```

- [ ] **Step 4: Correr los tests, confirmar que pasan**

Run: `cd backend && go test ./internal/services/... -run TestBuildVelocityPipeline -v`
Expected: PASS (4 tests).

- [ ] **Step 5: Implementar la evaluación**

En el mismo archivo, junto a `evaluateAggregateCondition` (línea ~385), siguiendo su misma forma:

```go
// evaluateVelocityCondition emite una bandera roja por cada par de eventos
// consecutivos separados por menos del gap configurado. El documento que
// devuelve el pipeline ES el segundo evento del par, y lleva prevTime y
// gapSeconds adjuntos: eso es lo que se muestra en la alerta.
func (e *RuleEngine) evaluateVelocityCondition(
	ctx context.Context,
	monitor *models.Monitor,
	rule models.Rule,
	cond models.VelocityCondition,
) ([]models.RedFlag, error) {
	pipeline := buildVelocityPipeline(cond)

	results, err := e.monitorRepo.AggregateData(ctx, monitor.CollectionID, pipeline)
	if err != nil {
		return nil, fmt.Errorf("velocity evaluation: %w", err)
	}

	today := time.Now()
	var redFlags []models.RedFlag
	for _, result := range results {
		groupKey := ""
		if cond.GroupBy != "" {
			groupKey = fmt.Sprintf("%v", result[cond.GroupBy])
		}
		gapSeconds := toFloat(result["gapSeconds"])

		redFlags = append(redFlags, models.RedFlag{
			Fingerprint:  models.AggRedFlagFingerprint(rule.ID, monitor.ID, today, groupKey+"|"+fmt.Sprintf("%v", result["_id"])),
			MonitorID:    monitor.ID,
			RuleID:       rule.ID,
			RuleName:     rule.Name,
			MonitorName:  monitor.Name,
			Severity:     rule.Severity,
			RedFlagType:  models.RedFlagTypeAggregate,
			GroupByField: cond.GroupBy,
			GroupByValue: groupKey,
			Message: fmt.Sprintf(
				"Regla de velocidad '%s': dos transacciones de %s='%s' separadas por %.0f segundos (máximo configurado: %s)",
				rule.Name, cond.GroupBy, groupKey, gapSeconds, cond.MaxGap,
			),
			MatchedData: result,
			MatchCount:  cond.MinEvents,
		})
	}
	return redFlags, nil
}
```

El fingerprint incluye el `_id` del documento además del grupo: dos pares distintos de la misma tarjeta en el mismo día son dos alertas distintas, no una sola deduplicada.

- [ ] **Step 6: Enganchar la evaluación al flujo**

En `rule_engine.go`, inmediatamente después del bucle de `rule.AggregateConditions` (que termina en la línea ~186), agregar el bucle equivalente para velocidad, replicando **exactamente** el mismo manejo de upsert, contador y disparadores:

```go
		// Evaluate velocity conditions (gap entre eventos consecutivos)
		for _, velCond := range rule.VelocityConditions {
			velRedFlags, err := e.evaluateVelocityCondition(ctx, monitor, rule, velCond)
			if err != nil {
				log.Printf("ERROR velocity condition for rule %s field=%s: %v", rule.ID.Hex(), velCond.TimeField, err)
				continue
			}
			for i := range velRedFlags {
				isNew, err := e.redFlagRepo.Upsert(ctx, &velRedFlags[i])
				if err != nil {
					log.Printf("ERROR upserting velocity red flag for rule %s: %v", rule.ID.Hex(), err)
					continue
				}
				redFlags = append(redFlags, velRedFlags[i])
				ruleRedFlagCount++
				if isNew {
					if err := e.ruleRepo.IncrementTriggerCount(ctx, rule.ID); err != nil {
						log.Printf("WARNING: failed to increment trigger count for rule %s: %v", rule.ID.Hex(), err)
					}
					e.triggerReportGeneration(velRedFlags[i])
					e.triggerNotification(velRedFlags[i], isNew)
					e.triggerScreening(velRedFlags[i], rule, isNew)
				}
			}
		}
```

- [ ] **Step 7: Correr el gate completo**

Run: `cd backend && go build ./... && go vet ./... && gofmt -l . && go test ./...`
Expected: todo en verde, incluidos los tests existentes sin cambios.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/services/rule_engine.go backend/internal/services/rule_engine_test.go
git commit -m "feat(velocidad): pipeline y evaluación de gap entre eventos consecutivos"
```

---

## Criterio de cierre del lote

- Gate (`go build ./...`, `go vet ./...`, `gofmt -l .`, `go test ./...`) sin bloqueantes en las 6 tareas.
- Recorrido manual end-to-end contra un entorno con datos (el monitor `CBCG Tarjetas` de producción es el caso real, pero el recorrido debe hacerse sobre una copia o un monitor de prueba, **no** creando reglas en producción):
  1. Configurar el timestamp derivado (`fechapoliza` YYYYMMDD + `horapoliza` HHMMSS → `timestamp`) vía `PUT /monitors/:id/derived-timestamp` → el schema devuelto incluye `timestamp` como campo `date`.
  2. Correr `POST /monitors/:id/backfill-timestamp` → responde con cuántos documentos actualizó; verificar en Mongo que `timestamp` quedó como fecha real y que `021517` se convirtió en `02:15:17`, no en `21:51:07`.
  3. Correr el backfill una segunda vez → `updated: 0` (idempotente).
  4. Subir un archivo nuevo → los documentos nuevos ya traen `timestamp` sin necesidad de backfill.
  5. Crear una regla con una `VelocityCondition` (`timeField: timestamp`, `maxGap: 35s`, `groupBy: tarjeta`, `minEvents: 2`, filtro `importe > 5000` y `mcc = 7995`) → se guarda con `201`.
  6. Ejecutar la regla → salta una bandera roja por cada par de transacciones de la misma tarjeta separadas por menos de 35 segundos, con el gap real en el mensaje.
  7. Intentar guardar una `VelocityCondition` con `timeField` apuntando a un campo numérico → `400` con mensaje explícito (no se acepta en silencio).
- Confirmar que un monitor sin timestamp derivado y una regla sin condiciones de velocidad siguen comportándose exactamente igual que antes.
