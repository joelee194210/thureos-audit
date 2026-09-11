# Cinco defectos abiertos y el reescalado de decimales — Plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cerrar los cinco defectos que quedaron abiertos después de las fases 1-3, incluido el que muta montos guardados al cambiar los decimales implícitos de un campo.

**Architecture:** Cinco piezas independientes. Tres de ellas —el tipo de red flag, la validación de agregados y el reescalado— comparten un patrón que ya existe en el repositorio: un validador puro en `services` que el handler llama al guardar, y un predicado con nombre en vez de una comparación repetida. Ninguna toca el camino de evaluación de reglas ya persistidas.

**Tech Stack:** Go 1.26 / Fiber v2 / MongoDB 7 · Next.js 16 / React 19 / TypeScript / shadcn-ui

**Spec:** `docs/superpowers/specs/2026-09-09-defectos-abiertos-y-reescalado-design.md`

## Global Constraints

- **Copy de usuario en español**, con acentos correctos. Nombres de campo, operadores y claves JSON en inglés.
- **Nombres de tests y comentarios en español**, como el resto del repositorio.
- **Ningún componente de frontend escribe un color literal**: utilidades de token o `@/lib/semantic-colors`.
- **No se toca `buildAggExpr`** (`rule_engine.go`): está en el camino de evaluación de reglas ya persistidas. Se le cierra la entrada validando al guardar.
- **No se reetiquetan red flags existentes.** Producción tiene 0; las que hubiera en local se quedan como están.
- **Los montos se convierten con potencias de diez enteras**, nunca multiplicando por su recíproco: `5000 * 0.1` da `500.00000000000006` en float64.
- Cada tarea termina con `go test -C backend ./...` y, si tocó frontend, `npx tsc --noEmit` + `npm run lint` + `npm run test:run` desde `frontend/`, todo en verde antes del commit.
- `frontend/src/app/(dashboard)/rules/page.tsx`, `.../red-flags/page.tsx` y `frontend/src/components/tabs/content/monitor-tab-content.tsx` son archivos grandes preexistentes. Hacer la edición mínima; no reestructurarlos. El drift de Prettier en ellos es preexistente y no se arregla acá.
- **No usar `git stash`**: el stack está compartido con otros checkouts y otras sesiones.

---

## Estructura de archivos

| Archivo | Responsabilidad | Tarea |
|---|---|---|
| `frontend/src/app/(dashboard)/rules/page.tsx` | Diálogo de IA: aviso de descartes | 1 |
| `backend/internal/services/rule_engine.go` | Borrado del evaluador muerto; tipo de la alerta de velocidad | 2, 3 |
| `backend/internal/models/red_flag.go` | `RedFlagTypeVelocity` + `EsAgrupada()` | 3 |
| `backend/internal/services/red_flag_report.go` | Usa el predicado | 3 |
| `backend/internal/handlers/red_flag_handler.go` | Usa el predicado | 3 |
| `frontend/src/lib/types.ts` | `RedFlagType` + `esAgrupada()` | 3 |
| `frontend/src/app/(dashboard)/red-flags/page.tsx` | Usa el predicado; etiqueta y contador | 3 |
| `frontend/src/lib/red-flag-report.ts` | Usa el predicado | 3 |
| `backend/internal/services/aggregate_validate.go` | **Nuevo.** Validación de condiciones agregadas | 4 |
| `backend/internal/handlers/rule_handler.go` | Llama al validador en Create y Update | 4 |
| `backend/internal/repository/monitor_repo.go` | `RescaleDataField` + expresión pura | 5 |
| `backend/internal/handlers/monitor_handler.go` | Contrato del `PUT /schema` | 5 |
| `frontend/src/lib/api/monitors.ts` | `updateSchema` con `rescaleExisting` | 5 |
| `frontend/src/components/tabs/content/monitor-tab-content.tsx` | Diálogo de confirmación | 5 |

Las tareas 1 y 2 no comparten ningún archivo con las demás. Las tareas 3, 4 y 5 tocan archivos distintos entre sí. **Ninguna tarea depende de otra**, pero se ejecutan en este orden para que la que tiene riesgo sobre datos llegue con la suite verde.

---

### Task 1: El aviso de descartes aparece aunque queden sugerencias válidas

El backend ya manda `discarded` completo. El frontend solo lo pinta dentro del bloque que aparece cuando **ninguna** sugerencia sobrevivió, así que con 5 generadas y 3 descartadas el oficial ve 2 y asume que eso fue todo lo que el modelo produjo.

**Files:**
- Modify: `frontend/src/app/(dashboard)/rules/page.tsx` — el bloque `{aiNoResults && !aiLoading && (...)}`, hoy en las líneas 1243-1265

**Interfaces:**
- Consumes: `aiDiscarded: DiscardedSuggestion[]`, `aiSuggestions: AIRuleSuggestion[]`, `aiNoResults: boolean`, `aiLoading: boolean` — todo ya existe en el componente.
- Produces: nada que otra tarea consuma.

**Sin test automatizado.** No hay lógica nueva: es una condición de renderizado. Un test que monte el diálogo y afirme que el texto aparece sería un detector de cambios sobre JSX. La verificación es manual y está en el Step 3.

- [ ] **Step 1: Reemplazar el bloque**

Sustituir íntegro el bloque `{aiNoResults && !aiLoading && (...)}` por estos dos:

```tsx
                  {/* Los descartes se muestran siempre que los haya, no solo
                      cuando no quedó ninguna sugerencia válida: con 5
                      generadas y 3 descartadas, ver 2 sin más contexto es
                      justo el caso donde se asume que eso fue todo. */}
                  {!aiLoading && aiDiscarded.length > 0 && (
                    <div className="rounded-md border border-dashed p-4">
                      <p className="text-sm font-medium">
                        {aiSuggestions.length > 0
                          ? `Se descartaron ${aiDiscarded.length} de ${
                              aiDiscarded.length + aiSuggestions.length
                            } sugerencias`
                          : "La IA no devolvió ninguna sugerencia válida"}
                      </p>
                      <ul className="mt-2 space-y-1">
                        {aiDiscarded.map((d, i) => (
                          <li key={i} className="text-xs text-muted-foreground">
                            <span className="font-medium">
                              {d.name || "Sin nombre"}
                            </span>
                            {": "}
                            {d.reason}
                          </li>
                        ))}
                      </ul>
                    </div>
                  )}

                  {aiNoResults && !aiLoading && aiDiscarded.length === 0 && (
                    <div className="rounded-md border border-dashed p-4">
                      <p className="text-sm font-medium">
                        La IA no devolvió ninguna sugerencia válida
                      </p>
                      <p className="mt-1 text-xs text-muted-foreground">
                        El modelo no devolvió ninguna regla. Probá describir el
                        criterio con los nombres exactos de los campos.
                      </p>
                    </div>
                  )}
```

