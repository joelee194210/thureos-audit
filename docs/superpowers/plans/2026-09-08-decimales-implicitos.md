# Decimales Implícitos Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Permitir configurar, por campo numérico de un monitor, cuántos decimales implícitos tiene un valor crudo (ej. `500000` con 2 decimales implícitos = `5000.00`), aplicando la conversión en la ingesta y rechazando filas cuyo valor ya trae punto decimal cuando esa configuración está activa.

**Architecture:** Un campo nuevo `ImpliedDecimals` en `models.SchemaField`, aplicado en `parseValue` (división) y en `validateFieldValue` (rechazo si el valor crudo ya tiene punto decimal) — ambas funciones ya existentes del plan de la bitácora de cargas. Un endpoint nuevo `PUT /monitors/:id/schema` para configurarlo, y una edición mínima en la pestaña "Esquema" (hoy de solo lectura) del detalle de monitor.

**Tech Stack:** Go/Fiber/MongoDB (backend), Next.js/React/TypeScript (frontend) — mismo stack, sin dependencias nuevas.

**Spec:** `docs/superpowers/specs/2026-09-08-decimales-implicitos-design.md`

## Global Constraints

- `ImpliedDecimals == 0` (el zero-value de Go, y lo que ya tienen todos los campos existentes) es "sin decimales implícitos" — el comportamiento actual, sin cambios, para cualquier campo que nunca se configure. Ningún test ni código debe romper este caso.
- Solo aplica semánticamente a campos `Type == FieldNumber` — en otros tipos el valor se ignora, sin error.
- Sin recalcular datos ya ingeridos — la configuración aplica desde el próximo upload en adelante (spec, "Fuera de alcance").
- `PUT /monitors/:id/schema` solo permite editar `ImpliedDecimals` de campos ya existentes — si el conjunto de nombres del schema recibido no coincide exactamente con el actual, se rechaza con `400`.
- Sin tests de UI/componentes — mismo criterio que el resto del proyecto.

⚠️ **Precondición de secuenciación (no técnica, de proceso):** este plan modifica `backend/internal/services/ingestion_service.go` (`parseValue`, `validateFieldValue`) y `backend/internal/models/monitor.go` (`SchemaField`) — los mismos archivos que el plan `docs/superpowers/plans/2026-09-08-bitacora-cargas.md` reescribe extensamente. **No despachar ninguna tarea de este plan hasta que ese otro plan complete su ejecución** (todas sus tareas + review final), para no tener dos procesos de subagent-driven-development escribiendo el mismo archivo en paralelo sobre una base movediza.

---

### Task 1: `ImpliedDecimals` en el modelo + división en `parseValue`

**Files:**
- Modify: `backend/internal/models/monitor.go`
- Modify: `backend/internal/services/ingestion_service.go`
- Test: `backend/internal/services/ingestion_service_test.go`

**Interfaces:**
- Produces: `models.SchemaField.ImpliedDecimals int` (Tareas 2-5 lo consumen). `parseValue` con soporte de división — Tarea 2 no lo modifica, pero corre en el mismo call-path.

- [ ] **Step 1: Agregar el campo al modelo**

En `backend/internal/models/monitor.go`, buscar:

```go
type SchemaField struct {
	Name     string    `bson:"name" json:"name"`
	Type     FieldType `bson:"type" json:"type"`
	Required bool      `bson:"required" json:"required"`
	Sample   string    `bson:"sample" json:"sample"`
}
```

Reemplazar por:

```go
type SchemaField struct {
	Name            string    `bson:"name" json:"name"`
	Type            FieldType `bson:"type" json:"type"`
	Required        bool      `bson:"required" json:"required"`
	Sample          string    `bson:"sample" json:"sample"`
	ImpliedDecimals int       `bson:"implied_decimals,omitempty" json:"impliedDecimals,omitempty"`
}
```

- [ ] **Step 2: Escribir los tests de `parseValue` con decimales implícitos (deben fallar)**

Agregar al final de `backend/internal/services/ingestion_service_test.go`:

```go
func TestParseValue_ImpliedDecimalsDividesCorrectly(t *testing.T) {
	schema := []models.SchemaField{{Name: "monto", Type: models.FieldNumber, ImpliedDecimals: 2}}
	got := parseValue("500000", schema, "monto")
	f, ok := got.(float64)
	if !ok {
		t.Fatalf("esperaba float64, obtuve %T", got)
	}
	if f != 5000.0 {
		t.Errorf("got %v, want 5000.0", f)
	}
}

func TestParseValue_ImpliedDecimalsZeroNoChange(t *testing.T) {
	schema := []models.SchemaField{{Name: "monto", Type: models.FieldNumber, ImpliedDecimals: 0}}
	got := parseValue("500000", schema, "monto")
	f, ok := got.(float64)
	if !ok {
		t.Fatalf("esperaba float64, obtuve %T", got)
	}
	if f != 500000.0 {
		t.Errorf("got %v, want 500000.0 (sin cambios cuando ImpliedDecimals=0)", f)
	}
}

func TestParseValue_ImpliedDecimalsSmallValue(t *testing.T) {
	schema := []models.SchemaField{{Name: "monto", Type: models.FieldNumber, ImpliedDecimals: 2}}
	got := parseValue("5", schema, "monto")
	f, ok := got.(float64)
	if !ok {
		t.Fatalf("esperaba float64, obtuve %T", got)
	}
	if f != 0.05 {
		t.Errorf("got %v, want 0.05", f)
	}
}

func TestParseValue_InvalidNumberFallsBackToRawString(t *testing.T) {
	schema := []models.SchemaField{{Name: "monto", Type: models.FieldNumber, ImpliedDecimals: 2}}
	got := parseValue("no-es-numero", schema, "monto")
	if got != "no-es-numero" {
		t.Errorf("got %v, want el string crudo sin parsear", got)
	}
}
```

- [ ] **Step 3: Correr los tests, confirmar que fallan**

Run: `cd backend && go test ./internal/services/... -run TestParseValue_ImpliedDecimals -v`
Expected: FAIL — `f != 5000.0` (la división todavía no existe, así que `TestParseValue_ImpliedDecimalsDividesCorrectly` da `500000` en vez de `5000`). Los otros 3 tests pueden pasar de entrada (no dependen de la división) — igual confirmá que compilan y corren.

- [ ] **Step 4: Implementar la división en `parseValue`**

Agregar `"math"` al bloque de imports existente de `ingestion_service.go`.

Buscar, dentro de `func parseValue(...)`:

```go
			case models.FieldNumber:
				if f, err := strconv.ParseFloat(value, 64); err == nil {
					return f
				}
```

Reemplazar por:

```go
			case models.FieldNumber:
				if f, err := strconv.ParseFloat(value, 64); err == nil {
					if field.ImpliedDecimals > 0 {
						f = f / math.Pow(10, float64(field.ImpliedDecimals))
					}
					return f
				}
```

- [ ] **Step 5: Correr los tests, confirmar que pasan**

Run: `cd backend && go test ./internal/services/... -run TestParseValue_ImpliedDecimals -v`
Expected: PASS (4/4)

Run también: `cd backend && go test ./internal/services/... -v` (suite completa) — confirmar que nada existente se rompió.
Expected: todos los tests existentes (incluidos los de las Tareas 1-2 del plan de la bitácora) siguen en PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/models/monitor.go backend/internal/services/ingestion_service.go backend/internal/services/ingestion_service_test.go
git commit -m "feat(decimales): campo ImpliedDecimals y división en parseValue"
```

---

### Task 2: Rechazo de filas con punto decimal cuando hay decimales implícitos configurados

**Files:**
- Modify: `backend/internal/services/ingestion_service.go`
- Test: `backend/internal/services/ingestion_service_test.go`

**Interfaces:**
- Consumes: `models.SchemaField.ImpliedDecimals` (Tarea 1).
- Produces: `validateFieldValue(raw string, expected models.SchemaField) bool` — firma CAMBIADA (antes tomaba `models.FieldType`) — Tareas 3+ y cualquier código futuro que la use deben usar la firma nueva. `firstInvalidField` no cambia su propia firma, solo el argumento que le pasa a `validateFieldValue`.

⚠️ **Esta tarea cambia la firma de una función ya aprobada y con tests existentes** (Tarea 2 del plan de la bitácora de cargas). Los 8 call-sites de test existentes deben actualizarse — están listados exactos en el Step 2, no hay que buscarlos a mano.

- [ ] **Step 1: Escribir los 2 tests nuevos (deben fallar — todavía ni compila, porque la firma vieja no acepta un `models.SchemaField` como segundo argumento)**

Agregar al final de `backend/internal/services/ingestion_service_test.go`:

```go
func TestValidateFieldValue_ImpliedDecimalsRejectsDotInRawValue(t *testing.T) {
	field := models.SchemaField{Type: models.FieldNumber, ImpliedDecimals: 2}
	if validateFieldValue("500.5", field) {
		t.Error("un valor con punto decimal en un campo con ImpliedDecimals>0 debería ser inválido")
	}
}

