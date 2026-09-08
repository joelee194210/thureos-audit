# Decimales Implícitos por Campo — Diseño

**Fecha:** 2026-09-08
**Estado:** Aprobado para pasar a plan de implementación (diseño validado en chat, sección por sección)

## Objetivo

Algunos archivos de origen (típico en exports bancarios/POS) codifican montos como enteros sin punto decimal, donde los últimos N dígitos son en realidad la parte decimal — ej. `500000` significa `5000.00` (2 decimales implícitos), `49999` significa `499.99`. Hoy, cualquier valor así se ingiere literal (`500000` queda como `500000`, no como `5000.00`), sin forma de corregirlo. Este feature permite configurar, por campo numérico de un monitor, cuántos decimales implícitos tiene, para que la ingesta divida el valor crudo por `10^N` antes de guardarlo.

## Fuera de alcance (explícito)

- **Recalcular datos ya ingeridos.** La configuración aplica desde el próximo upload en adelante — los documentos ya guardados con la interpretación anterior quedan como están. Mismo criterio ya usado para los monitores con estructura mezclada (spec de la bitácora de cargas, "CBCG Tarjetas").
- **Editor de esquema más amplio.** Solo se agrega edición del campo `ImpliedDecimals` a la pestaña "Esquema" (hoy de solo lectura) — el tipo de campo (`string`/`number`/`date`/`boolean`), el nombre, o cualquier otra propiedad de `SchemaField` siguen sin ser editables desde la UI.
- **Configuración vía la creación del monitor.** El toggle se configura DESPUÉS de que el monitor ya tiene un schema detectado (por eso vive en la pestaña "Esquema" de la página de detalle) — no se agrega al formulario de creación de monitor.

## Dependencia con el plan de la bitácora de cargas

Este feature reusa `firstInvalidField`/`validateFieldValue` (Tarea 2 del plan `docs/superpowers/plans/2026-09-08-bitacora-cargas.md`) para el rechazo de filas con valores que ya traen punto decimal, y modifica `parseValue` — ambos en `backend/internal/services/ingestion_service.go`, el mismo archivo que ese plan reescribe extensamente en sus Tareas 1, 2 y 4. **La implementación de este plan debe esperar a que el plan de la bitácora de cargas termine su ejecución** (Tareas 8-9 y review final) antes de despachar cualquier tarea que toque ese archivo — dos procesos de subagent-driven-development escribiendo el mismo archivo en paralelo generarían conflictos de merge y revisiones sobre una base movediza.

## Arquitectura

### Modelo de datos

`models.SchemaField` (`backend/internal/models/monitor.go`) gana un campo nuevo:

```go
type SchemaField struct {
    Name            string    `bson:"name" json:"name"`
    Type            FieldType `bson:"type" json:"type"`
    Required        bool      `bson:"required" json:"required"`
    Sample          string    `bson:"sample" json:"sample"`
    ImpliedDecimals int       `bson:"implied_decimals,omitempty" json:"impliedDecimals,omitempty"`
}
```

`0` (el zero-value de Go, y lo que ya tienen todos los campos existentes por default) significa "sin decimales implícitos, valor literal" — el comportamiento actual, sin cambios, para cualquier campo que nunca se configure. Solo aplica semánticamente a campos `Type == FieldNumber`; en campos de otro tipo el valor se ignora (no se valida como error, simplemente no tiene efecto).

### Endpoint de configuración

`PUT /monitors/:id/schema` (nuevo), protegido con `RequireComplianceOrAbove()` (mismo criterio que el resto de acciones de configuración de un monitor — crear, editar, subir). Body: `{"schema": [...]}`, el array completo de `SchemaField` con los `ImpliedDecimals` ya editados. El handler reemplaza `monitor.Schema` completo (no un PATCH parcial por campo — más simple, y el frontend ya tiene el array completo cargado desde `GET /monitors/:id`, así que reenviarlo completo no cuesta nada extra). Valida que los nombres de campo del array recibido coincidan exactamente con los del schema actual (incluye protección: no se puede agregar ni quitar campos por esta vía, solo editar `ImpliedDecimals` — y de paso deja la puerta cerrada a que alguien cambie `Type`/`Name` por un bug de frontend, ya que solo se acepta el schema si el conjunto de nombres coincide).

### Parseo (`parseValue`, `ingestion_service.go`)

Cuando `field.Type == FieldNumber` y `field.ImpliedDecimals > 0`: después de `strconv.ParseFloat(value, 64)` exitoso, dividir por `math.Pow(10, float64(field.ImpliedDecimals))` antes de devolver el resultado. Sin cambios para `ImpliedDecimals == 0` (todos los campos existentes, comportamiento actual intacto).

### Validación por fila (reusa `firstInvalidField`, `validateFieldValue`)

`validateFieldValue` gana una validación adicional: si `expected.Type == FieldNumber` y `expected.ImpliedDecimals > 0` y `raw` contiene un `.` (punto decimal literal en el valor crudo), el valor es inválido — la fila se rechaza con una razón explícita ("el campo tiene decimales implícitos configurados, pero el valor ya trae punto decimal"). Esto requiere cambiar la firma de `validateFieldValue`/`firstInvalidField` de recibir `models.FieldType` a recibir el `models.SchemaField` completo (para tener acceso a `ImpliedDecimals`) — cambio de firma que toca los call-sites ya existentes en `ingestDelimited`/`IngestExcel` (plan de la bitácora, Tarea 4).

### Frontend — edición en la pestaña "Esquema"

`frontend/src/components/tabs/content/monitor-tab-content.tsx`, dentro de `TabsContent value="schema"`: cada fila de campo con `Type === "number"` gana un `Select` (0-4) para `impliedDecimals`, inicializado con el valor actual. Un botón "Guardar" fijo (siempre visible en la pestaña, no condicionado a que haya cambios pendientes — más simple de implementar y de razonar) llama `monitorsApi.updateSchema(id, schema)` (nuevo método de `monitorsApi`, `PUT /monitors/:id/schema`). Mismo gating de rol que otras acciones de edición del monitor (`user.role !== "viewer"`).

## Manejo de errores

- `PUT /monitors/:id/schema` con un array cuyos nombres de campo no coinciden con el schema actual: `400`, mensaje explícito.
- Fila con valor ya-punto-decimal en un campo con `ImpliedDecimals` configurado: se rechaza como fila individual (mecanismo ya existente de la bitácora de cargas), no rompe el resto del archivo ni el upload completo.

## Testing

- `parseValue` con `ImpliedDecimals`: TDD — valor entero se divide correctamente (500000 → 5000, con 2 decimales), `ImpliedDecimals=0` no cambia el comportamiento actual (regresión), decimales en el borde (0 dígitos después de dividir, ej. "5" con 2 decimales → 0.05).
- `validateFieldValue` extendido: TDD — valor con punto decimal en un campo con `ImpliedDecimals>0` es inválido; el mismo valor en un campo con `ImpliedDecimals=0` sigue siendo válido (comportamiento actual sin cambios).
- Sin tests de UI/componentes — mismo criterio que el resto del proyecto.