Los tres casos quedan cubiertos: hay descartes (con o sin sugerencias válidas) → lista detallada; el modelo no devolvió nada → mensaje genérico; todo salió bien → no se muestra nada.

- [ ] **Step 2: Verificar que compila y pasa lint**

```bash
cd frontend && npx tsc --noEmit && npm run lint && npm run test:run
```
Expected: tsc sin salida, lint 0 errores (18 warnings preexistentes), vitest 28/28.

- [ ] **Step 3: Verificación manual**

Con el backend en `:8080` (`MONGO_URI=mongodb://localhost:27019/thureos_compliance`, `REDIS_URL=redis://localhost:6383`) y el frontend en `:3000`, sobre cualquier monitor con datos:

1. Abrir "Generar con IA" y pedir algo mitad expresable y mitad no, por ejemplo: *"reglas sobre montos altos y también sobre la geolocalización del cajero"*. El esquema no tiene geolocalización, así que parte de las sugerencias se descartan y parte sobreviven.
2. Confirmar que aparece el aviso **arriba de las sugerencias** diciendo "Se descartaron N de M sugerencias", con el nombre y el motivo de cada una.
3. Pedir algo enteramente inexpresable y confirmar que el encabezado vuelve a ser "La IA no devolvió ninguna sugerencia válida", con la lista de motivos.

Si el proveedor de IA no está configurado en el entorno local, alcanza con forzar `aiDiscarded` desde React DevTools y comprobar los tres estados; dejarlo dicho en el reporte si se hizo así.

- [ ] **Step 4: Commit**

```bash
git add "frontend/src/app/(dashboard)/rules/page.tsx"
git commit -m "fix(ia): mostrar los descartes aunque queden sugerencias válidas"
```

---

### Task 2: Borrar el evaluador de condiciones en Go

`EvaluateRecord` → `evaluateConditionGroup` → `evaluateCondition` evalúan condiciones en memoria. `EvaluateRecord` no tiene ningún llamador: el camino de ingesta usa `BuildMongoFilter` contra Mongo. Al no ejercitarse ya divergió de `conditionToMongo` —le faltan `in` y `not_in`, y su `eq` compara con `fmt.Sprintf("%v")` mientras el de Mongo cruza tipos BSON con `$in`— así que hay dos semánticas para el mismo operador y la que nadie corre es la que parece correcta al leerla.

**Files:**
- Modify: `backend/internal/services/rule_engine.go` — borrar las tres funciones (hoy líneas 245-329, desde el comentario `// EvaluateRecord checks a single record against a rule` hasta el cierre de `evaluateCondition`, justo antes del comentario `// BuildMongoFilter converts a ConditionGroup to a MongoDB query filter`)

**Interfaces:**
- Produces: nada. Es un borrado.
- `toFloat`, `regexp` y `strings` siguen usándose en `conditionToMongo`, `equalityCandidates`, `listCandidates` y `parseTimeWindow`: **los imports no cambian**.

- [ ] **Step 1: Confirmar que no hay llamadores**

```bash
grep -rn "EvaluateRecord\|evaluateConditionGroup\|evaluateCondition" backend/
```
Expected: solo las definiciones y las dos llamadas internas dentro de `evaluateConditionGroup`. Si aparece cualquier otro llamador —en `cmd/`, en un handler o en un test— **parar y reportar**: el supuesto del plan no se sostiene y el borrado deja de ser correcto.

- [ ] **Step 2: Borrar las tres funciones**

Borrar el bloque completo, incluidos sus comentarios. No dejar un stub ni un `//nolint`.

- [ ] **Step 3: Compilar y correr la suite**

```bash
go build -C backend ./... && go test -C backend ./...
```
Expected: compila sin errores de import no usado; todos los paquetes en verde. Si el compilador marca `regexp` o `strings` como no usados, revisar: significa que el borrado se llevó más de lo que debía.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/services/rule_engine.go
git commit -m "refactor(reglas): borrar el evaluador en Go, que no tiene llamadores"
```

---

### Task 3: `RedFlagTypeVelocity` y el predicado de alerta agrupada

Las alertas de velocidad se guardan como `aggregate`, así que no se pueden distinguir ni filtrar. Agregar el valor es una línea; lo que hay que arreglar son los seis sitios que preguntan `redFlagType === "aggregate"` para decidir si la alerta es **agrupada** —tiene `groupByField`, `groupByValue`, `matchCount`— y no de qué condición vino. Sin tocarlos, las alertas de velocidad se renderizarían como si fueran de fila y perderían el grupo.

**Files:**
- Modify: `backend/internal/models/red_flag.go` — constante y método
- Modify: `backend/internal/services/rule_engine.go` — `velocityRedFlagsFromResults`, el campo `RedFlagType`
- Modify: `backend/internal/services/red_flag_report.go:96`
- Modify: `backend/internal/handlers/red_flag_handler.go:293`
- Modify: `frontend/src/lib/types.ts` — tipo y predicado
- Modify: `frontend/src/app/(dashboard)/red-flags/page.tsx` — líneas 123, 655, 797 y la etiqueta de la línea 821
- Modify: `frontend/src/lib/red-flag-report.ts:144`
- Test: `backend/internal/services/red_flag_report_test.go` (o un archivo nuevo `backend/internal/models/red_flag_test.go`), y `frontend/src/lib/types.test.ts` (nuevo)

**Interfaces:**
- Produces: `models.RedFlagTypeVelocity RedFlagType = "velocity"`; `func (t models.RedFlagType) EsAgrupada() bool`; `export function esAgrupada(t: RedFlagType): boolean`.

- [ ] **Step 1: Escribir los tests que fallan**

Crear `backend/internal/models/red_flag_test.go`:

```go
package models

import "testing"

