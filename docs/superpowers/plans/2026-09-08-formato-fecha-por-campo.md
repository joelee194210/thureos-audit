# Formato de Fecha Configurable por Campo Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Permitir configurar, por campo `date` de un monitor, en qué formato vienen sus valores (un preset fijo de 4 formatos comunes — `DD/MM/YYYY`, `MM/DD/YYYY`, `DD-MM-YYYY`, `YYYY/MM/DD`), para que la validación y el parseo de cada carga posterior lo acepten en lugar de rechazarlo, sin dejar de aceptar ISO/RFC3339 en los campos que nunca se configuran.

**Architecture:** Un campo nuevo `DateFormat` en `models.SchemaField` (guarda una clave de preset, no un layout crudo de Go), una tabla exportada `services.DateFormatPresets` que resuelve esa clave al layout real, y una función pura `services.IsValidDateFormatPreset` para validarla. Se aplica en `validateFieldValue` y `parseValue` (ya existentes) de forma estricta y no aditiva: si el campo tiene `DateFormat` configurado, deja de aceptar ISO/RFC3339 para ese campo. `PUT /monitors/:id/schema` (ya existe, del plan de decimales implícitos) gana la validación del preset. Edición en la pestaña "Esquema", mismo patrón visual que el `Select` de `impliedDecimals`.

**Tech Stack:** Go/Fiber/MongoDB (backend), Next.js/React/TypeScript (frontend) — mismo stack, sin dependencias nuevas.

**Spec:** `docs/superpowers/specs/2026-09-08-formato-fecha-por-campo-design.md`

## Global Constraints

- `DateFormat == ""` (zero-value de Go, lo que ya tienen todos los campos existentes) es "sin formato configurado" — comportamiento actual sin cambios: acepta ISO (`YYYY-MM-DD`) o RFC3339. Ningún test ni código debe romper este caso.
- Comportamiento **estricto, no aditivo**: si `DateFormat != ""`, solo ese formato es válido — deja de aceptar ISO/RFC3339 para ese campo. No hay fallback ni combinación de formatos.
- Solo se guardan claves de preset reconocidas (`DateFormatPresets`), nunca un layout de Go crudo enviado por el cliente.
- Sin detección automática de formato en la carga inicial — `inferType`/`looksLikeDate` no cambian.
- Sin manejo de fechas seriales de Excel ni normalización de zona horaria — fuera de alcance (spec, "Fuera de alcance").
- Sin recalcular datos ya ingeridos — la configuración aplica desde el próximo upload en adelante.
- Sin test HTTP de `UpdateSchema` — el proyecto no mockea handlers (`monitorRepo` es un tipo concreto, no interfaz). La validación del preset se prueba como función pura (`IsValidDateFormatPreset`), seguir el patrón ya usado en `internal/handlers/dashboard_handler_test.go`.
- Sin tests de UI/componentes — mismo criterio que el resto del proyecto.

---

### Task 1: Modelo, tabla de presets y validador puro

**Files:**
- Modify: `backend/internal/models/monitor.go`
- Modify: `backend/internal/services/ingestion_service.go`
- Test: `backend/internal/services/ingestion_service_test.go`

**Interfaces:**
- Produces: `models.SchemaField.DateFormat string`. `services.DateFormatPresets map[string]string` (exportado). `services.IsValidDateFormatPreset(key string) bool` (exportado) — Task 2 y Task 3 lo consumen.

- [ ] **Step 1: Agregar el campo al modelo**

En `backend/internal/models/monitor.go`, dentro de `type SchemaField struct` (línea 28-34), agregar después de `ImpliedDecimals`:

```go
type SchemaField struct {
	Name            string    `bson:"name" json:"name"`
	Type            FieldType `bson:"type" json:"type"`
	Required        bool      `bson:"required" json:"required"`
	Sample          string    `bson:"sample" json:"sample"`
	ImpliedDecimals int       `bson:"implied_decimals,omitempty" json:"impliedDecimals,omitempty"`
	DateFormat      string    `bson:"date_format,omitempty" json:"dateFormat,omitempty"`
}
```

