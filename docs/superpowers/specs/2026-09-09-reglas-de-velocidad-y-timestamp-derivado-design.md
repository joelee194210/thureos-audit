# Reglas de Velocidad y Timestamp Derivado — Diseño

**Fecha:** 2026-09-09
**Estado:** Aprobado para pasar a plan de implementación (diseño validado en chat)

## Objetivo

Hacer expresable y evaluable este caso real, hoy imposible:

> Transacciones de más de USD 5.000, de categoría casino (MCC 7995), donde entre una y
> otra pasen menos de 35 segundos, en la misma tarjeta.

Cuatro bloqueos independientes lo impiden. Este documento los cubre a los cuatro, ordenados
por dependencia (el plan de implementación los ejecuta en ese orden, no todos a la vez).

## Contexto: qué se arregló ya y qué falta

Un hotfix previo (commits `8648867` y `85c8a2a`, ya en producción) corrigió el bug que hacía
que **ninguna** regla con ventana temporal disparara: el cutoff viajaba a Mongo como string
formateado y Mongo no compara entre tipos BSON distintos, así que el `$match` devolvía cero
documentos. Ese arreglo es prerrequisito de todo lo que sigue, pero no alcanza por sí solo.

Lo que sigue faltando, verificado contra producción:

1. **No hay timestamp.** El monitor real `CBCG Tarjetas` guarda fecha y hora como dos campos
   numéricos separados: `fechapoliza: 20260907` (YYYYMMDD) y `horapoliza: 21517` — que es
   `02:15:17`, con el cero inicial perdido al almacenarse como número. Ningún campo de tipo
   `date` existe, así que no hay sobre qué calcular diferencias de tiempo.
2. **El modelo no expresa "gap entre consecutivas".** `AggregateCondition`
   (`backend/internal/models/rule.go:73-85`) solo expresa "agregado dentro de una ventana
   móvil anclada a ahora" (N eventos en 35s), no "menos de 35s entre un evento y el
   siguiente".
3. **La UI no ofrece ventanas por debajo de 24h** (`frontend/src/app/(dashboard)/rules/page.tsx:1624-1631`
   y `:2576-2583`: solo 24h, 7d, 15d, 30d, 90d, 365d) ni ningún editor para `Filter`, aunque
   el backend ya lo soporta.
4. **La IA no puede generar el caso.** El prompt (`ai_rules_service.go:34-85`) nunca menciona
   `Filter`, y `filterValidSuggestions` (`:313-328`) descarta en silencio toda sugerencia
   cuyo `groupBy` o `timeField` venga vacío — incluso cuando un agregado global es válido
   para el motor.

## Decisiones tomadas

- **Semántica de la alerta:** un solo par basta. Dos transacciones consecutivas de la misma
  entidad, ambas pasando el filtro, separadas por menos del gap configurado → alerta.
- **Agrupación configurable**, con el campo de tarjeta como valor por defecto para este
  monitor.
- **Timestamp materializado, no calculado al vuelo:** se configura una vez por monitor, la
  ingesta lo escribe como fecha real en cada carga nueva, y un backfill de un solo uso lo
  calcula para los datos ya guardados. Queda disponible para reglas, chat, dashboards y
  filtros por igual, en vez de que cada consumidor repita la lógica.
- **Modo guiado de IA aditivo:** se agrega un modo "elegí los campos y describí el criterio";
  el texto libre actual no se elimina.

## Fuera de alcance (explícito)

- **Catálogo MCC dentro del motor de reglas.** "Casino" se expresa como una condición sobre
  el campo del propio monitor (`mcc = 7995`), no vía `$lookup` contra la colección `mcc`. El
  catálogo existe (`backend/internal/seed/mcc_data.go:102-106`, MCC 7995 → "Casinos y Juegos",
  riesgo alto) pero conectarlo al motor es un plan aparte.
- **Recalcular alertas históricas.** Las reglas nuevas evalúan de acá en adelante.
- **Detección automática del timestamp.** El usuario configura explícitamente qué campo es la
  fecha y cuál la hora; no hay heurística que lo adivine.

## Arquitectura

### Pieza 1 — Timestamp derivado por monitor

`models.Monitor` (`backend/internal/models/monitor.go`) gana un campo opcional:

```go
type Monitor struct {
    // ... campos existentes sin cambios ...
    DerivedTimestamp *DerivedTimestampConfig `bson:"derived_timestamp,omitempty" json:"derivedTimestamp,omitempty"`
}
```

con esta configuración:

```go
// DerivedTimestamp construye un campo de fecha real a partir de dos columnas
// numéricas separadas (patrón habitual en exports bancarios: fecha y hora en
// campos distintos). Nil = el monitor no tiene timestamp derivado, todo se
// comporta exactamente como hoy.
type DerivedTimestampConfig struct {
    DateField  string `bson:"date_field" json:"dateField"`   // ej. "fechapoliza"
    DateFormat string `bson:"date_format" json:"dateFormat"` // "YYYYMMDD"
    TimeField  string `bson:"time_field" json:"timeField"`   // ej. "horapoliza"
    TimeFormat string `bson:"time_format" json:"timeFormat"` // "HHMMSS" | "HHMM"
    TargetName string `bson:"target_name" json:"targetName"` // ej. "timestamp"
}
```