// Los seis sitios que preguntaban `== RedFlagTypeAggregate` querían saber si
// la alerta es agrupada —si trae groupByField, groupByValue y matchCount—, no
// de qué condición vino. Velocidad las trae igual.
func TestEsAgrupada(t *testing.T) {
	casos := []struct {
		tipo     RedFlagType
		esperado bool
	}{
		{RedFlagTypeAggregate, true},
		{RedFlagTypeVelocity, true},
		{RedFlagTypeRow, false},
		{RedFlagType(""), false},
	}
	for _, c := range casos {
		if got := c.tipo.EsAgrupada(); got != c.esperado {
			t.Errorf("EsAgrupada(%q) = %v, quería %v", c.tipo, got, c.esperado)
		}
	}
}
```

Crear `frontend/src/lib/types.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { esAgrupada } from "./types";

describe("esAgrupada", () => {
  it("es verdadero para los tipos que traen grupo", () => {
    expect(esAgrupada("aggregate")).toBe(true);
    expect(esAgrupada("velocity")).toBe(true);
  });

  it("es falso para las alertas de fila", () => {
    expect(esAgrupada("row")).toBe(false);
  });
});
```

- [ ] **Step 2: Correr los tests y ver que fallan**

```bash
go test -C backend ./internal/models/ -run TestEsAgrupada -v
cd frontend && npx vitest run src/lib/types.test.ts
```
Expected: Go falla con `undefined: RedFlagTypeVelocity`; vitest falla porque `esAgrupada` no está exportado.

- [ ] **Step 3: Backend — tipo y predicado**

En `backend/internal/models/red_flag.go`, reemplazar el bloque de constantes:

```go
const (
	RedFlagTypeRow       RedFlagType = "row"
	RedFlagTypeAggregate RedFlagType = "aggregate"
	RedFlagTypeVelocity  RedFlagType = "velocity"
)

// EsAgrupada dice si la alerta describe un grupo de registros —y por lo
// tanto trae GroupByField, GroupByValue y MatchCount— en vez de una fila
// suelta. Existe como predicado con nombre porque seis sitios preguntaban
// `== RedFlagTypeAggregate` queriendo saber esto, y al aparecer un segundo
// tipo agrupado los seis habrían quedado mal en silencio.
func (t RedFlagType) EsAgrupada() bool {
	return t == RedFlagTypeAggregate || t == RedFlagTypeVelocity
}
```

- [ ] **Step 4: Backend — usar el tipo y el predicado**

En `backend/internal/services/rule_engine.go`, dentro de `velocityRedFlagsFromResults`, cambiar el campo del `models.RedFlag` que se construye:

```go
			RedFlagType:  models.RedFlagTypeVelocity,
```

En `backend/internal/services/red_flag_report.go:96`:

```go
	isAggregate := rf.RedFlagType.EsAgrupada()
```

En `backend/internal/handlers/red_flag_handler.go:293`:

```go
	if redFlag.RedFlagType.EsAgrupada() {
```

- [ ] **Step 5: Frontend — tipo y predicado**

En `frontend/src/lib/types.ts`, ampliar el tipo y agregar el predicado justo debajo:

```ts
export type RedFlagType = "row" | "aggregate" | "velocity";

/**
 * Si la alerta describe un grupo de registros —y por lo tanto trae
 * groupByField, groupByValue y matchCount— en vez de una fila suelta. Las
 * de velocidad y las de agregado lo son; las de fila no.
 */
export function esAgrupada(t: RedFlagType): boolean {
  return t === "aggregate" || t === "velocity";
}
```

- [ ] **Step 6: Frontend — los cuatro sitios**

En `frontend/src/lib/red-flag-report.ts:144`:

```ts
  const isAggregate = esAgrupada(redFlag.redFlagType);
```
(importar `esAgrupada` desde `@/lib/types`, donde ese archivo ya importa `RedFlag`.)

En `frontend/src/app/(dashboard)/red-flags/page.tsx`, importar `esAgrupada` desde `@/lib/types` y cambiar:

- línea 123, dentro de `parseAggregateFromMessage`:
  ```tsx
  if (esAgrupada(redFlag.redFlagType)) return redFlag;
  ```
- línea 655, el contador:
  ```tsx
      if (esAgrupada(a.redFlagType)) aggCount++;
  ```
- línea 797:
  ```tsx
              const isAggregate = esAgrupada(redFlag.redFlagType);
  ```

- [ ] **Step 7: Frontend — la etiqueta y el contador dicen la verdad**

La tarjeta de resumen contaba solo agregadas y ahora cuenta las dos. En `frontend/src/app/(dashboard)/red-flags/page.tsx:712`:

```tsx
                  <p className="text-xs text-muted-foreground">Agrupadas</p>
```

Y la etiqueta de la fila deja de ser fija. En la línea 821, reemplazar el texto `Agregada` por:

```tsx
                              {redFlag.redFlagType === "velocity"
                                ? "Velocidad"
                                : "Agregada"}
```

- [ ] **Step 8: Correr todo**

```bash
go test -C backend ./...
cd frontend && npx tsc --noEmit && npm run lint && npm run test:run
```
Expected: todo verde, incluidos los dos tests nuevos.

- [ ] **Step 9: Commit**

```bash
git add backend/internal/models/red_flag.go backend/internal/models/red_flag_test.go backend/internal/services/rule_engine.go backend/internal/services/red_flag_report.go backend/internal/handlers/red_flag_handler.go frontend/src/lib/types.ts frontend/src/lib/types.test.ts frontend/src/lib/red-flag-report.ts "frontend/src/app/(dashboard)/red-flags/page.tsx"
git commit -m "feat(alertas): tipo propio para las alertas de velocidad"
```

---

### Task 4: Validar las condiciones agregadas al guardar la regla

`buildAggExpr` cae a `$sum` para cualquier `function` que no reconozca, y el `timeField` de un agregado no se chequea por tipo en ningún lado. La fase 3 le cerró las dos puertas a la IA; el editor manual las tiene abiertas. Medido en producción: la única regla viva del sistema tiene un agregado con ventana de 60s sobre `fechapoliza`, que es `number`, y nunca disparó.

**Files:**
- Create: `backend/internal/services/aggregate_validate.go`
- Modify: `backend/internal/handlers/rule_handler.go` — el bloque de validación en `Create` (hoy líneas 158-170) y en `Update` (hoy líneas 292-312)
- Test: `backend/internal/services/aggregate_validate_test.go`

**Interfaces:**
- Produces: `func ValidateAggregateCondition(cond models.AggregateCondition, schema []models.SchemaField) error` — devuelve `nil` si la condición puede evaluarse, o el motivo en español.
- Consumes: `parseTimeWindow` (`rule_engine.go`) y `models.AggFunc*`.

- [ ] **Step 1: Escribir los tests que fallan**

Crear `backend/internal/services/aggregate_validate_test.go`:

```go
package services

import (
	"strings"
	"testing"

	"github.com/thureos/compliance/internal/models"
)

func esquemaAgg() []models.SchemaField {
	return []models.SchemaField{
		{Name: "importe", Type: models.FieldNumber},
		{Name: "tarjeta", Type: models.FieldString},
		{Name: "fechapoliza", Type: models.FieldNumber},
		{Name: "timestamp", Type: models.FieldDate},
	}
}

func TestValidateAggregateCondition_Acepta(t *testing.T) {
	casos := []struct {
		nombre string
		cond   models.AggregateCondition
	}{
		{"suma con ventana", models.AggregateCondition{
			Field: "importe", Function: models.AggFuncSum, GroupBy: "tarjeta",
			TimeField: "timestamp", TimeWindow: "24h",
			Operator: models.OpGreaterThan, Threshold: 5000,
		}},
		{"agregado global", models.AggregateCondition{
			Field: "importe", Function: models.AggFuncSum, GroupBy: "",
			TimeField: "timestamp", TimeWindow: "24h",
			Operator: models.OpGreaterThan, Threshold: 5000,
		}},
		{"count sin field", models.AggregateCondition{
			Function: models.AggFuncCount, GroupBy: "tarjeta",
			Operator: models.OpGreaterEqual, Threshold: 3,
		}},
		{"sin ventana", models.AggregateCondition{
			Field: "importe", Function: models.AggFuncSum, GroupBy: "tarjeta",
			Operator: models.OpGreaterThan, Threshold: 5000,
		}},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if err := ValidateAggregateCondition(c.cond, esquemaAgg()); err != nil {
				t.Errorf("la rechazó: %v", err)
			}
		})
	}
}

