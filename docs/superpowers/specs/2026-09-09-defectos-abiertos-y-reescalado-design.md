# Cinco defectos abiertos y el reescalado de decimales

**Fecha:** 2026-09-09
**Estado:** aprobado para planificar

## Por qué

Quedaron cinco defectos abiertos después de las fases 1-3. Cuatro son chicos; el quinto
—cambiar los decimales implícitos de un campo que ya tiene datos— muta montos guardados y
necesita un contrato explícito.

Tres de los cinco son la misma falla que este proyecto lleva persiguiendo desde el hotfix
de la ventana temporal: **una regla que parece configurada y no dispara nunca**. Ya
apareció comparando un `Date` contra un string, descartando toda sugerencia con `groupBy`
vacío, comparando un `mcc` numérico contra `"7995"`, y emitiendo `importe > 500000` sobre
documentos guardados como `5000`.

Medido en producción el 2026-09-09, aparece una cuarta vez y en vivo:

```
reglas totales: 59 · borradas: 58 · vivas: 1

VIVA: Múltiples cargos en casino superiores a 5000 USD en menos de 35 segundos
      activa=true · disparos=0
      timeField fechapoliza -> number · ventana 60s
```

La única regla viva del sistema no disparó nunca y no puede disparar: `buildAggregatePipeline`
compara `fechapoliza` (un número) contra un `time.Time`, y MongoDB no compara entre tipos
BSON. El monitor (`CBCG Tarjetas`, 4 registros, `importe` con 2 decimales implícitos) no
tiene timestamp derivado ni ningún campo de tipo `date`.

Ese dato decide una de las preguntas de diseño de abajo, y agrega un paso de puesta en
marcha que no necesita código.

## Alcance

Cinco piezas independientes. Cada una se puede implementar, revisar y desplegar sola.

## Fuera de alcance

- **Migrar red flags existentes al tipo nuevo.** Producción tiene 0 red flags y 0 reglas
  con condiciones de velocidad: no hay nada que migrar. Las alertas de velocidad que
  hubiera en un entorno local siguen marcadas `aggregate`; no se reetiquetan.
- **Cambiar `buildAggExpr`.** Su `default: $sum` está en el camino de evaluación de reglas
  ya persistidas. Se le cierra la entrada validando al guardar, no se le cambia la salida.
- **Guardar el valor crudo y convertir al leer.** Sería la arquitectura correcta para los
  decimales implícitos y es un rediseño aparte.
- **Bloquear cargas durante un reescalado.** No hay mecanismo de lock; ver "La carrera".

---

## Pieza 1 — Los descartes parciales se ven

`GenerateRules` ya devuelve `discarded` completo, con nombre y motivo por sugerencia. El
frontend solo lo pinta dentro del bloque que aparece cuando **ninguna** sugerencia
sobrevivió (`page.tsx`, dentro de `aiNoResults`).

Si la IA devuelve 5 y se descartan 3, el oficial ve 2 y asume que eso fue todo lo que el
modelo produjo. Es el caso donde más se lo puede inducir a error, porque no hay ninguna
señal de que falte algo.

**Cambio:** el bloque de descartes sale del `aiNoResults` y pasa a renderizarse siempre que
`aiDiscarded.length > 0`, arriba de la lista de sugerencias. El texto de cabecera se adapta
al caso: con sugerencias válidas dice cuántas se descartaron; sin ninguna, mantiene el
mensaje actual.

Solo frontend. El backend ya manda todo lo necesario.

---

## Pieza 2 — `RedFlagTypeVelocity`

Las alertas de velocidad se guardan hoy como `RedFlagTypeAggregate`, así que no se pueden
distinguir ni filtrar. Agregar el valor es la parte fácil; el problema son los seis sitios
que preguntan `redFlagType === "aggregate"` para decidir si una alerta es **agrupada**
—tiene `groupByField`, `groupByValue`, `matchCount`— y no si viene de una condición
agregada:

| Sitio | Qué decide |
|---|---|
| `backend/internal/services/red_flag_report.go:96` | Si el reporte imprime el bloque de agrupación |
| `backend/internal/handlers/red_flag_handler.go:293` | Idem en el detalle del caso |
| `frontend/src/app/(dashboard)/red-flags/page.tsx:123` | Si hace falta reparsear el mensaje |
| `frontend/src/app/(dashboard)/red-flags/page.tsx:655` | El contador de la tarjeta de resumen |
| `frontend/src/app/(dashboard)/red-flags/page.tsx:797` | Cómo se renderiza la fila |
| `frontend/src/lib/red-flag-report.ts:144` | Idem en el reporte del frontend |

Agregar `"velocity"` sin tocarlos haría que las alertas de velocidad se rendericen como si
fueran de fila y pierdan el grupo. El concepto "agrupada" ya existe en el código; lo que
falta es nombrarlo.