**El relleno de ceros es obligatorio, no opcional.** `horapoliza` se guarda como número, así
que `021517` llega como `21517`: hay que rellenar a la izquierda hasta el ancho del formato
(6 dígitos para `HHMMSS`, 4 para `HHMM`) antes de parsear. Sin eso, `21517` se leería como
`21:51:7` en vez de `02:15:17` — un error silencioso de 19 horas. Lo mismo aplica al campo de
fecha por consistencia.

**Materialización en la ingesta:** en el bucle que arma cada documento
(`ingestDelimited`, `IngestExcel`, `IngestJSON` en `backend/internal/services/ingestion_service.go`),
después de parsear los campos, se computa el timestamp y se escribe como `time.Time` (fecha
BSON) bajo `TargetName`. Si alguno de los dos campos falta o no parsea, el documento se
ingiere igual pero **sin** el campo derivado — no se rechaza la fila: un timestamp
inconstruible es un dato incompleto, no un dato inválido.

**Exposición en el esquema:** al guardar la configuración, el campo derivado se agrega al
`Schema` del monitor como un `SchemaField` de tipo `date`. Así aparece automáticamente en los
selectores de campo de reglas, chat y dashboards, sin que ninguno de esos consumidores
necesite saber que es derivado.

**Backfill:** endpoint nuevo `POST /monitors/:id/backfill-timestamp`, protegido con
`RequireComplianceOrAbove()`. Recorre la colección de datos del monitor en lotes y escribe el
campo derivado en los documentos que no lo tengan. Devuelve cuántos actualizó y cuántos no
pudo construir. Es idempotente: correrlo dos veces no cambia nada la segunda vez.

### Pieza 2 — Condición de velocidad (gap entre consecutivas)

Tipo nuevo, hermano de `AggregateCondition`, no un reemplazo:

```go
// VelocityCondition detecta eventos consecutivos demasiado próximos en el
// tiempo dentro de una misma entidad (tarjeta, cliente, comercio).
// Es distinto de AggregateCondition: allí la ventana está anclada a "ahora"
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

`Rule` gana `VelocityConditions []VelocityCondition` (`bson:"velocity_conditions,omitempty"`),
siguiendo el mismo patrón aditivo que `AggregateConditions`: una regla sin condiciones de
velocidad se comporta exactamente como hoy.

**Evaluación** (MongoDB 7 en producción, así que `$setWindowFields` está disponible):

```
1. $match   → Filter (importe > 5000, mcc = 7995)
2. $sort    → { <groupBy>: 1, <timeField>: 1 }
3. $setWindowFields → partitionBy "$<groupBy>", sortBy { <timeField>: 1 },
                      output: { prevTime: { $shift: { output: "$<timeField>", by: -1 } } }
4. $addFields → gapSeconds: { $dateDiff: { startDate: "$prevTime",
                                           endDate: "$<timeField>", unit: "second" } }