func TestValidateFieldValue_NoImpliedDecimalsAcceptsDotInRawValue(t *testing.T) {
	field := models.SchemaField{Type: models.FieldNumber, ImpliedDecimals: 0}
	if !validateFieldValue("500.5", field) {
		t.Error("sin ImpliedDecimals configurado, un valor con punto decimal sigue siendo válido (comportamiento actual sin cambios)")
	}
}
```

- [ ] **Step 2: Actualizar los 8 call-sites existentes de `validateFieldValue` en los tests**

En `backend/internal/services/ingestion_service_test.go`, hacer estos 8 reemplazos EXACTOS (buscar cada línea, reemplazar tal cual — son las únicas 8 llamadas directas a `validateFieldValue` en el archivo, dentro de las funciones `TestValidateFieldValue_*` ya existentes de la Tarea 2 del plan de la bitácora):

| Buscar | Reemplazar por |
|---|---|
| `if !validateFieldValue("1234.56", models.FieldNumber) {` | `if !validateFieldValue("1234.56", models.SchemaField{Type: models.FieldNumber}) {` |
| `if validateFieldValue("n/a", models.FieldNumber) {` | `if validateFieldValue("n/a", models.SchemaField{Type: models.FieldNumber}) {` |
| `if !validateFieldValue("2026-09-08", models.FieldDate) {` | `if !validateFieldValue("2026-09-08", models.SchemaField{Type: models.FieldDate}) {` |
| `if validateFieldValue("no es una fecha", models.FieldDate) {` | `if validateFieldValue("no es una fecha", models.SchemaField{Type: models.FieldDate}) {` |
| `if !validateFieldValue("true", models.FieldBoolean) {` | `if !validateFieldValue("true", models.SchemaField{Type: models.FieldBoolean}) {` |
| `if validateFieldValue("tal vez", models.FieldBoolean) {` | `if validateFieldValue("tal vez", models.SchemaField{Type: models.FieldBoolean}) {` |
| `if !validateFieldValue("cualquier texto 123", models.FieldString) {` | `if !validateFieldValue("cualquier texto 123", models.SchemaField{Type: models.FieldString}) {` |
| `if !validateFieldValue("", models.FieldNumber) {` | `if !validateFieldValue("", models.SchemaField{Type: models.FieldNumber}) {` |

No tocar ningún otro test (en particular, los tests de `firstInvalidField` y de `compareSchema` no llaman a `validateFieldValue` directamente, no necesitan cambios).

- [ ] **Step 3: Cambiar la firma de `validateFieldValue` y actualizar `firstInvalidField`**

Buscar:

```go
func validateFieldValue(raw string, expected models.FieldType) bool {
	if raw == "" {
		return true
	}
	switch expected {
	case models.FieldNumber:
		_, err := strconv.ParseFloat(raw, 64)
		return err == nil
	case models.FieldBoolean:
		_, err := strconv.ParseBool(raw)
		return err == nil
	case models.FieldDate:
		if _, err := time.Parse("2006-01-02", raw); err == nil {
			return true
		}
		_, err := time.Parse(time.RFC3339, raw)
		return err == nil
	default:
		return true
	}
}
```

Reemplazar por:

```go
func validateFieldValue(raw string, expected models.SchemaField) bool {
	if raw == "" {
		return true
	}
	switch expected.Type {
	case models.FieldNumber:
		if expected.ImpliedDecimals > 0 && strings.Contains(raw, ".") {
			return false
		}
		_, err := strconv.ParseFloat(raw, 64)
		return err == nil
	case models.FieldBoolean:
		_, err := strconv.ParseBool(raw)
		return err == nil
	case models.FieldDate:
		if _, err := time.Parse("2006-01-02", raw); err == nil {
			return true
		}
		_, err := time.Parse(time.RFC3339, raw)
		return err == nil
	default:
		return true
	}
}
```

`"strings"` ya está importado en `ingestion_service.go` (lo usa `sanitizeFieldName`/`dedupeFieldNames`, código preexistente) — no hace falta agregarlo.

Buscar, dentro de `func firstInvalidField(...)`:

```go
				if !validateFieldValue(row[i], f.Type) {
```

Reemplazar por:

```go
				if !validateFieldValue(row[i], f) {
```

- [ ] **Step 4: Correr los tests, confirmar que todo pasa**

Run: `cd backend && go test ./internal/services/... -v`
Expected: PASS completo — los 2 tests nuevos, los 8 tests actualizados de `validateFieldValue`, y el resto de la suite (incluidos `TestFirstInvalidField_*`, `TestCompareSchema_*`, y los tests de `parseValue` de la Tarea 1) sin regresiones.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/ingestion_service.go backend/internal/services/ingestion_service_test.go
git commit -m "feat(decimales): rechazar filas con punto decimal cuando hay decimales implícitos configurados"
```

---

### Task 3: Endpoint `PUT /monitors/:id/schema`

**Files:**
- Modify: `backend/internal/handlers/monitor_handler.go`
- Modify: `backend/internal/router/router.go`

**Interfaces:**
- Consumes: `models.SchemaField` (Tarea 1), `h.monitorRepo.FindByID`/`Update` (ya existentes, usados por otros handlers del mismo archivo).
- Produces: `PUT /monitors/:id/schema` — Tarea 4 (frontend) lo consume.

Sin TDD — plomería, mismo criterio que el resto de los endpoints de este plan y del de la bitácora (sin tests HTTP de handlers en este proyecto, salvo funciones puras extraídas).

- [ ] **Step 1: Agregar el handler `UpdateSchema`**

Agregar a `backend/internal/handlers/monitor_handler.go`, después de cualquier handler existente relacionado a monitores (ej. después de `Update`):

```go
// UpdateSchema reemplaza el schema del monitor, permitiendo configurar
// por campo (hoy solo ImpliedDecimals) sin cambiar el conjunto de
// campos — el schema recibido debe tener exactamente los mismos
// nombres que el actual, o se rechaza. Esto evita que un bug de
// frontend agregue/quite campos o cambie Name/Type por esta vía.
func (h *MonitorHandler) UpdateSchema(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}

	monitor, err := h.monitorRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
	}

	var body struct {
		Schema []models.SchemaField `json:"schema"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	currentNames := make(map[string]bool, len(monitor.Schema))
	for _, f := range monitor.Schema {
		currentNames[f.Name] = true
	}
	if len(body.Schema) != len(currentNames) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "el schema enviado debe tener los mismos campos que el actual"})
	}
	for _, f := range body.Schema {
		if !currentNames[f.Name] {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "el schema enviado debe tener los mismos campos que el actual"})
		}
	}

	monitor.Schema = body.Schema
	if err := h.monitorRepo.Update(c.Context(), monitor.ID, bson.M{"schema": monitor.Schema}); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"schema": monitor.Schema})
}
```

- [ ] **Step 2: Registrar la ruta**

En `backend/internal/router/router.go`, agregar junto a las otras rutas de `/monitors/:id/...` (ej. después de la línea de `monitors.Put("/:id", ...)`):

```go
	monitors.Put("/:id/schema", middleware.RequireComplianceOrAbove(), h.Monitor.UpdateSchema)
```

- [ ] **Step 3: Verificar que compila**

Run: `cd backend && go build ./...`
Expected: sin errores.

- [ ] **Step 4: Verificación manual**

Con un backend local corriendo contra Mongo real (ver notas de verificación de tareas anteriores de este roadmap para levantar el entorno si hace falta): elegir un monitor con `schema` no vacío, hacer `curl -X PUT http://localhost:8080/api/v1/monitors/<id>/schema -H "Authorization: Bearer <token-compliance>" -H "Content-Type: application/json" -d '{"schema": [...]}'` con el mismo array de campos pero un `impliedDecimals` distinto en uno de ellos — confirmar `200` con el schema actualizado, y que `GET /monitors/<id>` refleja el cambio. Probar también enviar un schema con un campo de menos → confirmar `400`.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handlers/monitor_handler.go backend/internal/router/router.go
git commit -m "feat(decimales): endpoint PUT /monitors/:id/schema"
```

---

### Task 4: Cliente API y tipos del frontend

**Files:**
- Modify: `frontend/src/lib/types.ts`
- Modify: `frontend/src/lib/api/monitors.ts`

**Interfaces:**
- Produces: `SchemaField.impliedDecimals?: number`, `monitorsApi.updateSchema(id, schema)` — Tarea 5 lo consume.

Sin tests — mismo criterio que el resto de `lib/api/*.ts`.

- [ ] **Step 1: Agregar el campo al tipo `SchemaField`**

En `frontend/src/lib/types.ts`, buscar:

```ts
export interface SchemaField {
  name: string;
  type: "string" | "number" | "date" | "boolean";
  required: boolean;
  sample: string;
}
```

Reemplazar por:

```ts
export interface SchemaField {
  name: string;
  type: "string" | "number" | "date" | "boolean";
  required: boolean;
  sample: string;
  impliedDecimals?: number;
}
```

- [ ] **Step 2: Agregar el método al cliente**

En `frontend/src/lib/api/monitors.ts`, agregar al objeto `monitorsApi`, después de `update`:

```ts
  updateSchema: (id: string, schema: SchemaField[]) =>
    api.put<{ schema: SchemaField[] }>(`/monitors/${id}/schema`, { schema }),
```

`SchemaField` ya está importado en este archivo (lo usa `create`/`detectSchema`) — no hace falta agregar el import.

- [ ] **Step 3: Verificar que compila**

Run: `cd frontend && npx tsc --noEmit`
Expected: sin errores.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/types.ts frontend/src/lib/api/monitors.ts
git commit -m "feat(decimales): cliente API del frontend"
```

---

### Task 5: Edición de decimales implícitos en la pestaña "Esquema"

**Files:**
- Modify: `frontend/src/components/tabs/content/monitor-tab-content.tsx`

**Interfaces:**
- Consumes: `monitorsApi.updateSchema` (Tarea 4), `SchemaField.impliedDecimals` (Tarea 4), `Select`/`SelectContent`/`SelectItem`/`SelectTrigger`/`SelectValue` (shadcn, ya usados en otras páginas del proyecto — ej. `uploads/page.tsx`), `useToast` (ya importado en este archivo).

Sin tests (componente React).

- [ ] **Step 1: Agregar el import de `Select`**

Agregar al bloque de imports de `frontend/src/components/tabs/content/monitor-tab-content.tsx`:

```tsx
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
```

- [ ] **Step 2: Agregar `toastSuccess` a la destructuración existente de `useToast`**

Buscar:

```tsx
  const { toastError } = useToast();
```

Reemplazar por:

```tsx
  const { toastError, toastSuccess } = useToast();
```

- [ ] **Step 3: Agregar estado de edición del schema**

Cerca de la declaración de `const [monitor, setMonitor] = useState<Monitor | null>(null);`, agregar:

```tsx
  const [schemaEdits, setSchemaEdits] = useState<SchemaField[]>([]);
  const [savingSchema, setSavingSchema] = useState(false);
```

- [ ] **Step 4: Sincronizar `schemaEdits` cuando cambia `monitor.schema`**

Agregar, junto a los demás `useEffect` del componente:

```tsx
  useEffect(() => {
    if (monitor?.schema) {
      setSchemaEdits(monitor.schema);
    }
  }, [monitor?.schema]);
```

- [ ] **Step 5: Agregar el handler de edición y de guardado**

Agregar cerca de las demás funciones del componente (ej. junto a `loadMonitor`):

```tsx
  function updateImpliedDecimals(fieldName: string, value: number) {
    setSchemaEdits((prev) =>
      prev.map((f) => (f.name === fieldName ? { ...f, impliedDecimals: value } : f)),
    );
  }

  async function handleSaveSchema() {
    setSavingSchema(true);
    try {
      const result = await monitorsApi.updateSchema(id, schemaEdits);
      setMonitor((prev) => (prev ? { ...prev, schema: result.schema } : prev));
      toastSuccess("Esquema actualizado");
    } catch {
      toastError("No se pudo guardar el esquema");
    } finally {
      setSavingSchema(false);
    }
  }
```

- [ ] **Step 6: Reescribir la pestaña "Esquema" con el control editable**

Buscar el bloque completo:

```tsx
          <TabsContent value="schema">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Esquema detectado</CardTitle>
              </CardHeader>
              <CardContent>
                {!monitor.schema || monitor.schema.length === 0 ? (
                  <p className="text-sm text-muted-foreground">
                    Carga datos para detectar el esquema automaticamente
                  </p>
                ) : (
                  <div className="space-y-2">
                    {monitor.schema.map((field: SchemaField) => (
                      <div
                        key={field.name}
                        className="flex items-center justify-between rounded-md border p-3"
                      >
                        <div>
                          <p className="text-sm font-medium">{field.name}</p>
                          {field.sample && (
                            <p className="text-xs text-muted-foreground">
                              Ejemplo: {field.sample}
                            </p>
                          )}
                        </div>
                        <Badge variant="secondary">{field.type}</Badge>
                      </div>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>
```

Reemplazar por:

```tsx
          <TabsContent value="schema">
            <Card>
              <CardHeader className="flex flex-row items-center justify-between">
                <CardTitle className="text-base">Esquema detectado</CardTitle>
                {monitor.schema && monitor.schema.length > 0 && user?.role !== "viewer" && (
                  <Button size="sm" onClick={handleSaveSchema} disabled={savingSchema}>
                    {savingSchema ? "Guardando..." : "Guardar"}
                  </Button>
                )}
              </CardHeader>
              <CardContent>
                {!monitor.schema || monitor.schema.length === 0 ? (
                  <p className="text-sm text-muted-foreground">
                    Carga datos para detectar el esquema automaticamente
                  </p>
                ) : (
                  <div className="space-y-2">
                    {schemaEdits.map((field: SchemaField) => (
                      <div
                        key={field.name}
                        className="flex items-center justify-between rounded-md border p-3"
                      >
                        <div>
                          <p className="text-sm font-medium">{field.name}</p>
                          {field.sample && (
                            <p className="text-xs text-muted-foreground">
                              Ejemplo: {field.sample}
                            </p>
                          )}
                        </div>
                        <div className="flex items-center gap-2">
                          {field.type === "number" && user?.role !== "viewer" && (
                            <Select
                              value={String(field.impliedDecimals ?? 0)}
                              onValueChange={(v) => updateImpliedDecimals(field.name, Number(v))}
                            >
                              <SelectTrigger className="w-[180px]">
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                <SelectItem value="0">Sin decimales implícitos</SelectItem>
                                <SelectItem value="1">1 decimal implícito</SelectItem>
                                <SelectItem value="2">2 decimales implícitos</SelectItem>
                                <SelectItem value="3">3 decimales implícitos</SelectItem>
                                <SelectItem value="4">4 decimales implícitos</SelectItem>
                              </SelectContent>
                            </Select>
                          )}
                          <Badge variant="secondary">{field.type}</Badge>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>
```

`user` (de `useAuthStore`) y `Button` ya están importados/declarados en este archivo (usados en otras partes del componente) — no hace falta agregarlos.

- [ ] **Step 7: Verificar que compila**

Run: `cd frontend && npx tsc --noEmit`
Expected: sin errores.

- [ ] **Step 8: Verificar lint**

Run: `cd frontend && npm run lint`
Expected: 0 errores nuevos (warnings preexistentes sin cambios).

- [ ] **Step 9: Verificación manual**

Levantar el dev server, navegar al detalle de un monitor con schema ya establecido, ir a la pestaña "Esquema" → confirmar que los campos `number` muestran el selector de decimales implícitos, que los campos de otro tipo NO lo muestran, que cambiar el valor y hacer click en "Guardar" persiste (recargar la página y confirmar que el valor elegido sigue ahí), y que un usuario `viewer` ve el esquema pero sin el selector editable ni el botón "Guardar".

Subir después un archivo con un valor entero a ese campo (ej. `500000` con 2 decimales implícitos configurados) y confirmar en la pestaña "Datos" que el valor guardado es `5000` (o `5000.00` según cómo se muestre), no `500000`.

- [ ] **Step 10: Commit**

```bash
git add frontend/src/components/tabs/content/monitor-tab-content.tsx
git commit -m "feat(decimales): editar decimales implícitos por campo en la pestaña Esquema"
```

---

## Criterio de cierre del lote

- Gate (`go build ./...`, `go test ./...`, `npx tsc --noEmit`, `npm run lint`, `npm run build`) sin bloqueantes en las 5 tareas.
- Recorrido manual:
  - Configurar 2 decimales implícitos en un campo `monto`, subir un archivo con valores enteros → los valores guardados quedan divididos correctamente.
  - Subir al mismo monitor un archivo donde ese campo ya trae punto decimal → esa fila se rechaza (queda reflejado en la bitácora de cargas, si ese plan ya está mergeado).
  - Dejar un campo sin decimales implícitos configurados (valor 0) → comportamiento idéntico al actual, sin cambios.
  - Confirmar que un usuario `viewer` no puede editar el esquema.