// El caso exacto de la única regla viva en producción: ventana de 60s sobre
// un campo numérico. $match compara contra un time.Time, no matchea nada y
// la regla no dispara nunca.
func TestValidateAggregateCondition_RechazaTimeFieldNoDate(t *testing.T) {
	cond := models.AggregateCondition{
		Field: "importe", Function: models.AggFuncSum, GroupBy: "tarjeta",
		TimeField: "fechapoliza", TimeWindow: "60s",
		Operator: models.OpGreaterThan, Threshold: 5000,
	}
	err := ValidateAggregateCondition(cond, esquemaAgg())
	if err == nil {
		t.Fatal("la aceptó; tenía que rechazarla")
	}
	if !strings.Contains(err.Error(), "date") {
		t.Errorf("el mensaje no dice que hace falta un campo date: %v", err)
	}
}

func TestValidateAggregateCondition_Rechaza(t *testing.T) {
	casos := []struct {
		nombre string
		cond   models.AggregateCondition
	}{
		{"función inventada", models.AggregateCondition{
			Field: "importe", Function: models.AggFunction("distinct"), GroupBy: "tarjeta",
			Operator: models.OpGreaterThan, Threshold: 1,
		}},
		{"función vacía", models.AggregateCondition{
			Field: "importe", GroupBy: "tarjeta",
			Operator: models.OpGreaterThan, Threshold: 1,
		}},
		{"field vacío sin ser count", models.AggregateCondition{
			Function: models.AggFuncSum, GroupBy: "tarjeta",
			Operator: models.OpGreaterThan, Threshold: 1,
		}},
		{"field inexistente", models.AggregateCondition{
			Field: "no_existe", Function: models.AggFuncSum, GroupBy: "tarjeta",
			Operator: models.OpGreaterThan, Threshold: 1,
		}},
		{"groupBy inexistente", models.AggregateCondition{
			Field: "importe", Function: models.AggFuncSum, GroupBy: "no_existe",
			Operator: models.OpGreaterThan, Threshold: 1,
		}},
		{"timeField inexistente", models.AggregateCondition{
			Field: "importe", Function: models.AggFuncSum, GroupBy: "tarjeta",
			TimeField: "no_existe", TimeWindow: "24h",
			Operator: models.OpGreaterThan, Threshold: 1,
		}},
		{"ventana incompleta", models.AggregateCondition{
			Field: "importe", Function: models.AggFuncSum, GroupBy: "tarjeta",
			TimeField: "timestamp",
			Operator: models.OpGreaterThan, Threshold: 1,
		}},
		{"ventana de cero", models.AggregateCondition{
			Field: "importe", Function: models.AggFuncSum, GroupBy: "tarjeta",
			TimeField: "timestamp", TimeWindow: "0s",
			Operator: models.OpGreaterThan, Threshold: 1,
		}},
		{"ventana con unidad ambigua", models.AggregateCondition{
			Field: "importe", Function: models.AggFuncSum, GroupBy: "tarjeta",
			TimeField: "timestamp", TimeWindow: "5m",
			Operator: models.OpGreaterThan, Threshold: 1,
		}},
		{"filtro sobre campo inexistente", models.AggregateCondition{
			Field: "importe", Function: models.AggFuncSum, GroupBy: "tarjeta",
			Operator: models.OpGreaterThan, Threshold: 1,
			Filter: []models.Condition{{Field: "no_existe", Operator: models.OpEqual, Value: 1}},
		}},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if err := ValidateAggregateCondition(c.cond, esquemaAgg()); err == nil {
				t.Error("la aceptó; tenía que rechazarla")
			}
		})
	}
}
```

- [ ] **Step 2: Correr los tests y ver que fallan**

```bash
go test -C backend ./internal/services/ -run TestValidateAggregateCondition -v
```
Expected: FAIL con `undefined: ValidateAggregateCondition`.

- [ ] **Step 3: Escribir el validador**

Crear `backend/internal/services/aggregate_validate.go`:

```go
package services

import (
	"fmt"

	"github.com/thureos/compliance/internal/models"
)

