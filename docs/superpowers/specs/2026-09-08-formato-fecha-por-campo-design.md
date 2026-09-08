# Formato de Fecha Configurable por Campo — Diseño

**Fecha:** 2026-09-08
**Estado:** Aprobado para pasar a plan de implementación (diseño validado en chat, sección por sección)

## Objetivo

La detección y validación de campos `date` hoy solo reconoce dos formatos fijos: ISO
(`YYYY-MM-DD`) y RFC3339. Cualquier archivo de origen con fechas en otro formato común
(`DD/MM/YYYY`, `MM/DD/YYYY`, `DD-MM-YYYY`, `YYYY/MM/DD`) no se detecta como fecha en la
carga inicial, y si el campo ya quedó tipado como `date` en el esquema, esas filas se
rechazan por completo. Este feature permite configurar, por campo `date` de un monitor, en
qué formato vienen sus valores, para que la validación e ingesta lo acepten en lugar de
rechazarlo.

Mismo patrón que `ImpliedDecimals` (`docs/superpowers/specs/2026-09-08-decimales-implicitos-design.md`):
un atributo opcional por campo, editable desde la pestaña "Esquema", que cambia cómo se
valida y parsea cada carga posterior.

## Fuera de alcance (explícito)

- **Detección automática del formato.** La detección de esquema en la primera carga
  (`inferType`/`looksLikeDate`) no cambia — sigue reconociendo solo ISO/RFC3339 para
  clasificar un campo como `date`. `DateFormat` es una configuración que el usuario fija
  DESPUÉS, en la pestaña "Esquema", igual que `ImpliedDecimals` — no hay heurística que
  adivine el formato.
- **Fechas seriales de Excel.** `IngestExcel` usa `f.GetRows()`, que devuelve el valor ya
  formateado según el formato de celda de la hoja de origen — un problema distinto (de
  lectura de Excel, no de validación de formato) que queda como gap conocido para un plan
  aparte.
- **Normalización de zona horaria.** Sin cambios al manejo de offsets de RFC3339.
- **Formato libre (layout de Go).** Se ofrece un preset fijo de 4 formatos comunes, no un
  campo de texto libre con sintaxis de `time.Parse` — evita exponer sintaxis interna de Go
  al usuario y evita layouts inválidos.
- **Recalcular datos ya ingeridos.** La configuración aplica desde el próximo upload en
  adelante — mismo criterio que `ImpliedDecimals`.
