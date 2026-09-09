# Fase 3 — IA: velocidad, validación menos agresiva y modo guiado

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Que la generación de reglas por IA deje de descartar sugerencias válidas en silencio, sepa expresar condiciones de velocidad y filtros previos, y acepte que el usuario le señale los campos antes de describir el criterio.

**Architecture:** Todo el cambio de backend vive en `ai_rules_service.go`: la validación pasa de "cualquier campo vacío es un campo desconocido" a "solo se valida lo que está presente", el motivo de cada descarte se devuelve al frontend en vez de perderse en un `log.Printf`, y el prompt del sistema aprende `velocityConditions` y `filter`. La validación de velocidad no se reescribe: se reusa `ValidateVelocityCondition`, que ya rechaza lo que el motor no puede evaluar. En el frontend, el diálogo de IA suma un selector de campos con el mismo patrón de chips que ya usa "Campos a screenear" y muestra el motivo real de cada descarte.

**Tech Stack:** Go 1.26 / Fiber v2 / MongoDB · Next.js 16 / React 19 / TypeScript / shadcn-ui

**Spec:** `docs/superpowers/specs/2026-09-09-reglas-de-velocidad-y-timestamp-derivado-design.md` (sección "Pieza 3 — IA", líneas 196-209; fases en 240-245)

## Global Constraints

- **Copy de usuario en español**, con acentos correctos. Los nombres de campo, operadores y claves JSON quedan en inglés.
- **El `systemPrompt` se escribe en inglés**, igual que hoy, salvo la instrucción existente de que `name`/`description`/`reasoning` salgan en español. No hay selector de idioma en el producto.
- **Unidades válidas de tiempo para la IA: `s`, `min`, `h`, `d`.** `m` está deliberadamente fuera (`validAITimeWindow`, `ai_rules_service.go:303`): es ambiguo entre minutos y meses.
- **`VelocityCondition.MinEvents` solo admite 2.** `ValidateVelocityCondition` rechaza cualquier otro valor; el motor mide pares, no rachas.
- **Ningún componente de frontend escribe un color literal**: tokens (`bg-canvas`, `text-ink-muted`) o `@/lib/semantic-colors`.
- **Aditivo en todos los casos.** Una sugerencia sin `velocityConditions`, una petición sin `fields` y un monitor sin campos de fecha tienen que comportarse exactamente como hoy.
- Cada tarea termina con `go test ./...` (desde `backend/`) y, si tocó frontend, `npx tsc --noEmit` + `npm run lint` + `npm run test:run` (desde `frontend/`) en verde antes del commit.

---

## Estructura de archivos

| Archivo | Responsabilidad | Tareas |
|---|---|---|
| `backend/internal/services/ai_rules_service.go` | Prompt, llamada al proveedor, parseo y validación de sugerencias | 1, 2, 3, 4 |
| `backend/internal/services/ai_rules_service_test.go` | Tests de validación (ya existe, se extiende) | 1, 2, 3, 4 |
| `backend/internal/handlers/rule_handler.go` | Endpoint `POST /rules/ai-generate` | 2, 4 |
| `backend/internal/models/rule.go` | `AIRuleRequest` | 4 |
| `frontend/src/lib/types.ts` | `AIRuleSuggestion` | 3 |
| `frontend/src/lib/api/rules.ts` | Cliente de `generateAI` | 2, 3, 4 |
| `frontend/src/app/(dashboard)/rules/page.tsx` | Diálogo "Generar con IA" | 2, 3, 4 |

Las cuatro tareas tocan `ai_rules_service.go` en secuencia y **no son paralelizables**: la 2 depende de la firma que introduce la 1, la 3 extiende la validación de la 1, y la 4 extiende el mensaje que las anteriores no tocan. Ejecutar en orden.

---

### Task 1: La validación deja de tratar "vacío" como "campo inexistente"

Un `groupBy` vacío es válido para el motor: `buildAggregatePipeline` (`rule_engine.go:657-661`) usa `_id: null` y agrupa globalmente. Pero `suggestionReferencesUnknownField` hace `!fieldNames[agg.GroupBy]`, y `fieldNames[""]` es `false`, así que **toda sugerencia de agregado global se descarta entera**. Lo mismo pasa con `timeField` vacío (el motor simplemente no aplica ventana, `rule_engine.go:624`) y con `timeWindow` vacío (`validAITimeWindow` exige dígitos + unidad).

Esta tarea reemplaza el booleano por un motivo de descarte en texto, que la Task 2 va a devolver al frontend.

**Files:**
- Modify: `backend/internal/services/ai_rules_service.go:305-344`
- Test: `backend/internal/services/ai_rules_service_test.go`

**Interfaces:**
- Produces: `func discardReason(s AIRuleSuggestion, schema []models.SchemaField) string` — devuelve `""` si la sugerencia es guardable, o el motivo en español. Task 2 lo consume para reportar; Task 3 le agrega las ramas de velocidad.
- `filterValidSuggestions(suggestions []AIRuleSuggestion, schema []models.SchemaField) []AIRuleSuggestion` conserva su firma en esta tarea.

