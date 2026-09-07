# Roadmap P1 — analítica, automatización de ingeniería y detección avanzada

**Fecha:** 2026-09-07
**Estado:** Aprobado para desarrollo (secuencia: Fase 1 → Fase 2 → Fase 3)
**Contexto:** continuación del lote P0 (`2026-09-06-roadmap-p0-aml-design.md`, ya en producción). El dueño pidió llevar el producto "a la vanguardia" y priorizar las seis líneas identificadas; se acordó la secuencia por fases en el chat de brainstorming del 2026-09-07.

## Objetivo

Tres cosas independientes que conviene resolver juntas porque comparten infraestructura:

1. **Ingeniería:** que un cambio roto no llegue a `master` sin que alguien lo vea antes de mergear.
2. **Analítica:** que el oficial de cumplimiento (y quien diseña reglas) sepa qué reglas generan señal real vs. ruido, sin tener que leer casos uno por uno.
3. **Detección:** cerrar los huecos que el propio lote P0 dejó explícitos — escalamiento de SLA que hoy no dispara nada, y la correlación filtrada que el motor de reglas no soporta.

## Fuera de alcance (este lote)

- **Sanciones (Fase 3, ítem 6):** solo se especifica como tarea de descubrimiento, no de implementación — la fuente de datos (¿archivo OFAC SDN descargado manualmente? ¿API de un proveedor de terceros? ¿qué listas además de OFAC?) es una decisión de negocio que no está tomada. Ver la sección propia más abajo.
- Deploy/CD real (que el pipeline despliegue a un servidor) — Fase 1 es CI (build/test/lint/gate), no CD. `docker-compose.prod.yml` y los Dockerfiles ya existen de un lote anterior pero conectarlos a un target de deploy es una decisión de infraestructura aparte.
- Configurar `system_config` para los defaults de SLA por severidad (hoy hardcodeados en `SLADefaultForSeverity`) — no se toca; el gap real es que no todo red flag los recibe, no que no sean configurables.
- Cualquier cambio a `system_config` para umbrales de sanciones/scoring — depende de la Fase 3 fuera de alcance.

---

## Fase 1, ítem 1 — CI/CD (GitHub Actions + branch protection)

**Problema:** no existe `.github/workflows/`. La única red de seguridad hoy es que alguien corre build/test/lint/gate a mano antes de pushear — el hallazgo de SSRF de la sesión anterior lo agarró una revisión manual *después* del commit, no un gate antes de mergear.

**Diseño:**

- Un solo workflow (`.github/workflows/ci.yml`) con dos jobs paralelos, `backend` y `frontend`, disparado en `pull_request` contra `master` (y opcionalmente en push directo a `master`, para cubrir el propio pipeline mientras no haya branch protection activa todavía).
  - `backend`: `go build ./...`, `go vet ./...`, `go test ./...`, `gofmt -l .` (falla si devuelve algo), `golangci-lint run ./...`.
  - `frontend`: `npm ci`, `npm run lint`, `npm run test:run`, `npm run build`.
- **Sin contenedores de servicio (Mongo/Redis) en el pipeline:** verificado en esta sesión que ningún test actual toca Mongo/Redis en vivo — todo lo que depende de Mongo (repos, casi todos los handlers) no tiene test unitario, y lo que sí tiene test (services, algunos handlers) usa mocks o `httptest`. Si eso cambia en el futuro, ahí se agregan los `services:` del job.
- **Sin secrets:** confirmado que el repo no tiene ninguno configurado y ninguna suite los necesita hoy (`ANTHROPIC_API_KEY` no se usa en tests).
- **`procoder check` NO entra al pipeline:** es un binario Mach-O arm64 local (`~/.local/bin/procoder`) con estado en `.procoder/` (index, state) atado a esta máquina — no es portable a un runner de GitHub tal cual está. Queda como gate local (yo lo sigo corriendo antes de cada push, como en las sesiones anteriores), documentado explícitamente para que no se lea como un olvido.
- **Orden de activación (importante):** primero el workflow se mergea y corre en verde contra `master` un par de veces; recién después se activa "Require status checks to pass" en la protección de `master`. Activar la protección antes de tener el workflow verde bloquearía el propio push que lo agrega.
- **Branch protection en `master`:** requiere PR (no push directo), status checks `backend` y `frontend` en verde, y al menos 1 review (o "no reviews requeridas" si el equipo es de una sola persona — a definir con el dueño al implementar, no bloquea el diseño).
- **Prerrequisito de implementación:** los 2 hallazgos triviales de `golangci-lint` detectados en esta sesión (`notification_service.go:230` string de error capitalizada, `rule_templates.go:122` variable no usada) se corrigen como parte de esta tarea — si no, el primer PR nace en rojo.

