# Plan de implementación — Roadmap P1 (analítica, CI/CD, detección avanzada)

**Spec:** `docs/superpowers/specs/2026-09-07-roadmap-p1-analitica-design.md`
**Secuencia:** Fase 1 (Tareas 1-2) → Fase 2 (Tareas 3-5) → Fase 3 (Tareas 6-7)
**Regla de trabajo:** cada tarea sale con su test (TDD rojo→verde) donde el código lo permite; el gate (`procoder check`) debe estar limpio antes de cerrar cada tarea. La Tarea 7 es descubrimiento, no implementación — no tiene test.

---

## Fase 1

### Tarea 1 — CI/CD: workflow de GitHub Actions

**Archivos:** `.github/workflows/ci.yml` (nuevo), `backend/internal/services/notification_service.go` (fix trivial), `backend/internal/services/rule_templates.go` (fix trivial).

1. Corregir los 2 hallazgos de `golangci-lint` que hoy existen (string de error capitalizada en `notification_service.go:230`; variable `pMaxTx7d` no usada en `rule_templates.go:122`) — prerrequisito para que el primer PR nazca en verde.
2. Escribir `ci.yml` con dos jobs paralelos:
   - `backend`: `go build ./...`, `go vet ./...`, `go test ./...`, `gofmt -l .` (falla si la salida no está vacía), `golangci-lint run ./...`. Sin `services:` (nada en la suite actual toca Mongo/Redis en vivo).
   - `frontend`: `npm ci`, `npm run lint`, `npm run test:run`, `npm run build`.
   - Trigger: `pull_request` contra `master`; agregar también `push` a `master` para que el propio PR que introduce el workflow tenga cómo mostrarse en verde antes de que exista protección de branch.
3. Abrir como PR (no push directo) para validar el flujo real; una vez el workflow corre en verde contra `master` (mergeado o en push directo mientras no hay protección todavía), pasar a activar:
4. Branch protection en `master`: requerir PR, requerir que los checks `backend` y `frontend` pasen. Verificar con `gh api repos/:owner/:repo/branches/master/protection` que quedó activa.
5. Sin test automatizado (es configuración); verificación: el Action corre y muestra verde en GitHub para un PR de prueba.

### Tarea 2 — Analítica de efectividad de reglas (backend)

**Archivos:** `backend/internal/models/rule.go` (campo nuevo), `backend/internal/handlers/rule_template_handler.go` (persistir `template_id`), `backend/internal/handlers/rule_handler.go` (endpoint nuevo), `backend/internal/repository/rule_repo.go` o servicio nuevo `backend/internal/services/rule_effectiveness.go`, `backend/internal/router/router.go`, tests.

1. Agregar `TemplateID string` (bson `template_id,omitempty`) a `models.Rule`. `Instantiate` en `rule_template_handler.go` lo setea con `req.TemplateID` al crear la regla.
2. `rule_effectiveness.go`: función que agrega sobre `red_flags` filtrando por `rule_id` (o por `template_id` vía join con `rules`): total, desglose por `disposition` (incluye "abierta" para las sin `closed_at`), tasa de falsos positivos sobre las cerradas, tiempo medio de cierre.
3. `GET /api/v1/rules/:id/effectiveness` — agregación por regla individual.
4. `GET /api/v1/rule-templates/effectiveness` — misma agregación agrupada por `template_id`.
5. Tests: agregación con red flags de ejemplo (mezcla de disposiciones y algunas sin cerrar); tasa de falsos positivos calculada solo sobre cerradas; regla sin ninguna red flag no rompe (devuelve ceros, no error); `Instantiate` persiste `template_id`.

### Tarea 3 — Analítica de efectividad de reglas (frontend)

**Archivos:** `frontend/src/lib/api/rules.ts` (cliente), `frontend/src/app/(dashboard)/rules/page.tsx` (sección nueva, mínima), componente aislado si la tabla crece (`frontend/src/components/rules/effectiveness-table.tsx`).

1. Cliente API para los dos endpoints de la Tarea 2.
2. Tabla ordenable por % de falsos positivos en `rules/page.tsx`: regla, disparos, % falsos positivos, tiempo medio de cierre.
3. Sin gráfico nuevo — una tabla resuelve comparar reglas mejor que un chart para este caso de uso.

---

## Fase 2

### Tarea 4 — Backtesting de reglas

**Archivos:** `backend/internal/services/rule_engine.go` (parámetro dry-run en `EvaluateRuleForDateRange` o wrapper), `backend/internal/handlers/rule_handler.go` (endpoint), `backend/internal/router/router.go`, `frontend/src/lib/api/rules.ts`, UI en el formulario de regla existente, tests.