- [ ] **Step 2: Escribir los tests de `IsValidDateFormatPreset` (deben fallar — la función todavía no existe, no compila)**

En `backend/internal/services/ingestion_service_test.go`, agregar al final del archivo:

```go
func TestIsValidDateFormatPreset_EmptyIsValid(t *testing.T) {
	if !IsValidDateFormatPreset("") {
		t.Error("un DateFormat vacío (sin formato configurado) debe ser válido")
	}
}

func TestIsValidDateFormatPreset_KnownPresetsAreValid(t *testing.T) {
	for key := range DateFormatPresets {
		if !IsValidDateFormatPreset(key) {
			t.Errorf("%q está en DateFormatPresets y debería ser válido", key)
		}
	}
}

func TestIsValidDateFormatPreset_UnknownKeyIsInvalid(t *testing.T) {
	if IsValidDateFormatPreset("YYYY-DD-MM") {
		t.Error("una clave que no está en DateFormatPresets debe ser inválida")
	}
}
```

- [ ] **Step 3: Correr los tests, confirmar que fallan**

Run: `cd backend && go test ./internal/services/... -run TestIsValidDateFormatPreset -v`
Expected: FAIL — `undefined: IsValidDateFormatPreset` / `undefined: DateFormatPresets` (no compila).

- [ ] **Step 4: Implementar `DateFormatPresets` e `IsValidDateFormatPreset`**

En `backend/internal/services/ingestion_service.go`, agregar junto a `looksLikeDate` (después de la línea 627, antes de `parseValue`):

```go
// DateFormatPresets maps a SchemaField.DateFormat key to the Go time
// layout it represents. Only these 4 keys are valid — see
// IsValidDateFormatPreset.
var DateFormatPresets = map[string]string{
	"DD/MM/YYYY": "02/01/2006",
	"MM/DD/YYYY": "01/02/2006",
	"DD-MM-YYYY": "02-01-2006",
	"YYYY/MM/DD": "2006/01/02",
}

// IsValidDateFormatPreset reports whether key is either "" (sin formato
// configurado, comportamiento por defecto) or a recognized entry of
// DateFormatPresets.
func IsValidDateFormatPreset(key string) bool {
	if key == "" {
		return true
	}
	_, ok := DateFormatPresets[key]
	return ok
}
```

- [ ] **Step 5: Correr los tests, confirmar que pasan**

Run: `cd backend && go test ./internal/services/... -run TestIsValidDateFormatPreset -v`
Expected: PASS (3 tests).

- [ ] **Step 6: Commit**

```bash
git add backend/internal/models/monitor.go backend/internal/services/ingestion_service.go backend/internal/services/ingestion_service_test.go
git commit -m "feat(fecha): campo DateFormat, tabla de presets y validador"
```

---

### Task 2: Validación y parseo respetan `DateFormat`

**Files:**
- Modify: `backend/internal/services/ingestion_service.go`
- Test: `backend/internal/services/ingestion_service_test.go`

**Interfaces:**
- Consumes: `models.SchemaField.DateFormat`, `services.DateFormatPresets` (Task 1).
- Produces: `validateFieldValue` y `parseValue` con soporte de `DateFormat` para `FieldDate` — Task 3 no los modifica, pero `firstInvalidField`/`ingestDelimited`/`IngestExcel` los usan sin cambios en su propia firma.

- [ ] **Step 1: Escribir los tests (deben fallar)**

En `backend/internal/services/ingestion_service_test.go`, agregar:

```go
func TestValidateFieldValue_DateFormatDDMMYYYY_Valid(t *testing.T) {
	field := models.SchemaField{Type: models.FieldDate, DateFormat: "DD/MM/YYYY"}
	if !validateFieldValue("08/09/2026", field) {
		t.Error("'08/09/2026' debería ser válido con DateFormat=DD/MM/YYYY")
	}
}

func TestValidateFieldValue_DateFormatConfigured_RejectsISO(t *testing.T) {
	field := models.SchemaField{Type: models.FieldDate, DateFormat: "DD/MM/YYYY"}
	if validateFieldValue("2026-09-08", field) {
		t.Error("con DateFormat configurado, un valor en formato ISO debe rechazarse (validación estricta, no aditiva)")
	}
}

func TestValidateFieldValue_NoDateFormat_StillAcceptsISO(t *testing.T) {
	field := models.SchemaField{Type: models.FieldDate}
	if !validateFieldValue("2026-09-08", field) {
		t.Error("sin DateFormat configurado, ISO sigue siendo válido (comportamiento actual sin cambios)")
	}
}

func TestParseValue_DateFormatDDMMYYYY_ParsesCorrectLayout(t *testing.T) {
	schema := []models.SchemaField{{Name: "fecha", Type: models.FieldDate, DateFormat: "DD/MM/YYYY"}}
	got := parseValue("08/09/2026", schema, "fecha")
	tm, ok := got.(time.Time)
	if !ok {
		t.Fatalf("esperaba time.Time, obtuve %T (%v)", got, got)
	}
	if tm.Year() != 2026 || tm.Month() != time.September || tm.Day() != 8 {
		t.Errorf("got %v, want 8 de septiembre de 2026 (confirma que se usó DD/MM/YYYY, no MM/DD/YYYY)", tm)
	}
}

func TestParseValue_NoDateFormat_StillParsesISO(t *testing.T) {
	schema := []models.SchemaField{{Name: "fecha", Type: models.FieldDate}}
	got := parseValue("2026-09-08", schema, "fecha")
	tm, ok := got.(time.Time)
	if !ok {
		t.Fatalf("esperaba time.Time, obtuve %T", got)
	}
	if tm.Year() != 2026 || tm.Month() != time.September || tm.Day() != 8 {
		t.Errorf("got %v, want 8 de septiembre de 2026 (regresión: sin DateFormat, ISO debe seguir parseando igual que hoy)", tm)
	}
}
```

El bloque `import` de `ingestion_service_test.go` no incluye `"time"` todavía (solo
`context`, `encoding/json`, `errors`, `net/http`, `net/http/httptest`, `strings`,
`testing`, y `github.com/thureos/compliance/internal/models`) — agregarlo:

```go
import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/thureos/compliance/internal/models"
)
```

- [ ] **Step 2: Correr los tests, confirmar que fallan**

Run: `cd backend && go test ./internal/services/... -run 'TestValidateFieldValue_DateFormat|TestParseValue_DateFormat' -v`
Expected: FAIL — `TestValidateFieldValue_DateFormatDDMMYYYY_Valid` y `TestParseValue_DateFormatDDMMYYYY_ParsesCorrectLayout` fallan porque hoy `DateFormat` se ignora (solo se acepta ISO/RFC3339); `TestValidateFieldValue_DateFormatConfigured_RejectsISO` falla porque hoy SÍ acepta ISO aunque haya un preset configurado.

- [ ] **Step 3: Implementar el soporte en `validateFieldValue`**

En `backend/internal/services/ingestion_service.go`, reemplazar el `case models.FieldDate:` dentro de `validateFieldValue` (línea 674-679 actual):

```go
	case models.FieldDate:
		if expected.DateFormat != "" {
			_, err := time.Parse(DateFormatPresets[expected.DateFormat], raw)
			return err == nil
		}
		if _, err := time.Parse("2006-01-02", raw); err == nil {
			return true
		}
		_, err := time.Parse(time.RFC3339, raw)
		return err == nil
```

- [ ] **Step 4: Implementar el soporte en `parseValue`**

En el mismo archivo, reemplazar el `case models.FieldDate:` dentro de `parseValue` (línea 647-653 actual):