**Testing:** no aplica TDD tradicional (es config, no código de producto); la verificación es que el workflow corra y quede en verde contra el estado actual de `master` antes de activar branch protection.

## Fase 1, ítem 2 — Analítica de efectividad de reglas

**Problema:** cada caso cerrado ya guarda `disposition` (`false_positive` / `confirmed_ros` / `no_action`), pero no hay ninguna vista que la agregue. Es la métrica más básica de un motor AML — qué reglas generan señal real vs. ruido — y los datos ya existen.

**Diseño:**

- **No es un widget del dashboard configurable existente:** los widgets (`bar_chart`, `stat`, etc.) agregan sobre la colección de datos ingeridos de un monitor (`data_<monitor_id>`), no sobre `red_flags`/`rules`. Forzar esta métrica en ese sistema significaría extender el motor de widgets para una necesidad completamente distinta. Se hace una vista dedicada en su lugar.
- **Endpoint nuevo:** `GET /api/v1/rules/:id/effectiveness` → por regla: total de red flags generadas, desglose por `disposition` (incluye "sin cerrar" para las que siguen abiertas), tasa de falsos positivos (`false_positive` / cerradas con disposition), tiempo medio de cierre (`closed_at - created_at`). Agregación por `rule_id` sobre `red_flags`, sin nueva colección.
- **`GET /api/v1/rule-templates/effectiveness`** (o un query param en el endpoint anterior): la misma agregación pero agrupada por tipología en vez de por regla individual — para eso, **`Rule` necesita un campo `template_id`** que hoy no existe (`Instantiate` en `rule_template_handler.go` no lo setea). Se agrega como tarea explícita de esta fase, no como afterthought: sin él, no se puede saber qué reglas nacieron de qué tipología para agregar por typology.
- **Frontend:** sección "Efectividad" dentro de `rules/page.tsx` (tabla: regla, disparos, % falsos positivos, tiempo medio de cierre) — ordenable por % de falsos positivos para que salte a la vista qué reglas generan ruido. Sin gráfico nuevo tipo "dashboard"; una tabla resuelve el caso de uso (comparar reglas) mejor que un chart.

**Testing:** unit tests de la agregación (falsos positivos, sin cerrar, tiempo de cierre) con datos de red_flags de ejemplo; test de que `Instantiate` persiste `template_id`.

---

## Fase 2, ítem 3 — Backtesting de reglas

**Problema:** una regla nueva se activa a ciegas. No hay forma de ver cuántas alertas hubiera generado antes de prenderla — el motor de efectividad de la Fase 1 mide *después* del hecho; esto mide *antes*.

**Diseño:**

- **`POST /api/v1/monitors/:id/rules/backtest`** recibe el cuerpo de una regla (misma forma que `CreateRuleRequest`, sin persistir) más un rango de fechas opcional (default: todo el histórico del monitor). Corre la misma lógica de evaluación que `EvaluateRuleForDateRange` (reutilizada, no reimplementada) pero en modo *dry run*: no llama a `redFlagRepo.Upsert`, no dispara notificación ni reporte, solo cuenta matches y devuelve una muestra (primeros N) para que el usuario juzgue si son señal o ruido.
- **Reutiliza la Fase 1 conceptualmente:** el resultado del backtest se puede comparar contra la tasa de falsos positivos de reglas similares ya activas (mismo campo, funciones de agregación parecidas) para dar contexto — "esta regla habría generado 340 alertas; reglas comparables activas tienen 12% de falsos positivos" — pero esa comparación es una mejora de UX, no un bloqueante: el backtest funciona solo con el conteo crudo.
- **Frontend:** botón "Probar contra histórico" en el formulario de creación/edición de regla (tanto manual como desde plantilla) → muestra conteo + tabla de muestra antes de guardar.

**Testing:** unit test de que el backtest no persiste nada (ni red flag, ni notificación, ni incremento de `trigger_count`); test de que cuenta correctamente contra un dataset de monitor de prueba.

## Fase 2, ítem 4 — Escalamiento automático de SLA

**Problema:** `sla_due_at` se calcula (`SLADueAt()`, pura función de `created_at` + severidad) pero **solo se persiste cuando alguien asigna el caso** (`AssignCase` en `red_flag_handler.go:443`). Una red flag nueva sin asignar no tiene `sla_due_at` en la base — el badge de "SLA vencido" en el frontend nunca puede aparecer para ella, y nada dispara nada cuando el plazo pasa.

**Diseño:**