1. `EvaluateRuleForDateRange` gana un modo dry-run (parámetro o variante) que corre la misma lógica de matching pero no llama a `redFlagRepo.Upsert`, no incrementa `trigger_count`, no dispara `triggerReportGeneration` ni `triggerNotification` — devuelve el conteo y una muestra (primeros 20) de matches.
2. `POST /api/v1/monitors/:id/rules/backtest` recibe el cuerpo de una regla (misma forma que `CreateRuleRequest`) + rango de fechas opcional (default: todo el histórico ingerido del monitor); no persiste nada.
3. Frontend: botón "Probar contra histórico" en el formulario de creación/edición de regla (manual y desde plantilla) → muestra conteo + tabla de muestra antes de guardar.
4. Tests: backtest no persiste red flag ni incrementa `trigger_count` ni encola notificación; conteo correcto contra un dataset de monitor de prueba con condiciones simples y agregadas.

### Tarea 5 — Escalamiento automático de SLA

**Archivos:** `backend/internal/models/red_flag.go` (campo `escalation_notified_at`), `backend/internal/services/rule_engine.go` (fijar `sla_due_at` al crear), `backend/internal/handlers/red_flag_handler.go` (`AssignCase` deja de setearlo), `backend/internal/services/sla_escalation.go` (nuevo, cron), `backend/cmd/server/main.go` (wiring del cron), tests.

1. `models.RedFlag` gana `EscalationNotifiedAt *time.Time`.
2. En `rule_engine.go`, en el punto donde ya se decide `isNew` (mismo lugar que `triggerNotification`/`triggerReportGeneration`), setear `sla_due_at = SLADueAt(rf)` en el red flag antes de persistir. `AssignCase` (`red_flag_handler.go:443`) deja de calcularlo/setearlo — ya llega seteado desde la creación.
3. `sla_escalation.go`: cron (mismo patrón que `Scheduler`/`MonitorPuller`, corre cada minuto/hora — a definir el intervalo al implementar) que busca red flags con `sla_due_at < now()`, `status` no terminal, `escalation_notified_at` nulo. Para cada una: encola `JobKindNotify` con un payload de tipo `sla_breach` (reusa el worker existente, no un job nuevo), marca `escalation_notified_at = now()`.
4. Wiring en `main.go`: arrancar el cron igual que `scheduler`/`monitorPuller`.
5. Tests: `sla_due_at` queda seteado al crear (no requiere assign); el filtro del cron trae exactamente las vencidas+no-notificadas+no-cerradas; correr el cron dos veces sobre la misma red flag no re-notifica.

---

## Fase 3

### Tarea 6 — Correlación filtrada en agregados (alcance acotado)

**Archivos:** `backend/internal/models/rule.go` (`Filter` en `AggregateCondition`), `backend/internal/services/rule_engine.go` (`buildAggregatePipeline`, `buildAggregatePipelineDateScoped`), tests.

1. Agregar `Filter []Condition` (opcional) a `models.AggregateCondition`.
2. En `buildAggregatePipeline`/`buildAggregatePipelineDateScoped`: si `Filter` no está vacío, agregar un `$match` con `BuildMongoFilter(ConditionGroup{Logic: AND, Conditions: Filter})` **antes** del `$group` existente (después del filtro de ventana temporal si lo hay). Reglas existentes sin `Filter` no cambian de comportamiento — verificar con los tests actuales de agregación en verde.
3. Tests: agregado con `Filter` que reduce el universo antes de agrupar (ej. solo montos en una franja); agregado sin `Filter` se comporta idéntico a antes (regresión).
4. **No incluida en esta tarea** (queda para cuando el negocio la pida): tipología "cuenta dormida→activa" (necesita expresar una condición temporal de inactividad previa que el modelo de `Condition` actual no cubre) y visualización de grafo de contrapartes (requiere resolución de entidades que no existe hoy). Ambas se retoman con su propia sesión de brainstorming si el negocio las prioriza.

### Tarea 7 — Sanciones (OFAC/ONU): descubrimiento, no implementación

**Archivos:** ninguno todavía.

1. Sesión de brainstorming aparte (no parte de este plan) que resuelva primero: qué listas son obligatorias para los mercados objetivo, mecanismo de origen de datos (archivo público descargado periódicamente vs. API de proveedor pago), frecuencia de actualización, y quién en el negocio es dueño de esa decisión.
2. Sin tarea de código hasta que lo anterior esté resuelto.

---

## Criterio de cierre del lote

- **Fase 1:** PR de prueba mergeado con ambos checks en verde; branch protection activa en `master`; tabla de efectividad visible en `/rules` con datos reales de casos ya cerrados en este entorno.
- **Fase 2:** backtest de una regla contra el histórico de un monitor de prueba muestra un conteo coherente sin dejar rastro en `red_flags`; una red flag nueva sin asignar tiene `sla_due_at` desde su creación; forzar el reloj (o crear una red flag con severidad `critical` y esperar) dispara exactamente una notificación de escalamiento.
- **Fase 3 (Tarea 6 solamente — Tarea 7 no cierra en este lote):** una regla con `Filter` en su condición agregada solo cuenta lo que el filtro deja pasar; el gate (Go tests + `procoder check`) sin bloqueantes.