**Cambio:**

- `models.RedFlagTypeVelocity RedFlagType = "velocity"`, y `rule_engine.go` lo usa al armar
  las alertas de velocidad (`evaluateVelocityCondition` y su variante date-scoped).
- `func (t RedFlagType) EsAgrupada() bool` en `models/red_flag.go`: verdadero para
  `aggregate` y `velocity`. Los dos sitios de backend pasan a usarlo.
- `export function esAgrupada(t: RedFlagType): boolean` en `frontend/src/lib/types.ts`,
  junto al tipo que describe — el archivo ya exporta `CASE_LABELS`, así que no es
  solo-tipos. Los cuatro sitios de frontend pasan a usarlo.
- La tarjeta de resumen contaba "Agregadas"; pasa a contar las dos y se llama
  **"Agrupadas"**.
- La etiqueta de la fila deja de ser fija: `"Agregada"` para `aggregate`, `"Velocidad"`
  para `velocity`.

**Compatibilidad:** las alertas ya guardadas siguen con su tipo. `EsAgrupada()` las sigue
tratando igual que antes, así que ninguna vista cambia para los datos existentes.

---

## Pieza 3 — Borrar el evaluador en Go

`EvaluateRecord` → `evaluateConditionGroup` → `evaluateCondition` son ~90 líneas que
evalúan condiciones en memoria. **`EvaluateRecord` no tiene ningún llamador** — ni en
producción, ni en tests. El camino de ingesta usa `BuildMongoFilter` contra Mongo.

Al no ejercitarse, ya divergió de `conditionToMongo`: le faltan `in` y `not_in` (caen al
`return false` final) y su `eq` compara con `fmt.Sprintf("%v")` mientras el de Mongo cruza
tipos BSON con `$in`. Dos semánticas distintas para el mismo operador, y la que nadie corre
es la que parece correcta al leerla.

**Cambio:** borrar las tres funciones. Si alguna vez hace falta evaluar en memoria, se
escribe contra la semántica de `conditionToMongo`, que es la que está en producción.

---

## Pieza 4 — Validar las condiciones agregadas al guardar

`buildAggExpr` cae a `$sum` para cualquier `function` que no reconozca, así que un
`"distinct"` inventado o un `function` ausente comparan el umbral contra otra cantidad. Y
el `timeField` de un agregado no se chequea por tipo en ningún lado: es el defecto que la
evidencia de producción muestra vivo.

La fase 3 le cerró las dos puertas a la IA (`discardReason`). El editor manual las tiene
las dos abiertas.

**Cambio:** `ValidateAggregateCondition(cond models.AggregateCondition, schema []models.SchemaField) error`
en `backend/internal/services/aggregate_validate.go`, hermano de `ValidateVelocityCondition`,
llamado desde `Create` y `Update` de reglas. Rechaza:

- `function` fuera de `sum|count|avg|min|max`
- `field` vacío cuando `function != count`, o inexistente en el esquema
- `groupBy` inexistente (vacío es válido: agregado global)
- `timeField` inexistente, **o de un tipo distinto de `date`**
- ventana incompleta (uno de `timeField`/`timeWindow` sin el otro)
- `timeWindow` que no parsea a una duración positiva
- `filter` sobre campos inexistentes

Es deliberadamente el mismo criterio que `discardReason` aplica a las sugerencias de IA: lo
que la IA no puede sugerir tampoco se puede escribir a mano.

**Se valida en `Create` y en `Update`.** En `Update` es lo que importa: el editor manual
deja elegir cualquier campo como `timeField`, así que validar solo al crear dejaría abierto
el camino de introducir el defecto editando una regla sana.

**Costo medido:** una sola regla viva en producción queda bloqueada al editarse, y es una
que nunca disparó. El 400 no interrumpe trabajo; informa de una regla muerta.

---

## Pieza 5 — Reescalar al cambiar los decimales implícitos

Los decimales implícitos se aplican **en la ingesta**: `parseValue` divide por `10^n` antes
de guardar. Cambiar `impliedDecimals` en el esquema no toca lo ya guardado, así que la
colección queda con dos escalas para el mismo campo y nada lo señala. A diferencia del
timestamp derivado, no hay backfill.

**Contrato:** `PUT /monitors/:id/schema` acepta `rescaleExisting bool` junto a `schema`.

| Situación | Respuesta |
|---|---|
| Ningún `impliedDecimals` cambió | Igual que hoy. `rescaleExisting` se ignora. |
| Cambió y la colección está vacía | Se guarda, sin reescalar. |
| Cambió, hay datos, `rescaleExisting` ausente o falso | **409**, con los campos afectados, su cambio de escala y el conteo de registros. No se guarda nada. |
| Cambió, hay datos, `rescaleExisting: true` | Se guarda el esquema y se convierten los documentos en la misma petición. La respuesta incluye cuántos se convirtieron por campo. |