```go
			case models.FieldDate:
				if field.DateFormat != "" {
					if t, err := time.Parse(DateFormatPresets[field.DateFormat], value); err == nil {
						return t
					}
					break
				}
				if t, err := time.Parse("2006-01-02", value); err == nil {
					return t
				}
				if t, err := time.Parse(time.RFC3339, value); err == nil {
					return t
				}
```

El `break` es necesario: sin él, un valor que no matchea el `DateFormat` configurado caería a intentar ISO/RFC3339 igual — exactamente el comportamiento aditivo que el diseño descarta explícitamente.

- [ ] **Step 5: Correr los tests, confirmar que pasan**

Run: `cd backend && go test ./internal/services/... -v`
Expected: PASS — todos los tests del paquete, incluidos los 5 nuevos y los existentes de `ImpliedDecimals`/fecha sin cambios (regresión).

- [ ] **Step 6: Commit**

```bash
git add backend/internal/services/ingestion_service.go backend/internal/services/ingestion_service_test.go
git commit -m "feat(fecha): validar y parsear FieldDate según DateFormat configurado"
```

---

### Task 3: Endpoint `PUT /monitors/:id/schema` valida el preset

**Files:**
- Modify: `backend/internal/handlers/monitor_handler.go`

**Interfaces:**
- Consumes: `services.IsValidDateFormatPreset` (Task 1).

- [ ] **Step 1: Agregar la validación al handler**

En `backend/internal/handlers/monitor_handler.go`, dentro de `UpdateSchema`, después del bloque que valida que los nombres de campo coincidan (línea ~717-720, justo antes de `monitor.Schema = body.Schema`) agregar:

```go
	for _, f := range body.Schema {
		if !services.IsValidDateFormatPreset(f.DateFormat) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("formato de fecha inválido: %s", f.DateFormat)})
		}
	}

```

`monitor_handler.go` ya importa `"fmt"` (línea 9) y `"github.com/thureos/compliance/internal/services"` (usado por `services.IngestionService` en la struct del handler) — no hace falta tocar el bloque `import`.

- [ ] **Step 2: Verificar que compila**

Run: `cd backend && go build ./...`
Expected: sin errores.

- [ ] **Step 3: Verificación manual (opcional, requiere backend + MongoDB + Redis corriendo)**

Con el servidor levantado (`go run cmd/server/main.go`) y un token de un usuario `admin`/`compliance`:

```bash
curl -s -X PUT http://localhost:8080/api/v1/monitors/<id>/schema \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"schema":[{"name":"fecha","type":"date","required":true,"sample":"08/09/2026","dateFormat":"YYYY-DD-MM"}]}'
```

Expected: `400 {"error":"formato de fecha inválido: YYYY-DD-MM"}` (clave inexistente en `DateFormatPresets`). Repetir con `"dateFormat":"DD/MM/YYYY"` → `200`, el schema se guarda.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/handlers/monitor_handler.go
git commit -m "feat(fecha): validar preset de DateFormat en PUT /monitors/:id/schema"
```

---

### Task 4: Edición de formato de fecha en la pestaña "Esquema"

**Files:**
- Modify: `frontend/src/lib/types.ts`
- Modify: `frontend/src/components/tabs/content/monitor-tab-content.tsx`

**Interfaces:**
- Consumes: `PUT /monitors/:id/schema` (Task 3, sin cambio de firma — `monitorsApi.updateSchema(id, schema: SchemaField[])` ya existe).
- Produces: ninguna — última tarea del plan.

- [ ] **Step 1: Agregar el campo al tipo `SchemaField`**

En `frontend/src/lib/types.ts`, dentro de `export interface SchemaField` (línea 52-58), agregar después de `impliedDecimals`:

```ts
export interface SchemaField {
  name: string;
  type: "string" | "number" | "date" | "boolean";
  required: boolean;
  sample: string;
  impliedDecimals?: number;
  dateFormat?: string;
}
```

- [ ] **Step 2: Agregar el handler de edición**

En `frontend/src/components/tabs/content/monitor-tab-content.tsx`, justo después de `updateImpliedDecimals` (línea 181-185), agregar:

```tsx
  function updateDateFormat(fieldName: string, value: string) {
    setSchemaEdits((prev) =>
      prev.map((f) => (f.name === fieldName ? { ...f, dateFormat: value } : f)),
    );
  }