- [ ] **Step 1: Escribir los tests que fallan**

Agregar al final de `backend/internal/services/ai_rules_service_test.go`:

```go
// Un agregado global (groupBy vacío) es válido para el motor: agrupa con
// _id:null. Descartarlo entero era la causa más frecuente de "generé reglas
// y no salió ninguna".
func TestDiscardReason_AceptaAgregadoGlobal(t *testing.T) {
	s := AIRuleSuggestion{
		Name: "Total diario",
		AggregateConditions: []models.AggregateCondition{{
			Field:      "monto",
			Function:   models.AggFuncSum,
			GroupBy:    "",
			TimeField:  "fecha",
			TimeWindow: "24h",
			Operator:   models.OpGreaterThan,
			Threshold:  50000,
		}},
	}
	if got := discardReason(s, testSchema()); got != "" {
		t.Errorf("discardReason = %q, quería aceptarla", got)
	}
}

// Sin timeField ni timeWindow el motor no aplica ventana: es un agregado
// sobre todo el histórico, perfectamente expresable.
func TestDiscardReason_AceptaAgregadoSinVentana(t *testing.T) {
	s := AIRuleSuggestion{
		Name: "Conteo por cliente",
		AggregateConditions: []models.AggregateCondition{{
			Field:     "monto",
			Function:  models.AggFuncCount,
			GroupBy:   "cliente",
			Operator:  models.OpGreaterEqual,
			Threshold: 5,
		}},
	}
	if got := discardReason(s, testSchema()); got != "" {
		t.Errorf("discardReason = %q, quería aceptarla", got)
	}
}

// Media ventana es intención a medio expresar: "5 transacciones en 60s" que
// pierde el "en 60s" se convierte en "5 transacciones alguna vez", que es una
// regla mucho más ruidosa que la pedida. Se descarta.
func TestDiscardReason_DescartaVentanaIncompleta(t *testing.T) {
	s := AIRuleSuggestion{
		Name: "Ventana a medias",
		AggregateConditions: []models.AggregateCondition{{
			Field:     "monto",
			Function:  models.AggFuncCount,
			GroupBy:   "cliente",
			TimeField: "fecha",
			Operator:  models.OpGreaterEqual,
			Threshold: 5,
		}},
	}
	if got := discardReason(s, testSchema()); got == "" {
		t.Error("discardReason = \"\", quería descartarla por ventana incompleta")
	}
}

func TestDiscardReason_DescartaCamposInexistentes(t *testing.T) {
	casos := []struct {
		nombre string
		s      AIRuleSuggestion
	}{
		{"campo de condición inventado", AIRuleSuggestion{
			ConditionGroup: models.ConditionGroup{
				Logic:      "AND",
				Conditions: []models.Condition{{Field: "no_existe", Operator: models.OpGreaterThan, Value: 1}},
			},
		}},
		{"groupBy inventado", AIRuleSuggestion{
			AggregateConditions: []models.AggregateCondition{{
				Field: "monto", Function: models.AggFuncSum, GroupBy: "no_existe",
				Operator: models.OpGreaterThan, Threshold: 1,
			}},
		}},
		{"ventana con unidad ambigua", AIRuleSuggestion{
			AggregateConditions: []models.AggregateCondition{{
				Field: "monto", Function: models.AggFuncSum, GroupBy: "cliente",
				TimeField: "fecha", TimeWindow: "5m",
				Operator: models.OpGreaterThan, Threshold: 1,
			}},
		}},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := discardReason(c.s, testSchema()); got == "" {
				t.Error("discardReason = \"\", quería descartarla")
			}
		})
	}
}
```

- [ ] **Step 2: Correr los tests y ver que fallan**

Run: `go test -C backend ./internal/services/ -run TestDiscardReason -v`
Expected: FAIL con `undefined: discardReason`.

- [ ] **Step 3: Implementar**

En `backend/internal/services/ai_rules_service.go`, reemplazar `suggestionReferencesUnknownField` (líneas 329-344) por:

```go
// discardReason devuelve por qué una sugerencia no se puede guardar, o ""
// si es guardable. Solo se valida lo que está PRESENTE: un groupBy vacío es
// un agregado global (rule_engine.go:657-661, _id:null) y una ventana vacía
// es un agregado sobre todo el histórico (rule_engine.go:624). Tratar el
// vacío como campo inexistente descartaba sugerencias perfectamente válidas
// y dejaba al usuario mirando una lista vacía.
func discardReason(s AIRuleSuggestion, schema []models.SchemaField) string {
	fieldNames := make(map[string]bool, len(schema))
	for _, f := range schema {
		fieldNames[f.Name] = true
	}

	for _, cond := range s.ConditionGroup.Conditions {
		if !fieldNames[cond.Field] {
			return fmt.Sprintf("la condición usa el campo %q, que no está en el esquema", cond.Field)
		}
	}

	for _, agg := range s.AggregateConditions {
		// Para "count" el motor ignora Field (buildAggExpr → $sum:1), así
		// que puede venir vacío; para el resto es lo que se agrega.
		if agg.Function != models.AggFuncCount && agg.Field == "" {
			return "el agregado no dice qué campo agregar"
		}
		if agg.Field != "" && !fieldNames[agg.Field] {
			return fmt.Sprintf("el agregado usa el campo %q, que no está en el esquema", agg.Field)
		}
		if agg.GroupBy != "" && !fieldNames[agg.GroupBy] {
			return fmt.Sprintf("el agregado agrupa por %q, que no está en el esquema", agg.GroupBy)
		}
		if agg.TimeField != "" && !fieldNames[agg.TimeField] {
			return fmt.Sprintf("el agregado mide el tiempo sobre %q, que no está en el esquema", agg.TimeField)
		}
		// Media ventana es intención a medio expresar: sin el par completo
		// el motor ignora la ventana y "5 en 60s" se vuelve "5 alguna vez".
		if (agg.TimeField == "") != (agg.TimeWindow == "") {
			return "la ventana de tiempo está incompleta: hacen falta el campo de fecha y la duración"
		}
		if agg.TimeWindow != "" && !validAITimeWindow.MatchString(agg.TimeWindow) {
			return fmt.Sprintf("la ventana %q no usa una unidad válida (s, min, h, d)", agg.TimeWindow)
		}
	}

	return ""
}
```

Y reescribir `filterValidSuggestions` (líneas 312-327) para que lo use:

```go
func filterValidSuggestions(suggestions []AIRuleSuggestion, schema []models.SchemaField) []AIRuleSuggestion {
	valid := make([]AIRuleSuggestion, 0, len(suggestions))
	for _, s := range suggestions {
		if reason := discardReason(s, schema); reason != "" {
			log.Printf("ai-rules: descartando sugerencia %q — %s", s.Name, reason)
			continue
		}
		valid = append(valid, s)
	}
	return valid
}
```

El comentario de bloque de `filterValidSuggestions` (líneas 305-311) se mantiene: sigue explicando por qué se descarta entera y no se repara a medias.

- [ ] **Step 4: Correr los tests**

Run: `go test -C backend ./internal/services/ -run 'TestDiscardReason|TestFilterValidSuggestions' -v`
Expected: PASS, incluidos los cuatro tests preexistentes de `TestFilterValidSuggestions_*`.

- [ ] **Step 5: Suite completa y commit**

```bash
go test -C backend ./...
git add backend/internal/services/ai_rules_service.go backend/internal/services/ai_rules_service_test.go
git commit -m "fix(ia): no descartar sugerencias por campos opcionales vacíos"
```

---

### Task 2: Decir cuántas sugerencias se generaron y por qué se descartaron

Hoy `GenerateRules` devuelve solo las válidas y el motivo muere en un `log.Printf` del servidor. El frontend adivina: si la lista vino vacía muestra un texto genérico (`page.tsx:1200-1212`) que siempre culpa a los campos fuera del esquema, aunque el motivo real haya sido otro.

**Files:**
- Modify: `backend/internal/services/ai_rules_service.go:108-145`, `305-327`
- Modify: `backend/internal/handlers/rule_handler.go:396-401`
- Modify: `frontend/src/lib/api/rules.ts:49-54`
- Modify: `frontend/src/app/(dashboard)/rules/page.tsx:722-744`, `1200-1212`
- Test: `backend/internal/services/ai_rules_service_test.go`

**Interfaces:**
- Consumes: `discardReason(s, schema) string` (Task 1).
- Produces: `AIGenerationResult{Suggestions []AIRuleSuggestion, Generated int, Discarded []DiscardedSuggestion}` y `DiscardedSuggestion{Name, Reason string}`. `GenerateRules` pasa a devolver `(AIGenerationResult, error)`. Task 4 le agrega un parámetro de entrada, no cambia la salida.

- [ ] **Step 1: Escribir el test que falla**

Agregar a `backend/internal/services/ai_rules_service_test.go`:

```go
// Cuando todo se descarta, la respuesta tiene que poder explicar por qué:
// el usuario ve terminar "Generando..." y necesita saber qué corregir.
func TestPartitionSuggestions_ReportaGeneradasYDescartadas(t *testing.T) {
	suggestions := []AIRuleSuggestion{
		{
			Name: "Válida",
			ConditionGroup: models.ConditionGroup{
				Logic:      "AND",
				Conditions: []models.Condition{{Field: "monto", Operator: models.OpGreaterThan, Value: 10000}},
			},
		},
		{
			Name: "Campo inventado",
			ConditionGroup: models.ConditionGroup{
				Logic:      "AND",
				Conditions: []models.Condition{{Field: "no_existe", Operator: models.OpGreaterThan, Value: 1}},
			},
		},
	}

	got := partitionSuggestions(suggestions, testSchema())

	if got.Generated != 2 {
		t.Errorf("Generated = %d, quería 2", got.Generated)
	}
	if len(got.Suggestions) != 1 || got.Suggestions[0].Name != "Válida" {
		t.Errorf("Suggestions = %+v, quería solo la válida", got.Suggestions)
	}
	if len(got.Discarded) != 1 {
		t.Fatalf("Discarded = %+v, quería una", got.Discarded)
	}
	if got.Discarded[0].Name != "Campo inventado" || got.Discarded[0].Reason == "" {
		t.Errorf("Discarded[0] = %+v, quería el nombre y un motivo no vacío", got.Discarded[0])
	}
}
```