- **Corrección de raíz:** `sla_due_at` se calcula y persiste al **crear** la red flag (en el mismo punto donde ya se decide `isNew` en `rule_engine.go`), no al asignar. `AssignCase` deja de setearlo — ya está seteado desde el nacimiento del caso.
- **Job periódico de escalamiento** (mismo patrón que `Scheduler`/`MonitorPuller`: un cron que corre cada minuto/hora): busca red flags con `sla_due_at < now()`, estado no terminal, y `escalation_notified_at` nulo (campo nuevo — evita re-notificar en cada tick). Para cada una, encola una notificación (reusa `JobKindNotify` y el worker existente) con un tipo de payload distinto (`type: "sla_breach"` en vez de `"red_flag.created"`) y marca `escalation_notified_at`.
- **No escala el `status`** (no lo mueve a `escalated` automáticamente) — el estado `escalated` sigue siendo una decisión humana del analista; esto solo *avisa* que el plazo venció, no toca el workflow.

**Testing:** unit test de que `sla_due_at` queda seteado en la creación (no en el assign); unit test del filtro del job (vencidas + no notificadas + no cerradas → sí; el resto → no); test de que no re-notifica dos veces la misma red flag.

---

## Fase 3, ítem 5 — Correlación filtrada / grafo de contrapartes

**Problema:** el propio spec de P0 lo dejó explícito: "las tipologías que exigen correlación filtrada (estructuración clásica, cuenta dormida→activa) quedan fuera hasta que el engine soporte agregados con filtro". Es la pieza que falta para detectar smurfing/estructuración real entre múltiples cuentas — hoy el motor agrega sin poder decir "solo cuento transacciones donde el monto está entre X e Y" dentro del propio agregado.

**Diseño (nivel de alcance, no implementación — el detalle fino se define en la fase de planning cuando le toque el turno):**

- Extender `AggregateCondition` con un `Filter []Condition` opcional: condiciones que se aplican **antes** de agrupar/agregar (un `$match` adicional en el pipeline de Mongo, antes del `$group` que ya existe en `buildAggregatePipeline`). Esto es aditivo — las reglas existentes sin `Filter` siguen funcionando idéntico.
- Con eso resuelto, se pueden agregar las tipologías que P0 dejó afuera: estructuración clásica (monto filtrado a la franja justo debajo del umbral de reporte, agregado por cuenta) y cuenta dormida→activa (filtro por inactividad previa, requiere una condición temporal que el modelo actual no expresa — **esta es la que puede necesitar más que un `Filter` simple**, queda marcada para resolver en el planning).
- El grafo de contrapartes en sentido literal (visualización de red, no solo agregación) es un salto más grande — de "sumar por contraparte" a "encontrar estructuras de N cuentas intermedias" — y no está claro que el modelo de datos actual (transacciones planas por monitor) alcance sin un paso de resolución de entidades primero. Se deja explícitamente para evaluar cuando llegue el turno de esta fase, no se diseña a ciegas ahora.

## Fase 3, ítem 6 — Screening de sanciones (OFAC/ONU) — solo descubrimiento

**No se diseña en este documento.** Es el único ítem de los seis donde el contrato de datos externo es una incógnita real (¿qué listas, con qué frecuencia de actualización, vía qué mecanismo — descarga de archivo público, API paga de un proveedor, ambas?) y una decisión de negocio (compliance/legal define qué listas son obligatorias para los mercados donde se vende). Cuando le toque el turno, la sesión de brainstorming para este ítem arranca por esas preguntas, no por el diseño técnico.

---

## Decisiones de diseño

1. **`procoder check` no entra a CI:** es una herramienta local no portable; el gate de CI cubre lo que sí es portable (build/test/lint) y `procoder` se sigue corriendo a mano como hasta ahora.
2. **Efectividad de reglas es una vista dedicada, no un widget:** los widgets del dashboard configurable agregan sobre datos de monitor, no sobre metadata de reglas/casos — son sistemas distintos con objetivos distintos.
3. **`template_id` en `Rule`:** sin él, la analítica por tipología (a diferencia de por regla individual) es imposible. Se agrega en la Fase 1, no se pospone.
4. **Backtesting reutiliza `EvaluateRuleForDateRange`:** una sola implementación de "cómo se evalúa una regla contra un rango de fechas", con un flag de dry-run, en vez de una segunda copia de la lógica de evaluación.
5. **SLA se fija al crear, no al asignar:** es un plazo objetivo (desde cuándo existe el problema), no depende de si alguien ya lo tomó — asignar tarde no debería mover el reloj.
6. **El escalamiento notifica, no transiciona estado:** mover `status` a `escalated` es una decisión humana; el job solo se asegura de que alguien se entere de que el plazo venció.
7. **Fase 3 se especifica a nivel de alcance, no de implementación:** tanto el grafo de contrapartes como sanciones tienen incógnitas reales (modelo de datos para el primero, fuente de datos/decisión de negocio para el segundo) que conviene resolver cuando les toque el turno, con información más fresca, en vez de comprometerse ahora.