// aggFuncionesValidas son las que buildAggExpr sabe traducir. Cualquier otra
// cae a su `default: $sum`, así que el umbral termina comparándose contra
// una cantidad distinta de la que el usuario pidió, sin ningún error.
var aggFuncionesValidas = map[models.AggFunction]bool{
	models.AggFuncSum:   true,
	models.AggFuncCount: true,
	models.AggFuncAvg:   true,
	models.AggFuncMin:   true,
	models.AggFuncMax:   true,
}

// ValidateAggregateCondition rechaza al guardar las condiciones agregadas
// que no podrían disparar nunca. Es deliberadamente el mismo criterio que
// discardReason aplica a las sugerencias de IA: lo que la IA no puede
// sugerir tampoco se puede escribir a mano desde el editor.
//
// El chequeo de tipo sobre TimeField es el que más importa: la ventana se
// arma con un $match contra un time.Time (rule_engine.go, buildAggregatePipeline)
// y MongoDB no compara entre tipos BSON, así que una ventana sobre un campo
// numérico descarta todos los documentos en silencio.
func ValidateAggregateCondition(cond models.AggregateCondition, schema []models.SchemaField) error {
	byName := make(map[string]models.SchemaField, len(schema))
	for _, f := range schema {
		byName[f.Name] = f
	}

	if !aggFuncionesValidas[cond.Function] {
		return fmt.Errorf("función de agregación inválida: %q (válidas: sum, count, avg, min, max)", cond.Function)
	}

	// Para count el motor ignora Field (buildAggExpr emite $sum:1).
	if cond.Function != models.AggFuncCount && cond.Field == "" {
		return fmt.Errorf("el agregado no dice qué campo agregar")
	}
	if cond.Field != "" {
		if _, ok := byName[cond.Field]; !ok {
			return fmt.Errorf("el campo a agregar %q no existe en el esquema del monitor", cond.Field)
		}
	}

	// Un groupBy vacío es un agregado global: el motor agrupa con _id:null.
	if cond.GroupBy != "" {
		if _, ok := byName[cond.GroupBy]; !ok {
			return fmt.Errorf("el campo de agrupación %q no existe en el esquema del monitor", cond.GroupBy)
		}
	}

	if (cond.TimeField == "") != (cond.TimeWindow == "") {
		return fmt.Errorf("la ventana de tiempo está incompleta: hacen falta el campo de fecha y la duración")
	}
	if cond.TimeField != "" {
		f, ok := byName[cond.TimeField]
		if !ok {
			return fmt.Errorf("el campo de tiempo %q no existe en el esquema del monitor", cond.TimeField)
		}
		if f.Type != models.FieldDate {
			return fmt.Errorf("el campo de tiempo %q es de tipo %s: se necesita un campo date", cond.TimeField, f.Type)
		}
		if parseTimeWindow(cond.TimeWindow) <= 0 {
			return fmt.Errorf("ventana de tiempo inválida: %q (formatos válidos: 35s, 5min, 24h, 7d)", cond.TimeWindow)
		}
	}

	for _, f := range cond.Filter {
		if _, ok := byName[f.Field]; !ok {
			return fmt.Errorf("el filtro del agregado referencia un campo inexistente: %q", f.Field)
		}
	}

	return nil
}
```

- [ ] **Step 4: Correr los tests**

```bash
go test -C backend ./internal/services/ -run TestValidateAggregateCondition -v
```
Expected: PASS, los 15 subtests.

- [ ] **Step 5: Llamarlo desde `Create`**

En `backend/internal/handlers/rule_handler.go`, el bloque de `Create` que hoy empieza en `if len(req.VelocityConditions) > 0 {` y busca el monitor adentro. Reemplazarlo por:

```go
	if len(req.VelocityConditions) > 0 || len(req.AggregateConditions) > 0 {
		monitor, err := h.monitorRepo.FindByID(c.Context(), monitorID)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
		}
		for _, ac := range req.AggregateConditions {
			if err := services.ValidateAggregateCondition(ac, monitor.Schema); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
			}
		}
		if len(req.VelocityConditions) > 0 {
			req.VelocityConditions = services.NormalizeVelocityConditions(req.VelocityConditions)
			for _, vc := range req.VelocityConditions {
				if err := services.ValidateVelocityCondition(vc, monitor.Schema); err != nil {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
				}
			}
			h.ensureVelocityIndexes(c.Context(), monitor, req.VelocityConditions)
		}
	}
```

- [ ] **Step 6: Llamarlo desde `Update`**

En `Update`, el bloque `if req.VelocityConditions != nil { ... }` busca la regla y el monitor adentro. Los agregados se asignan antes, en `if req.AggregateConditions != nil { update["aggregate_conditions"] = ... }`, **sin validar**.

Reemplazar los dos bloques por uno solo que busque la regla y el monitor una vez:

```go
	if req.AggregateConditions != nil || req.VelocityConditions != nil {
		// Misma validación que al crear: sin esto, editar sería una puerta
		// trasera para guardar una condición que jamás dispararía — el modo
		// de falla silenciosa que toda esta funcionalidad evita.
		rule, err := h.ruleRepo.FindByID(c.Context(), id)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "rule not found"})
		}
		monitor, err := h.monitorRepo.FindByID(c.Context(), rule.MonitorID)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
		}
		if req.AggregateConditions != nil {
			for _, ac := range req.AggregateConditions {
				if err := services.ValidateAggregateCondition(ac, monitor.Schema); err != nil {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
				}
			}
			update["aggregate_conditions"] = req.AggregateConditions
		}
		if req.VelocityConditions != nil {
			req.VelocityConditions = services.NormalizeVelocityConditions(req.VelocityConditions)
			for _, vc := range req.VelocityConditions {
				if err := services.ValidateVelocityCondition(vc, monitor.Schema); err != nil {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
				}
			}
			h.ensureVelocityIndexes(c.Context(), monitor, req.VelocityConditions)
			update["velocity_conditions"] = req.VelocityConditions
		}
	}