- [ ] **Step 2: Correr el test y ver que falla**

Run: `go test -C backend ./internal/services/ -run TestPartitionSuggestions -v`
Expected: FAIL con `undefined: partitionSuggestions`.

- [ ] **Step 3: Implementar el backend**

En `ai_rules_service.go`, agregar los tipos junto a `AIRuleSuggestion` (después de la línea 37):

```go
// DiscardedSuggestion es una sugerencia que la IA devolvió y el backend no
// puede guardar, con el motivo listo para mostrarle al usuario.
type DiscardedSuggestion struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// AIGenerationResult acompaña a las sugerencias válidas con lo que hizo
// falta descartar. Sin esto el frontend solo sabe que la lista vino vacía y
// tiene que inventar una explicación genérica.
type AIGenerationResult struct {
	Suggestions []AIRuleSuggestion    `json:"suggestions"`
	Generated   int                   `json:"generated"`
	Discarded   []DiscardedSuggestion `json:"discarded,omitempty"`
}
```

Reemplazar `filterValidSuggestions` por `partitionSuggestions` (mismo lugar, líneas 312-327 tras la Task 1):

```go
func partitionSuggestions(suggestions []AIRuleSuggestion, schema []models.SchemaField) AIGenerationResult {
	result := AIGenerationResult{
		Suggestions: make([]AIRuleSuggestion, 0, len(suggestions)),
		Generated:   len(suggestions),
	}
	for _, s := range suggestions {
		if reason := discardReason(s, schema); reason != "" {
			log.Printf("ai-rules: descartando sugerencia %q — %s", s.Name, reason)
			result.Discarded = append(result.Discarded, DiscardedSuggestion{Name: s.Name, Reason: reason})
			continue
		}
		result.Suggestions = append(result.Suggestions, s)
	}
	return result
}
```

Cambiar la firma de `GenerateRules` (línea 108) y sus tres retornos de error a `AIGenerationResult{}`:

```go
func (s *AIRulesService) GenerateRules(ctx context.Context, schema []models.SchemaField, dataSample string, userPrompt string) (AIGenerationResult, error) {
```

Los `return nil, ...` de las líneas 112, 116, 134, 136 y 141 pasan a `return AIGenerationResult{}, ...`, y el retorno final (línea 144) a:

```go
	return partitionSuggestions(suggestions, schema), nil
```

Los tests preexistentes `TestFilterValidSuggestions_*` pasan a llamar a `partitionSuggestions(...).Suggestions`: renombrar las llamadas, no los tests.

En `backend/internal/handlers/rule_handler.go`, líneas 396-401:

```go
	result, err := h.aiRulesService.GenerateRules(c.Context(), monitor.Schema, req.DataSample, req.Prompt)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(result)
```

- [ ] **Step 4: Implementar el frontend**

En `frontend/src/lib/api/rules.ts`, reemplazar el tipo de retorno de `generateAI` (líneas 49-54):

```ts
  generateAI: (data: {
    monitorId: string;
    prompt: string;
    dataSample?: string;
  }) => api.post<AIGenerationResult>("/rules/ai-generate", data),
```

y agregar al final del archivo, junto a `BacktestResult`:

```ts
export interface DiscardedSuggestion {
  name: string;
  reason: string;
}

export interface AIGenerationResult {
  suggestions: AIRuleSuggestion[];
  generated: number;
  discarded?: DiscardedSuggestion[];
}
```

En `frontend/src/app/(dashboard)/rules/page.tsx`, agregar el estado junto a `aiSuggestions` (línea 438):

```tsx
  const [aiDiscarded, setAiDiscarded] = useState<DiscardedSuggestion[]>([]);
```

(importar `DiscardedSuggestion` desde `@/lib/api/rules`), y reescribir `generateAIRules` (líneas 722-744):

```tsx
  async function generateAIRules() {
    if (!selectedMonitor) return;
    setAILoading(true);
    setAiNoResults(false);
    setAiDiscarded([]);
    try {
      const result = await rulesApi.generateAI({
        monitorId: selectedMonitor,
        prompt: aiPrompt || "Generate monitoring rules for anomaly detection",
      });
      setAiSuggestions(result.suggestions);
      setAiDiscarded(result.discarded ?? []);
      setAiNoResults(result.suggestions.length === 0);
    } catch (err) {
      console.error("Failed to generate AI rules:", err);
      toastError(
        err instanceof Error ? err.message : "Error al generar reglas con IA",
      );
    } finally {
      setAILoading(false);
    }
  }
```

Y reemplazar el bloque `aiNoResults` (líneas 1200-1212) por uno que muestre el motivo real:

```tsx
                  {aiNoResults && !aiLoading && (
                    <div className="rounded-md border border-dashed p-4">
                      <p className="text-sm font-medium">
                        La IA no devolvió ninguna sugerencia válida
                      </p>
                      {aiDiscarded.length > 0 ? (
                        <ul className="mt-2 space-y-1">
                          {aiDiscarded.map((d, i) => (
                            <li key={i} className="text-xs text-muted-foreground">
                              <span className="font-medium">{d.name || "Sin nombre"}</span>
                              {": "}
                              {d.reason}
                            </li>
                          ))}
                        </ul>
                      ) : (
                        <p className="mt-1 text-xs text-muted-foreground">
                          El modelo no devolvió ninguna regla. Probá describir el
                          criterio con los nombres exactos de los campos.
                        </p>
                      )}
                    </div>
                  )}
```

- [ ] **Step 5: Correr todo y commitear**

```bash
go test -C backend ./...
cd frontend && npx tsc --noEmit && npm run lint && npm run test:run && cd ..
git add backend/internal/services/ai_rules_service.go backend/internal/services/ai_rules_service_test.go backend/internal/handlers/rule_handler.go frontend/src/lib/api/rules.ts "frontend/src/app/(dashboard)/rules/page.tsx"
git commit -m "feat(ia): la respuesta dice qué se descartó y por qué"
```

---

### Task 3: El prompt aprende `velocityConditions` y `filter`

El motor sabe detectar "dos transacciones de la misma tarjeta separadas por menos de 35 segundos, ambas de casino" desde la Fase 1, y la UI lo sabe editar desde la Fase 2. La IA no: su vocabulario termina en `aggregateConditions`, así que ese caso —el que motivó todo el rediseño— no lo puede generar.

**Files:**
- Modify: `backend/internal/services/ai_rules_service.go:29-37` (struct), `43-94` (prompt), `discardReason`
- Modify: `frontend/src/lib/types.ts:412-420`
- Modify: `frontend/src/app/(dashboard)/rules/page.tsx:746-767`
- Test: `backend/internal/services/ai_rules_service_test.go`

**Interfaces:**
- Consumes: `discardReason(s, schema) string` (Task 1); `NormalizeVelocityConditions(conds)` y `ValidateVelocityCondition(cond, schema) error` de `velocity_validate.go` — ya existen, **no se reimplementan**: rechazan `timeField` que no sea de tipo `date`, `maxGap` no parseable, `groupBy` fuera del esquema y `minEvents != 2`.
- Produces: `AIRuleSuggestion.VelocityConditions []models.VelocityCondition`.

- [ ] **Step 1: Escribir los tests que fallan**

Agregar a `backend/internal/services/ai_rules_service_test.go`:

```go
// El caso que motivó el rediseño: dos transacciones de la misma tarjeta a
// menos de 35s, filtradas por rubro. Si la IA lo devuelve bien armado, tiene
// que sobrevivir a la validación.
func TestDiscardReason_AceptaVelocidadValida(t *testing.T) {
	s := AIRuleSuggestion{
		Name: "Casino en ráfaga",
		VelocityConditions: []models.VelocityCondition{{
			TimeField: "fecha",
			MaxGap:    "35s",
			GroupBy:   "cliente",
			MinEvents: 2,
			Filter:    []models.Condition{{Field: "monto", Operator: models.OpGreaterThan, Value: 5000}},
		}},
	}
	if got := discardReason(s, testSchema()); got != "" {
		t.Errorf("discardReason = %q, quería aceptarla", got)
	}
}

// minEvents omitido vale 2 (NormalizeVelocityConditions): pedirle a la IA que
// lo repita sería exigirle un detalle que el propio formulario completa solo.
func TestDiscardReason_AceptaVelocidadSinMinEvents(t *testing.T) {
	s := AIRuleSuggestion{
		VelocityConditions: []models.VelocityCondition{{
			TimeField: "fecha", MaxGap: "60s", GroupBy: "cliente",
		}},
	}
	if got := discardReason(s, testSchema()); got != "" {
		t.Errorf("discardReason = %q, quería aceptarla", got)
	}
}

// El motor mide diferencias de tiempo: un campo que no es date no matchea
// nada y la regla no dispararía nunca. Es el bug de Date-contra-string.
func TestDiscardReason_DescartaVelocidadInvalida(t *testing.T) {
	casos := []struct {
		nombre string
		cond   models.VelocityCondition
	}{
		{"campo de tiempo que no es date", models.VelocityCondition{
			TimeField: "cliente", MaxGap: "35s", GroupBy: "cliente", MinEvents: 2,
		}},
		{"gap no parseable", models.VelocityCondition{
			TimeField: "fecha", MaxGap: "un rato", GroupBy: "cliente", MinEvents: 2,
		}},
		{"filtro sobre campo inexistente", models.VelocityCondition{
			TimeField: "fecha", MaxGap: "35s", GroupBy: "cliente", MinEvents: 2,
			Filter: []models.Condition{{Field: "no_existe", Operator: models.OpEqual, Value: 1}},
		}},
		{"racha en vez de par", models.VelocityCondition{
			TimeField: "fecha", MaxGap: "35s", GroupBy: "cliente", MinEvents: 5,
		}},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			s := AIRuleSuggestion{VelocityConditions: []models.VelocityCondition{c.cond}}
			if got := discardReason(s, testSchema()); got == "" {
				t.Error("discardReason = \"\", quería descartarla")
			}
		})
	}
}

// El filtro previo de un agregado se valida igual que el de velocidad.
func TestDiscardReason_DescartaFiltroDeAgregadoInexistente(t *testing.T) {
	s := AIRuleSuggestion{
		AggregateConditions: []models.AggregateCondition{{
			Field: "monto", Function: models.AggFuncSum, GroupBy: "cliente",
			TimeField: "fecha", TimeWindow: "24h",
			Operator: models.OpGreaterThan, Threshold: 1,
			Filter: []models.Condition{{Field: "no_existe", Operator: models.OpEqual, Value: 1}},
		}},
	}
	if got := discardReason(s, testSchema()); got == "" {
		t.Error("discardReason = \"\", quería descartarla")
	}
}
```