- **Validar que `DateFormat` solo se configure en campos `Type == FieldDate`.** Igual que
  `ImpliedDecimals` en campos que no son `number`: si se configura en un campo de otro tipo,
  el valor se ignora (no se valida como error, simplemente no tiene efecto) — la UI de todos
  modos solo lo expone en campos `date`.

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
    DateFormat      string    `bson:"date_format,omitempty" json:"dateFormat,omitempty"`
}
```

`""` (zero-value, lo que ya tienen todos los campos existentes por default) significa "sin
formato configurado" — el comportamiento actual, sin cambios: se acepta ISO o RFC3339. Un
valor no vacío debe ser una de las claves de preset reconocidas (ver abajo); no se guarda
un layout de Go crudo.

### Presets reconocidos

Nueva tabla **exportada** en `ingestion_service.go`, junto a `looksLikeDate` (exportada
porque `UpdateSchema`, en el paquete `handlers`, necesita validar contra ella — ver
"Endpoint de configuración"):

```go
var DateFormatPresets = map[string]string{
    "DD/MM/YYYY": "02/01/2006",
    "MM/DD/YYYY": "01/02/2006",
    "DD-MM-YYYY": "02-01-2006",
    "YYYY/MM/DD": "2006/01/02",
}
```

La clave (`"DD/MM/YYYY"`, etc.) es lo que se guarda en `SchemaField.DateFormat`, viaja por
la API, y se muestra en el `Select` del frontend — un solo vocabulario entre backend y
frontend, sin traducción intermedia.

### Comportamiento — estricto, no aditivo

Si un campo tiene `DateFormat` configurado, **solo** ese formato es válido — deja de
aceptar ISO/RFC3339 para ese campo. Mismo criterio que `ImpliedDecimals`: configurar 2
decimales implícitos rechaza valores con punto en vez de tolerarlos "además". Si un campo
debe seguir aceptando ISO/RFC3339 tal como hoy, simplemente no se le configura
`DateFormat`. Evita la ambigüedad de qué formato tiene prioridad cuando dos son válidos
para el mismo string.

### Endpoint de configuración

Reusa `PUT /monitors/:id/schema` (ya existe, agregado por el plan de decimales
implícitos) — no hace falta un endpoint nuevo, el body ya es el array completo de
`SchemaField`. El handler (`UpdateSchema`, `backend/internal/handlers/monitor_handler.go`)
gana una validación adicional: para cada campo del body con `DateFormat != ""`, esa clave
debe existir en `services.DateFormatPresets`; si no, `400` con mensaje explícito ("formato
de fecha inválido: <valor>"). La validación de nombres de campo ya existente (el schema
recibido debe tener los mismos nombres que el actual) no cambia.

### Parseo (`parseValue`, `ingestion_service.go`)

En el `case models.FieldDate`: si `field.DateFormat != ""`, resolver el layout de Go vía
`DateFormatPresets[field.DateFormat]` y parsear exclusivamente con ese layout. Si
`field.DateFormat == ""`, mantener la cadena actual (ISO, luego RFC3339).

### Validación por fila (`validateFieldValue`, `ingestion_service.go`)

En el `case models.FieldDate`: mismo criterio — si `expected.DateFormat != ""`, validar
exclusivamente contra `DateFormatPresets[expected.DateFormat]`; si `expected.DateFormat ==
""`, mantener la validación actual (ISO o RFC3339). `firstInvalidField` no distingue POR
QUÉ `validateFieldValue` devolvió `false` — para cualquier campo `date` inválido (sin
importar si el problema es que no es fecha en absoluto o que no matchea el `DateFormat`
configurado) ya arma el mismo mensaje genérico existente, sin cambios:
`` `valor "<raw>" no es del tipo date` `` (mismo mecanismo ya verificado en producción para
`ImpliedDecimals`, que en la bitácora de cargas se ve como `valor "150.50" no es del tipo
number`). No hace falta tocar `firstInvalidField` — el mensaje diferenciado por causa que
mencionaba una versión anterior de este documento no existe en el código real y no se
agrega acá.

### Frontend — edición en la pestaña "Esquema"

`frontend/src/components/tabs/content/monitor-tab-content.tsx`: mismo patrón que el
`Select` de `impliedDecimals` (línea ~790), pero condicionado a `field.type === "date"` en
vez de `"number"`:

```tsx
{field.type === "date" && user?.role !== "viewer" && (
  <Select
    value={field.dateFormat ?? "auto"}
    onValueChange={(v) => updateDateFormat(field.name, v === "auto" ? "" : v)}
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

Nueva función `updateDateFormat(fieldName, value)`, mismo cuerpo que
`updateImpliedDecimals` (línea ~181) pero escribiendo `dateFormat` en vez de
`impliedDecimals` sobre `schemaEdits`. El botón "Guardar" y `handleSaveSchema` ya existentes
no cambian — siguen mandando el array completo de `schemaEdits`, ahora con `dateFormat`
incluido cuando aplica. `frontend/src/lib/types.ts` gana `dateFormat?: string` en
`SchemaField`, junto al `impliedDecimals?: number` existente. `monitorsApi.updateSchema` no
cambia de firma (ya manda el array completo).

## Manejo de errores

- `PUT /monitors/:id/schema` con un `DateFormat` que no está en `DateFormatPresets`: `400`,
  mensaje explícito con el valor recibido.
- Fila con un valor que no matchea el `DateFormat` configurado: se rechaza como fila
  individual (mecanismo ya existente de `firstInvalidField`/bitácora de cargas), no rompe
  el resto del archivo ni el upload completo.

## Testing

- `validateFieldValue` extendido: TDD —
  - un valor en el preset configurado (ej. `"08/09/2026"` con `DateFormat="DD/MM/YYYY"`) es
    válido;
  - el mismo valor en formato ISO (`"2026-09-08"`) es **inválido** cuando el campo tiene un
    preset custom configurado — este es el caso que documenta el comportamiento estricto;
  - un campo sin `DateFormat` sigue aceptando ISO/RFC3339 como hoy (regresión).
- `parseValue` extendido: TDD — con `DateFormat="DD/MM/YYYY"`, `"08/09/2026"` parsea al
  `time.Time` correcto (8 de septiembre, no 9 de agosto — confirma que no se está usando el
  layout equivocado).
- `UpdateSchema` (handler): TDD — un `DateFormat` fuera de `DateFormatPresets` devuelve
  `400`; un `DateFormat` válido se guarda y se refleja en la respuesta.
- Sin tests de UI/componentes — mismo criterio que decimales implícitos y que el resto del
  proyecto.