```

Cuidado al mover el `update["aggregate_conditions"]`: tiene que quedar **dentro** del nuevo bloque y solo cuando `req.AggregateConditions != nil`, para no borrar los agregados de una regla en una edición que no los mandaba.

- [ ] **Step 7: Correr la suite completa**

```bash
go test -C backend ./...
```
Expected: todo verde. Los tests de handlers que crean o editan reglas con agregados ahora pasan por el validador: si alguno falla, revisar si su fixture tenía un `timeField` que no era `date` — en ese caso el fixture encodifica el defecto y hay que arreglar el fixture, no aflojar el validador. Si aparece un caso así, decirlo en el reporte.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/services/aggregate_validate.go backend/internal/services/aggregate_validate_test.go backend/internal/handlers/rule_handler.go
git commit -m "feat(reglas): validar las condiciones agregadas al guardar"
```

---

### Task 5: Reescalar los datos al cambiar los decimales implícitos

`parseValue` divide por `10^n` **en la ingesta**, así que cambiar `impliedDecimals` en el esquema deja la colección con dos escalas para el mismo campo y nada lo señala. Es el defecto con riesgo sobre datos, y va último a propósito.

**Files:**
- Modify: `backend/internal/repository/monitor_repo.go` — `rescaleExpression` (pura) y `RescaleDataField`
- Modify: `backend/internal/handlers/monitor_handler.go` — `UpdateSchema`
- Modify: `frontend/src/lib/api/monitors.ts` — `updateSchema`
- Modify: `frontend/src/components/tabs/content/monitor-tab-content.tsx` — `handleSaveSchema` y el diálogo
- Test: `backend/internal/repository/rescale_test.go` (nuevo — el paquete no tiene tests hoy; este cubre solo la función pura)
- Test: `backend/internal/handlers/monitor_schema_test.go` (nuevo)

**Interfaces:**
- Produces: `func rescaleExpression(field string, delta int) bson.D`; `func (r *MonitorRepository) RescaleDataField(ctx context.Context, collectionID, field string, delta int, cutoff time.Time) (int64, error)`; `func cambiosDeEscala(actual, nuevo []models.SchemaField) map[string]int`.
- El `PUT /monitors/:id/schema` acepta `rescaleExisting bool` además de `schema`, y puede responder **409**.

**Cómo se testea el contrato.** Los tests de `handlers` en este repositorio ejercitan funciones puras (`canReadDashboard`, por ejemplo), no endpoints contra una base. Así que la decisión del contrato se extrae a `cambiosDeEscala`, que es la única parte con lógica: una vez que sabe qué campos cambiaron y cuánto, las cuatro filas de la tabla son ramas triviales sobre ese resultado y el conteo de registros. Esas ramas se verifican en el Step 10, a mano.

- [ ] **Step 1: Escribir los tests que fallan**

Crear `backend/internal/repository/rescale_test.go`:

```go
package repository

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

// Son montos: la conversión tiene que usar potencias de diez enteras. Con
// `$multiply` por 0.1, 5000 da 500.00000000000006 en float64.
func TestRescaleExpression_UsaPotenciasEnteras(t *testing.T) {
	casos := []struct {
		nombre   string
		delta    int
		operador string
		factor   int64
	}{
		{"de 2 a 3 decimales: el valor se achica", 1, "$divide", 10},
		{"de 0 a 2 decimales", 2, "$divide", 100},
		{"de 2 a 0 decimales: el valor se agranda", -2, "$multiply", 100},
		{"de 3 a 2 decimales", -1, "$multiply", 10},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			expr := rescaleExpression("importe", c.delta)

			raw, err := bson.Marshal(expr)
			if err != nil {
				t.Fatalf("no serializa: %v", err)
			}
			var m bson.M
			if err := bson.Unmarshal(raw, &m); err != nil {
				t.Fatalf("no deserializa: %v", err)
			}

			operandos, ok := m[c.operador].(bson.A)
			if !ok {
				t.Fatalf("esperaba %s, quedó %#v", c.operador, m)
			}
			if operandos[0] != "$importe" {
				t.Errorf("primer operando = %v, quería $importe", operandos[0])
			}
			if got, ok := operandos[1].(int64); !ok || got != c.factor {
				t.Errorf("factor = %#v, quería el entero %d", operandos[1], c.factor)
			}
		})
	}
}

// delta 0 no es un cambio de escala: no hay nada que convertir y devolver
// una expresión igual sería reescribir toda la colección para nada.
func TestRescaleExpression_DeltaCeroEsNil(t *testing.T) {
	if expr := rescaleExpression("importe", 0); expr != nil {
		t.Errorf("con delta 0 esperaba nil, quedó %#v", expr)
	}
}
```

- [ ] **Step 2: Correr el test y ver que falla**

```bash
go test -C backend ./internal/repository/ -run TestRescaleExpression -v
```
Expected: FAIL con `undefined: rescaleExpression`.

- [ ] **Step 3: Escribir la expresión y el método del repositorio**

Agregar a `backend/internal/repository/monitor_repo.go`, junto a `BackfillDataField`:

```go
// rescaleExpression arma la conversión de un campo entre dos escalas de
// decimales implícitos. delta = decimalesNuevos - decimalesViejos:
//
//	delta > 0  el valor guardado se achica  → dividir por 10^delta
//	delta < 0  el valor guardado se agranda → multiplicar por 10^-delta
//
// Siempre por una potencia de diez ENTERA, nunca multiplicando por su
// recíproco: son montos, y `5000 * 0.1` en float64 da 500.00000000000006,
// mientras que `5000 / 10` da 500 exacto.
func rescaleExpression(field string, delta int) bson.D {
	if delta == 0 {
		return nil
	}

	op := "$divide"
	exp := delta
	if delta < 0 {
		op = "$multiply"
		exp = -delta
	}

	factor := int64(1)
	for i := 0; i < exp; i++ {
		factor *= 10
	}

	return bson.D{{Key: op, Value: bson.A{"$" + field, factor}}}
}

// RescaleDataField convierte un campo numérico entre dos escalas de
// decimales implícitos, en una sola operación del servidor.
//
// Solo toca documentos ingeridos hasta `cutoff`: los posteriores ya se
// guardaron con la configuración nueva y convertirlos otra vez los dejaría
// mal. El llamador toma el cutoff ANTES de escribir el esquema, de modo que
// la carrera que queda —una carga concurrente en la ventana de milisegundos
// entre ambos— deje una fila en la escala vieja, que se ve al mirar los
// datos, en vez de una convertida dos veces, que no se distingue de un monto
// legítimo.
//
// Los documentos sin el campo, o donde no es numérico, se dejan como están.
func (r *MonitorRepository) RescaleDataField(
	ctx context.Context,
	collectionID string,
	field string,
	delta int,
	cutoff time.Time,
) (int64, error) {
	expr := rescaleExpression(field, delta)
	if expr == nil {
		return 0, nil
	}

	res, err := r.GetDataCollection(collectionID).UpdateMany(ctx,
		bson.M{
			field:          bson.M{"$type": "number"},
			"_ingested_at": bson.M{"$lte": cutoff},
		},
		mongo.Pipeline{
			{{Key: "$set", Value: bson.D{{Key: field, Value: expr}}}},
		},
	)
	if err != nil {
		return 0, fmt.Errorf("rescaling %s: %w", field, err)
	}
	return res.ModifiedCount, nil
}
```