- [ ] **Step 2: Correr los tests y ver que fallan**

Run: `go test -C backend ./internal/services/ -run 'TestDiscardReason_.*Velocidad|TestDiscardReason_DescartaFiltro' -v`
Expected: FAIL — `unknown field VelocityConditions in struct literal`, y el test del filtro pasa en falso hasta que se agregue la rama.

- [ ] **Step 3: Extender el struct y la validación**

En `ai_rules_service.go`, agregar el campo a `AIRuleSuggestion` (después de la línea 33):

```go
	VelocityConditions  []models.VelocityCondition  `json:"velocityConditions,omitempty"`
```

En `discardReason`, dentro del bucle de agregados, después de la validación de la ventana:

```go
		if reason := filterFieldsReason(agg.Filter, fieldNames, "el filtro del agregado"); reason != "" {
			return reason
		}
```

y antes del `return ""` final:

```go
	// La validación de velocidad no se duplica: ValidateVelocityCondition ya
	// rechaza todo lo que el motor no puede evaluar — campo de tiempo que no
	// es date, gap no parseable, agrupación fuera del esquema, minEvents != 2
	// y filtro sobre campos inexistentes. Es la MISMA función que corre al
	// guardar la regla, así que una sugerencia que pasa acá se puede aplicar.
	for _, vc := range NormalizeVelocityConditions(s.VelocityConditions) {
		if err := ValidateVelocityCondition(vc, schema); err != nil {
			return fmt.Sprintf("la condición de velocidad no es válida: %v", err)
		}
	}
```

Y el helper, junto a `discardReason`. Solo lo usa el filtro de agregados: el
de velocidad ya lo valida `ValidateVelocityCondition` (`velocity_validate.go:70-74`)
y duplicarlo sería código muerto.

```go
// filterFieldsReason valida los campos del filtro previo de un agregado: el
// filtro reduce el universo antes de agrupar, y un campo inexistente lo
// vacía entero en vez de acotarlo.
func filterFieldsReason(filter []models.Condition, fieldNames map[string]bool, que string) string {
	for _, cond := range filter {
		if !fieldNames[cond.Field] {
			return fmt.Sprintf("%s usa el campo %q, que no está en el esquema", que, cond.Field)
		}
	}
	return ""
}
```

> Nota para quien implemente: `ValidateVelocityCondition` recibe `schema`, no el mapa de nombres — por eso `discardReason` toma `schema` y construye el mapa adentro.

- [ ] **Step 4: Enseñarle el formato al prompt**

En el `systemPrompt`, insertar después del bloque `"aggregateConditions"` (línea 67, antes de `"severity"`):

```
  "velocityConditions": [
    {
      "timeField": "field_name",
      "maxGap": "35s|60s|5min",
      "groupBy": "field_name",
      "minEvents": 2,
      "filter": [
        {"field": "field_name", "operator": "op", "value": value}
      ]
    }
  ],
```

Y agregar, después del párrafo que explica `aggregateConditions` (después de la línea 90):

```
Use "velocityConditions" when the user cares about how close two CONSECUTIVE
events of the same entity are — "two transactions on the same card less than
35 seconds apart", "two withdrawals from the same account within a minute".
This is different from "aggregateConditions": there, the window is anchored
to now and you count how many events fall inside it; here, the distance
between one event and the previous one of the same entity is measured.
"timeField" MUST be a field whose type is "date" in the schema you were
given — the engine measures time differences and a non-date field silently
matches nothing. "maxGap" takes the same units as "timeWindow". "minEvents"
is always 2: the engine measures pairs, not streaks. If the schema has no
date field, do not emit "velocityConditions" at all.

Both "aggregateConditions" and "velocityConditions" accept an optional
"filter": a list of conditions that reduces the universe BEFORE grouping or
before pairing. Use it to say "only casino transactions", "only amounts over
5000" — e.g. mcc = 7995 combined with a 35s "maxGap" expresses "two casino
transactions on the same card less than 35 seconds apart".
```

- [ ] **Step 5: Que la sugerencia aplicada conserve la velocidad**