**La conversión.** Con `delta = nuevo - viejo`:

- `delta > 0` (más decimales): el valor guardado se achica → dividir por `10^delta`
- `delta < 0` (menos decimales): se agranda → multiplicar por `10^(-delta)`

Se hace con un pipeline de update (`$set` con `$divide` / `$multiply`) por una **potencia de
diez entera**, nunca multiplicando por su recíproco: `5000 * 0.1` en float64 da
`500.00000000000006`, `5000 / 10` da `500` exacto. Son montos.

El filtro es `{campo: {$type: "number"}, _ingested_at: {$lte: cutoff}}` — los documentos sin
el campo, o con basura no numérica, se dejan como están.

**La carrera.** Entre el cambio de esquema y una carga concurrente hay una ventana que sin
un lock no se cierra. Se acota tomando el `cutoff` **antes** de escribir el esquema:

- Documento ingerido antes del `cutoff`: escala vieja, entra en el filtro, se convierte. ✔
- Documento ingerido después de escribir el esquema: ya viene en escala nueva, queda fuera
  del filtro. ✔
- Documento ingerido en la ventana entre el `cutoff` y la escritura: escala vieja, queda
  fuera del filtro, **se queda en la escala vieja**. ✘

La dirección es deliberada. El modo de falla que queda es "una fila se quedó en la escala
vieja" —visible al mirar los datos y corregible— y no "una fila se convirtió dos veces",
que es silencioso y no se distingue de un monto legítimo. La ventana es de milisegundos y
las cargas son manuales.

**Frontend:** al guardar el esquema, si algún campo cambió de decimales y el monitor tiene
registros, se muestra un `AlertDialog` con los campos, la conversión de ejemplo y el
conteo, y recién al confirmar se manda `rescaleExisting: true`. El texto dice explícitamente
que la operación no se deshace sola.

**Repositorio:** `RescaleDataField(ctx, collectionID, field string, delta int, cutoff time.Time) (int64, error)`
en `MonitorRepository`, junto a `BackfillDataField`. La expresión se arma en una función
pura y testeable aparte; el método del repositorio es una envoltura fina, consistente con
que el paquete `repository` no tiene tests hoy.

---

## Manejo de errores

- Condición agregada inválida al crear o editar una regla: **400** con el motivo en español.
- Cambio de decimales con datos y sin `rescaleExisting`: **409** con los campos y el conteo.
- Fallo a mitad de un reescalado de varios campos: los campos ya convertidos quedan
  convertidos. La respuesta informa cuáles se completaron; no hay rollback. Reintentar la
  misma petición volvería a convertir los ya hechos, así que la respuesta de error lo dice
  explícitamente y pide revisar antes de reintentar.

## Testing

- **Pieza 1:** verificación visual; no hay lógica nueva.
- **Pieza 2:** test de `EsAgrupada()` sobre los tres tipos; test de `esAgrupada` en vitest.
- **Pieza 3:** el borrado se valida con que la suite siga verde.
- **Pieza 4:** tabla sobre `ValidateAggregateCondition` con un caso por rama de rechazo,
  más los casos válidos (agregado global, `count` sin `field`, sin ventana). Un caso usa
  exactamente la forma de la regla viva de producción (`timeField` de tipo `number`).
- **Pieza 5:** tests de la función pura que arma la expresión, cubriendo `delta` positivo,
  negativo y cero, y comprobando que se usa una potencia de diez entera y no un recíproco.
  Test del handler para las cuatro filas de la tabla del contrato.

## Orden

1. Pieza 1 (frontend aislado)
2. Pieza 3 (borrado aislado)
3. Pieza 2 (tipo + predicado)
4. Pieza 4 (validación)
5. Pieza 5 (reescalado)

Las primeras cuatro no se tocan entre sí. La quinta es la única con riesgo sobre datos y va
al final para que llegue con la suite ya verde.

## Puesta en marcha en producción

No requiere código. Después de desplegar, sobre `CBCG Tarjetas`:

1. Configurar el timestamp derivado: `fechapoliza` (`YYYYMMDD`) + `horapoliza` (`HHMMSS`)
   → `timestamp`, y correr el backfill.
2. Rearmar la regla viva como condición de velocidad sobre `timestamp`, agrupando por
   tarjeta, con gap de 35s y filtro por `mcc` — en vez del agregado con ventana de 60s
   sobre `fechapoliza`, que no puede disparar.
3. Verificar que el umbral de `importe` esté en la escala guardada (el campo tiene 2
   decimales implícitos).