Agregar `"time"` a los imports del archivo si no está.

- [ ] **Step 4: Correr el test**

```bash
go test -C backend ./internal/repository/ -run TestRescaleExpression -v
```
Expected: PASS.

- [ ] **Step 5: El test de `cambiosDeEscala`, que falla**

Crear `backend/internal/handlers/monitor_schema_test.go`:

```go
package handlers

import (
	"testing"

	"github.com/thureos/compliance/internal/models"
)

func TestCambiosDeEscala(t *testing.T) {
	actual := []models.SchemaField{
		{Name: "importe", ImpliedDecimals: 2},
		{Name: "comision", ImpliedDecimals: 0},
		{Name: "tarjeta"},
	}

	casos := []struct {
		nombre   string
		nuevo    []models.SchemaField
		esperado map[string]int
	}{
		{"sin cambios", actual, map[string]int{}},
		{"un campo suma decimales", []models.SchemaField{
			{Name: "importe", ImpliedDecimals: 3},
			{Name: "comision", ImpliedDecimals: 0},
			{Name: "tarjeta"},
		}, map[string]int{"importe": 1}},
		{"un campo los saca", []models.SchemaField{
			{Name: "importe", ImpliedDecimals: 0},
			{Name: "comision", ImpliedDecimals: 0},
			{Name: "tarjeta"},
		}, map[string]int{"importe": -2}},
		{"dos campos a la vez", []models.SchemaField{
			{Name: "importe", ImpliedDecimals: 4},
			{Name: "comision", ImpliedDecimals: 2},
			{Name: "tarjeta"},
		}, map[string]int{"importe": 2, "comision": 2}},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			got := cambiosDeEscala(actual, c.nuevo)
			if len(got) != len(c.esperado) {
				t.Fatalf("cambios = %v, quería %v", got, c.esperado)
			}
			for campo, delta := range c.esperado {
				if got[campo] != delta {
					t.Errorf("delta de %q = %d, quería %d", campo, got[campo], delta)
				}
			}
		})
	}
}
```

Run: `go test -C backend ./internal/handlers/ -run TestCambiosDeEscala -v`
Expected: FAIL con `undefined: cambiosDeEscala`.

- [ ] **Step 6: El contrato del handler**

En `backend/internal/handlers/monitor_handler.go`, agregar el helper junto a `schemaSinCampoDerivado`:

```go
// cambiosDeEscala devuelve, por campo, cuánto cambian sus decimales
// implícitos entre el esquema guardado y el que se está por guardar. El
// delta es `nuevo - viejo`, que es lo que espera RescaleDataField. Los
// campos que no cambian no aparecen: un mapa vacío significa que no hay
// nada que convertir.
func cambiosDeEscala(actual, nuevo []models.SchemaField) map[string]int {
	viejos := make(map[string]int, len(actual))
	for _, f := range actual {
		viejos[f.Name] = f.ImpliedDecimals
	}
	cambios := make(map[string]int)
	for _, f := range nuevo {
		if delta := f.ImpliedDecimals - viejos[f.Name]; delta != 0 {
			cambios[f.Name] = delta
		}
	}
	return cambios
}
```

Run: `go test -C backend ./internal/handlers/ -run TestCambiosDeEscala -v`
Expected: PASS.

Y dentro de `UpdateSchema`:

Ampliar el body:

```go
	var body struct {
		Schema []models.SchemaField `json:"schema"`
		// RescaleExisting confirma que se conviertan los documentos ya
		// guardados. Sin él, un cambio de decimales sobre un monitor con
		// datos se rechaza en vez de dejar dos escalas en la colección.
		RescaleExisting bool `json:"rescaleExisting"`
	}
```

Después de la validación de `DateFormat` que ya está, y **antes** de guardar, agregar:

```go
	// El schema enviado ya se validó arriba: tiene exactamente los mismos
	// campos que el actual, así que comparar por nombre es seguro.
	cambios := cambiosDeEscala(monitor.Schema, body.Schema)

	if len(cambios) > 0 && !body.RescaleExisting {
		registros, err := h.monitorRepo.GetDataCollection(monitor.CollectionID).
			CountDocuments(c.Context(), bson.M{})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if registros > 0 {
			campos := make([]string, 0, len(cambios))
			for name := range cambios {
				campos = append(campos, name)
			}
			sort.Strings(campos)
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": fmt.Sprintf(
					"cambiar los decimales implícitos de %s deja los %d registros ya cargados en la escala vieja; mandá rescaleExisting para convertirlos",
					strings.Join(campos, ", "), registros,
				),
				"fields":  campos,
				"records": registros,
			})
		}
	}

	// El cutoff se toma ANTES de escribir el esquema: ver RescaleDataField.
	cutoff := time.Now()
```

Y después de que `h.monitorRepo.Update` haya guardado el esquema:

```go
	rescaled := map[string]int64{}
	for name, delta := range cambios {
		if !body.RescaleExisting {
			continue
		}
		n, err := h.monitorRepo.RescaleDataField(c.Context(), monitor.CollectionID, name, delta, cutoff)
		if err != nil {
			// Los campos ya convertidos quedan convertidos: no hay rollback.
			// Reintentar la misma petición los convertiría de nuevo, así que
			// el error dice explícitamente qué se completó.
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":    fmt.Sprintf("el esquema se guardó pero falló el reescalado de %q: %v. Revisá los datos antes de reintentar: los campos ya convertidos no se deshacen.", name, err),
				"rescaled": rescaled,
			})
		}
		rescaled[name] = n
	}

	return c.JSON(fiber.Map{"schema": monitor.Schema, "rescaled": rescaled})
```