5. $match   → { prevTime: { $ne: null }, gapSeconds: { $lte: <maxGap en segundos> } }
```

Cada documento que sobrevive el paso 5 **es** la segunda transacción de un par que viola el
umbral, y lleva `prevTime` y `gapSeconds` adjuntos — que es exactamente lo que hay que mostrar
en la bandera roja: las dos transacciones y cuánto tiempo pasó entre ellas.

**Validado contra producción (2026-09-09):** este pipeline se corrió tal cual contra el Mongo
real (7.0.40) sobre la colección de `CBCG Tarjetas`. `$setWindowFields`, `$shift` y
`$dateDiff` ejecutan correctamente: el primer documento de cada partición devuelve
`gapSeconds: null` (sin anterior, descartado por el `$ne: null`) y el resto devuelve el gap.
La prueba se hizo sobre `_ingested_at` por no haber todavía un timestamp real — y devolvió
`0` para todos los pares, porque los cuatro registros se ingirieron en el mismo lote. Eso
confirma de forma empírica por qué la Pieza 1 es prerrequisito de la Pieza 2: sin el
timestamp derivado del dato de origen, la condición de velocidad mide el momento de la carga,
no el de la transacción.

**`MinEvents > 2` — simplificación explícita.** Para exigir rachas más largas se agrega un
`$group` por partición contando gaps que califican, y se pide `count >= MinEvents - 1`. Esto
cuenta gaps que califican dentro de la partición, **sin exigir que sean consecutivos entre
sí**: tres pares rápidos separados por horas cumplirían un `MinEvents: 4`. Es una
aproximación deliberada — el caso elegido (`MinEvents: 2`, un par) es exacto, y la
aproximación solo afecta configuraciones que este diseño no prioriza. Queda documentado en el
código, no escondido.

**Rechazo de configuraciones sin sentido:** si `TimeField` está vacío, o el campo no es de
tipo `date` en el esquema del monitor, o `MaxGap` no parsea, la condición se rechaza al
guardar la regla (`400`), no en silencio al evaluar. Este es justamente el modo de falla que
nos costó caro con la ventana temporal.

### Pieza 3 — UI de reglas

`frontend/src/app/(dashboard)/rules/page.tsx`:

- **Ventanas cortas:** el selector de ventana (creación `:1624-1631` y edición `:2576-2583`)
  suma `30s`, `60s`, `5min`, `15min`, `1h`, `6h`, `12h` antes de las opciones actuales. El
  default sigue siendo `30d` — no se cambia el comportamiento de las reglas existentes.
- **Editor de `Filter`:** las condiciones agregadas ganan un bloque "Filtrar antes de agrupar"
  con el mismo editor de condiciones que ya usa `ConditionGroup`. Hoy `Filter` no tiene UI
  alguna pese a estar soportado y testeado en el backend.
- **Editor de condición de velocidad:** bloque nuevo con campo de tiempo (solo campos `date`
  del esquema), gap máximo (mismo set de unidades cortas), agrupar por, mínimo de eventos
  (default 2) y el mismo editor de `Filter`.

### Pieza 4 — Generación con IA

- **El prompt enseña lo que falta** (`ai_rules_service.go:34-85`): documentar `filter` dentro
  de `aggregateConditions` y el bloque nuevo `velocityConditions`, con un ejemplo explícito
  del caso de velocidad ("dos transacciones de la misma tarjeta a menos de 60s, ambas sobre
  cierto monto").
- **Dejar de descartar agregados globales** (`ai_rules_service.go:313-328`): un `groupBy`
  vacío es válido para el motor (`rule_engine.go:471-474`, agrupa globalmente); hoy se
  descarta la sugerencia entera. La validación pasa a exigir solo que los campos **presentes**
  existan en el esquema.
- **Decir por qué no hay sugerencias:** cuando todas se descartan, la respuesta incluye
  cuántas se generaron y cuántas se descartaron, para que el frontend explique el motivo en
  vez de mostrar el vacío. (El aviso genérico ya se agregó en el hotfix; esto lo hace
  específico.)
- **Modo guiado, aditivo:** el diálogo suma un selector múltiple de campos del monitor. Los
  campos elegidos viajan al prompt como contexto explícito ("el usuario quiere una regla sobre
  estos campos: …"), lo que ataca de raíz la causa principal de sugerencias descartadas: la IA
  inventando nombres de campo que no existen. El textarea libre sigue estando.

## Manejo de errores

- Configuración de timestamp derivado que referencia campos inexistentes, o formatos no
  soportados: `400` al guardar.
- Documento cuyo timestamp no se puede construir: se ingiere sin el campo derivado; el
  backfill lo reporta como "no construible" en vez de fallar todo el lote.
- `VelocityCondition` inválida: `400` al guardar la regla.
- Proveedor de IA sin responder: ya acotado a 90s por el hotfix previo.

## Testing

- **Derivación del timestamp:** TDD sobre la función pura que combina fecha + hora. Casos
  obligatorios: relleno de ceros (`21517` → `02:15:17`, el caso que motiva todo esto),
  medianoche (`0` → `00:00:00`), formato `HHMM`, campo faltante, valor no parseable.
- **Pipeline de velocidad:** tests de forma del pipeline (que incluya `$setWindowFields` con
  la partición y el orden correctos, y que el `$match` final compare contra un número de
  segundos), siguiendo el estilo de `rule_engine_test.go` — y **con la rama activada**, que es
  precisamente lo que faltó en la ventana temporal y dejó pasar el bug a producción.
- **Validación al guardar:** una `VelocityCondition` sin `TimeField`, con `MaxGap` inválido, o
  apuntando a un campo que no es `date`, debe rechazarse.
- **Backfill:** idempotencia (correrlo dos veces no cambia el resultado) y conteo correcto de
  no construibles.
- Sin tests de UI/componentes — mismo criterio que el resto del proyecto.

## Orden de implementación sugerido

El plan debe ejecutarse por fases, no todo junto:

1. **Fase 1 (desbloquea el caso):** timestamp derivado (modelo, ingesta, backfill, exposición
   en el esquema) + `VelocityCondition` (modelo, pipeline, validación).
2. **Fase 2:** UI — ventanas cortas, editor de `Filter`, editor de velocidad.
3. **Fase 3:** IA — prompt, validación menos agresiva, modo guiado.

Al terminar la Fase 1 el caso ya es expresable vía API; la Fase 2 lo hace usable desde la
interfaz; la Fase 3 lo hace generable automáticamente.