```

- [ ] **Step 3: Agregar el control en la pestaña "Esquema"**

En el mismo archivo, dentro del `.map((field: SchemaField) => ...)` de la pestaña Esquema, inmediatamente después del bloque `{field.type === "number" && ... <Select> ... </Select>)}` (cierra en línea 806) y antes de `<Badge variant="secondary">{field.type}</Badge>` (línea 807), agregar:

```tsx
                          {field.type === "date" && user?.role !== "viewer" && (
                            <Select
                              value={field.dateFormat ?? "auto"}
                              onValueChange={(v) =>
                                updateDateFormat(field.name, v === "auto" ? "" : v)
                              }
                            >
                              <SelectTrigger className="w-[180px]">
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                <SelectItem value="auto">Automático (ISO / RFC3339)</SelectItem>
                                <SelectItem value="DD/MM/YYYY">DD/MM/YYYY</SelectItem>
                                <SelectItem value="MM/DD/YYYY">MM/DD/YYYY</SelectItem>
                                <SelectItem value="DD-MM-YYYY">DD-MM-YYYY</SelectItem>
                                <SelectItem value="YYYY/MM/DD">YYYY/MM/DD</SelectItem>
                              </SelectContent>
                            </Select>
                          )}
```

`"auto"` es un valor centinela: Radix `Select`/`SelectItem` no admite `value=""`, y `"auto"` es lo que se traduce de vuelta a `""` (sin formato configurado) al guardar — mismo motivo por el que no se puede usar la cadena vacía directamente.

- [ ] **Step 4: Verificar que compila**

Run: `cd frontend && npx tsc --noEmit`
Expected: sin errores.

- [ ] **Step 5: Verificar lint**

Run: `cd frontend && npm run lint`
Expected: sin errores nuevos (los warnings preexistentes de `exhaustive-deps`/`no-location-assign` no cuentan).

- [ ] **Step 6: Verificación manual**

Con backend + frontend levantados: crear o reusar un monitor con un campo `date`, en la pestaña "Esquema" configurar `DD/MM/YYYY` y Guardar → confirmar toast "Esquema actualizado". Subir un archivo con ese campo en `08/09/2026` → se acepta. Subir un archivo con el mismo campo en `2026-09-08` (ISO) → se rechaza, motivo visible en la bitácora de cargas (`/uploads`). Loguearse como `viewer` → el `Select` no aparece en la pestaña Esquema para ningún campo (ni el de decimales ni el de fecha).

- [ ] **Step 7: Commit**

```bash
git add frontend/src/lib/types.ts frontend/src/components/tabs/content/monitor-tab-content.tsx
git commit -m "feat(fecha): editar formato de fecha por campo en la pestaña Esquema"
```

---

## Criterio de cierre del lote

- Gate (`go build ./...`, `go test ./...`, `npx tsc --noEmit`, `npm run lint`, `npm run build`) sin bloqueantes en las 4 tareas.
- Recorrido manual:
  - Configurar `DD/MM/YYYY` en un campo `fecha`, subir un archivo con `08/09/2026` → se guarda como 8 de septiembre de 2026 (no 9 de agosto).
  - Subir al mismo monitor un archivo donde ese campo ya trae fecha en ISO (`2026-09-08`) → esa fila se rechaza, reflejado en la bitácora de cargas.
  - Dejar un campo `date` sin `DateFormat` configurado → sigue aceptando ISO/RFC3339, comportamiento idéntico al actual.
  - `PUT /monitors/:id/schema` con un `DateFormat` fuera de `DateFormatPresets` → `400`.
  - Confirmar que un usuario `viewer` no puede editar el esquema (ni decimales ni formato de fecha).