En `frontend/src/lib/types.ts`, agregar a `AIRuleSuggestion` (después de la línea 416):

```ts
  velocityConditions?: VelocityCondition[];
```

En `frontend/src/app/(dashboard)/rules/page.tsx`, dentro de `applyAISuggestion` (línea 753):

```tsx
        aggregateConditions: suggestion.aggregateConditions,
        velocityConditions: suggestion.velocityConditions,
```

`rulesApi.create` ya acepta `velocityConditions` desde la Entrega B: no hay que tocar el cliente.

- [ ] **Step 6: Correr todo y commitear**

```bash
go test -C backend ./...
cd frontend && npx tsc --noEmit && npm run lint && npm run test:run && cd ..
git add backend/internal/services/ai_rules_service.go backend/internal/services/ai_rules_service_test.go frontend/src/lib/types.ts "frontend/src/app/(dashboard)/rules/page.tsx"
git commit -m "feat(ia): generar condiciones de velocidad y filtros previos"
```

---

### Task 4: Modo guiado — elegir los campos antes de describir el criterio

Aditivo: el textarea libre no se toca. Los campos elegidos viajan al prompt como contexto explícito, que es lo que ataca de raíz la causa principal de sugerencias descartadas — la IA inventando nombres de campo.

**Files:**
- Modify: `backend/internal/models/rule.go:196-200`
- Modify: `backend/internal/services/ai_rules_service.go:108-119`, `275-285`
- Modify: `backend/internal/handlers/rule_handler.go:396`
- Modify: `frontend/src/lib/api/rules.ts` (`generateAI`)
- Modify: `frontend/src/app/(dashboard)/rules/page.tsx` (estado + diálogo de IA)
- Test: `backend/internal/services/ai_rules_service_test.go`

**Interfaces:**
- Consumes: `AIGenerationResult` (Task 2).
- Produces: `buildUserMessage(schema, dataSample, userPrompt string, fields []string) string`; `GenerateRules(ctx, schema, dataSample, userPrompt string, fields []string)`; `AIRuleRequest.Fields []string`.

- [ ] **Step 1: Escribir los tests que fallan**

Agregar a `backend/internal/services/ai_rules_service_test.go`:

```go
func TestBuildUserMessage_IncluyeLosCamposElegidos(t *testing.T) {
	msg := buildUserMessage(testSchema(), "", "montos raros", []string{"monto", "cliente"})
	if !strings.Contains(msg, "monto, cliente") {
		t.Errorf("el mensaje no nombra los campos elegidos:\n%s", msg)
	}
	if !strings.Contains(msg, "montos raros") {
		t.Errorf("el mensaje perdió el contexto libre:\n%s", msg)
	}
}

// El modo guiado es aditivo: sin campos elegidos el mensaje tiene que ser
// exactamente el de antes, byte por byte.
func TestBuildUserMessage_SinCamposNoCambia(t *testing.T) {
	conNil := buildUserMessage(testSchema(), "", "hola", nil)
	conVacio := buildUserMessage(testSchema(), "", "hola", []string{})
	if conNil != conVacio {
		t.Errorf("nil y slice vacío dieron mensajes distintos:\n%q\n%q", conNil, conVacio)
	}
	if strings.Contains(conNil, "Fields the user") {
		t.Errorf("agregó la sección de campos sin campos:\n%s", conNil)
	}
}
```

Agregar `"strings"` a los imports del archivo de tests.

- [ ] **Step 2: Correr los tests y ver que fallan**

Run: `go test -C backend ./internal/services/ -run TestBuildUserMessage -v`
Expected: FAIL con `too many arguments in call to buildUserMessage`.

- [ ] **Step 3: Implementar el backend**

En `ai_rules_service.go`, reemplazar `buildUserMessage` (líneas 275-285):

```go
func buildUserMessage(schema []models.SchemaField, dataSample, userPrompt string, fields []string) string {
	schemaJSON, _ := json.Marshal(schema)
	msg := fmt.Sprintf("Schema:\n%s\n", string(schemaJSON))
	if dataSample != "" {
		msg += fmt.Sprintf("\nSample data:\n%s\n", dataSample)
	}
	// Modo guiado: el usuario señaló los campos antes de escribir el
	// criterio. Nombrarlos explícitamente ataca la causa principal de
	// sugerencias descartadas — la IA inventando nombres de campo.
	if len(fields) > 0 {
		msg += fmt.Sprintf(
			"\nFields the user wants this rule to be about: %s. Build the rules around these fields; you may reference other schema fields only if the criteria require it.\n",
			strings.Join(fields, ", "),
		)
	}
	if userPrompt != "" {
		msg += fmt.Sprintf("\nUser context: %s", userPrompt)
	}
	return msg
}
```

Agregar `"strings"` a los imports de `ai_rules_service.go`.

Cambiar la firma de `GenerateRules` (línea 108) y la llamada de la línea 119:

```go
func (s *AIRulesService) GenerateRules(ctx context.Context, schema []models.SchemaField, dataSample string, userPrompt string, fields []string) (AIGenerationResult, error) {
```
```go
	userMessage := buildUserMessage(schema, dataSample, userPrompt, fields)
```