Agregar los imports que falten (`sort`, `strings`, `time`).

- [ ] **Step 7: Correr la suite del backend**

```bash
go test -C backend ./...
```
Expected: verde. Si algún test de handlers llamaba a `UpdateSchema` cambiando decimales sobre un monitor con datos, ahora recibe 409: es el contrato nuevo, y el test hay que actualizarlo mandando `rescaleExisting`.

- [ ] **Step 8: El cliente de API**

En `frontend/src/lib/api/monitors.ts`:

```ts
  updateSchema: (id: string, schema: SchemaField[], rescaleExisting = false) =>
    api.put<{ schema: SchemaField[]; rescaled?: Record<string, number> }>(
      `/monitors/${id}/schema`,
      { schema, rescaleExisting },
    ),
```

- [ ] **Step 9: El diálogo de confirmación**

En `frontend/src/components/tabs/content/monitor-tab-content.tsx`, agregar el estado y separar el guardado en dos:

```tsx
  const [confirmarReescalado, setConfirmarReescalado] = useState<
    { campo: string; de: number; a: number }[] | null
  >(null);
```

```tsx
  /** Campos cuyos decimales implícitos cambiaron respecto del esquema guardado. */
  function cambiosDeEscala() {
    if (!monitor) return [];
    return schemaEdits
      .map((f) => {
        const previo = monitor.schema.find((x) => x.name === f.name);
        const de = previo?.impliedDecimals ?? 0;
        const a = f.impliedDecimals ?? 0;
        return de === a ? null : { campo: f.name, de, a };
      })
      .filter((x): x is { campo: string; de: number; a: number } => x !== null);
  }

  async function handleSaveSchema() {
    // Cambiar los decimales no recalcula lo ya guardado: sin este aviso, la
    // colección queda con dos escalas del mismo campo y nada lo señala.
    const cambios = cambiosDeEscala();
    if (cambios.length > 0 && (monitor?.recordCount ?? 0) > 0) {
      setConfirmarReescalado(cambios);
      return;
    }
    await guardarEsquema(false);
  }

  async function guardarEsquema(rescaleExisting: boolean) {
    setSavingSchema(true);
    try {
      const result = await monitorsApi.updateSchema(id, schemaEdits, rescaleExisting);
      setMonitor((prev) => (prev ? { ...prev, schema: result.schema } : prev));
      const convertidos = Object.values(result.rescaled ?? {}).reduce((a, b) => a + b, 0);
      toastSuccess(
        convertidos > 0
          ? `Esquema actualizado — ${convertidos} registros convertidos`
          : "Esquema actualizado",
      );
    } catch {
      toastError("No se pudo guardar el esquema");
    } finally {
      setSavingSchema(false);
      setConfirmarReescalado(null);
    }
  }
```

Y el diálogo, junto a los `AlertDialog` que el archivo ya usa:

```tsx
      <AlertDialog
        open={confirmarReescalado !== null}
        onOpenChange={(open) => !open && setConfirmarReescalado(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Convertir los datos ya cargados</AlertDialogTitle>
            <AlertDialogDescription asChild>
              <div className="space-y-2">
                <p>
                  Este monitor tiene {monitor?.recordCount ?? 0} registros
                  guardados en la escala actual. Al cambiar los decimales
                  implícitos, sus valores se convierten:
                </p>
                <ul className="space-y-1">
                  {(confirmarReescalado ?? []).map((c) => (
                    <li key={c.campo} className="text-xs">
                      <span className="font-medium">{c.campo}</span>
                      {`: ${c.de} → ${c.a} decimales`}
                    </li>
                  ))}
                </ul>
                <p className="font-medium">Esta operación no se deshace sola.</p>
              </div>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancelar</AlertDialogCancel>
            <AlertDialogAction onClick={() => guardarEsquema(true)}>
              Convertir y guardar
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
```

- [ ] **Step 10: Correr todo**

```bash
go test -C backend ./...
cd frontend && npx tsc --noEmit && npm run lint && npm run test:run
```

- [ ] **Step 11: Verificación manual — es la que importa**

Con backend y frontend levantados, sobre un monitor de prueba nuevo (**no** sobre uno con datos que importen):

1. Cargar un archivo con un campo numérico, por ejemplo `importe` con valor `500000`.
2. En Esquema, ponerle 2 decimales implícitos y Guardar. Confirmar que aparece el diálogo con "1 registro" y "importe: 0 → 2 decimales".
3. Confirmar. En Mongo, el valor tiene que ser `5000`.
4. Volver a Esquema, pasarlo a 3 decimales, confirmar. El valor tiene que ser `500`.
5. Volver a 0 decimales, confirmar. El valor tiene que volver a `500000` **exacto** — sin `500000.00000000006`. Este paso es el que verifica que la conversión usa potencias enteras.
6. Cambiar el nombre de nada y guardar el esquema sin tocar decimales: no debe aparecer el diálogo.
7. Sobre un monitor **sin** registros, cambiar decimales: tampoco debe aparecer.

- [ ] **Step 12: Commit**

```bash
git add backend/internal/repository/monitor_repo.go backend/internal/repository/rescale_test.go backend/internal/handlers/monitor_handler.go backend/internal/handlers/monitor_schema_test.go frontend/src/lib/api/monitors.ts frontend/src/components/tabs/content/monitor-tab-content.tsx
git commit -m "feat(monitores): convertir los datos al cambiar los decimales implícitos"
```

---

## Puesta en marcha en producción

No requiere código y va después de desplegar. Sobre el monitor `CBCG Tarjetas`:

1. Configurar el timestamp derivado: `fechapoliza` (`YYYYMMDD`) + `horapoliza` (`HHMMSS`) → `timestamp`, y correr el backfill.
2. Rearmar la regla viva —*"Múltiples cargos en casino superiores a 5000 USD en menos de 35 segundos"*— como condición de velocidad sobre `timestamp`, agrupada por tarjeta, gap de 35s y filtro por `mcc`, en vez del agregado con ventana de 60s sobre `fechapoliza`, que no puede disparar.
3. Verificar que el umbral de `importe` esté en la escala guardada: el campo tiene 2 decimales implícitos.
4. Ejecutar la regla y confirmar qué alertas genera.