En `backend/internal/models/rule.go`, líneas 196-200:

```go
type AIRuleRequest struct {
	MonitorID  string   `json:"monitorId"`
	Prompt     string   `json:"prompt"`
	DataSample string   `json:"dataSample,omitempty"`
	// Fields es el modo guiado: los campos que el usuario eligió en el
	// diálogo antes de describir el criterio. Vacío = modo libre de siempre.
	Fields []string `json:"fields,omitempty"`
}
```

En `backend/internal/handlers/rule_handler.go`, línea 396:

```go
	result, err := h.aiRulesService.GenerateRules(c.Context(), monitor.Schema, req.DataSample, req.Prompt, req.Fields)
```

- [ ] **Step 4: Implementar el frontend**

En `frontend/src/lib/api/rules.ts`, agregar `fields` al payload de `generateAI`:

```ts
  generateAI: (data: {
    monitorId: string;
    prompt: string;
    dataSample?: string;
    fields?: string[];
  }) => api.post<AIGenerationResult>("/rules/ai-generate", data),
```

En `frontend/src/app/(dashboard)/rules/page.tsx`, agregar el estado junto a `aiPrompt` (línea 441):

```tsx
  const [aiFields, setAiFields] = useState<string[]>([]);
```

Mandarlo en `generateAIRules`, dentro de la llamada a `rulesApi.generateAI`:

```tsx
        fields: aiFields,
```

Y agregar el selector en el diálogo, entre el `<Select>` de monitor y el textarea de contexto (después de la línea 1175), con el mismo patrón de chips que "Campos a screenear" (líneas 1398-1423):

```tsx
                  <div className="space-y-2">
                    <Label>Campos sobre los que querés la regla (opcional)</Label>
                    <p className="text-xs text-muted-foreground">
                      Elegirlos primero evita que la IA invente nombres de
                      campo, que es el motivo más común de que no salga
                      ninguna sugerencia.
                    </p>
                    <div className="flex flex-wrap gap-2">
                      {(monitors.find((m) => m.id === selectedMonitor)?.schema ?? []).map(
                        (f) => {
                          const active = aiFields.includes(f.name);
                          return (
                            <button
                              key={f.name}
                              type="button"
                              onClick={() =>
                                setAiFields((prev) =>
                                  active
                                    ? prev.filter((n) => n !== f.name)
                                    : [...prev, f.name],
                                )
                              }
                              className={`rounded-md border px-2 py-1 text-xs ${
                                active ? "border-primary bg-primary/10" : ""
                              }`}
                            >
                              {f.name}
                            </button>
                          );
                        },
                      )}
                    </div>
                  </div>
```

El `onValueChange` del `<Select>` de monitor (línea 1162) pasa a limpiar la selección, porque los campos de un monitor no existen en otro:

```tsx
                      onValueChange={(v) => {
                        setSelectedMonitor(v);
                        setAiFields([]);
                      }}
```

- [ ] **Step 5: Correr todo y commitear**

```bash
go test -C backend ./...
cd frontend && npx tsc --noEmit && npm run lint && npm run test:run && cd ..
git add backend/internal/models/rule.go backend/internal/services/ai_rules_service.go backend/internal/services/ai_rules_service_test.go backend/internal/handlers/rule_handler.go frontend/src/lib/api/rules.ts "frontend/src/app/(dashboard)/rules/page.tsx"
git commit -m "feat(ia): modo guiado — elegir los campos antes del criterio"
```

---

## Verificación manual (después de la Task 4)

Con backend en `:8080` (`MONGO_URI=mongodb://localhost:27019/thureos_compliance`, `REDIS_URL=redis://localhost:6383`) y frontend en `:3000`, sobre un monitor que tenga un campo de tipo `date` (el timestamp derivado sirve):

1. **Agregado global.** Pedir "reglas sobre el total transado por día". La sugerencia con `groupBy` vacío ahora aparece en vez de desaparecer.
2. **Motivo del descarte.** Pedir algo inexpresable con el esquema ("reglas sobre la geolocalización del cajero"). El cartel lista nombre y motivo por sugerencia, no el texto genérico.
3. **Velocidad.** Elegir los campos `tarjeta`, `timestamp`, `mcc` e `importe`, y escribir: *"transacciones de casino de más de 5000 dólares con menos de 35 segundos entre una y otra"*. La sugerencia tiene que traer `velocityConditions` con `maxGap: "35s"` y un `filter` sobre `mcc`.
4. **Aplicar.** Darle "Aplicar" y verificar en Mongo que la regla guardada tiene `velocity_conditions` con `time_field`, `max_gap`, `group_by`, `min_events: 2` y `filter`.
5. **Ejecutar.** "Ejecutar ahora" y confirmar que genera las red flags de velocidad esperadas.
6. **No regresión.** Generar sin elegir ningún campo: el flujo libre de siempre sigue funcionando igual.

Limpiar el monitor, la regla y las red flags de prueba al terminar.
