# Plan de implementación — Roadmap P0 AML

**Spec:** `docs/superpowers/specs/2026-09-06-roadmap-p0-aml-design.md`
**Secuencia:** A3 (tipologías) → A4 (notificaciones) → A1 (case management)
**Regla de trabajo:** cada tarea sale con su test (TDD rojo→verde); el gate (`procoder check`) debe estar limpio antes de cerrar cada tarea.

---

## Tarea 1 — Catálogo de tipologías y binding (A3, backend)

**Archivos:** `backend/internal/services/rule_templates.go` (nuevo), `backend/internal/services/rule_templates_test.go` (nuevo).

1. Definir tipos: `RuleTemplate{ID, Name, Description, SuggestedSeverity, Placeholders, Rule RuleTemplateRule}`, con `RuleTemplateRule` portando la forma parametrizada de la regla (condiciones simples, grupo AND/OR, condiciones agregadas con ventana) y placeholders `$AMOUNT`, `$DATE`, `$ACCOUNT`, `$COUNTERPART`, `$COUNTRY`, `$MCC`, `$THRESHOLD_*`.
2. Escribir el catálogo v1 con 10 tipologías (ver spec).
3. Implementar `BindTemplate(t RuleTemplate, fieldMap map[string]string, params map[string]float64) (models.Rule, error)`:
   - rechazar placeholder sin mapeo (error con lista de faltantes);
   - rechazar columna vacía;
   - sustituir campos en condiciones simples y agregadas, y valores paramétricos (umbrales) desde `params` con default del template;
   - devolver `models.Rule` lista para persistir (sin ID ni timestamps — eso lo pone el repo).
4. Tests de binding: mapeo completo feliz; placeholder faltante; columna vacía; params ausentes usan defaults; la regla resultante pasa la validación existente de `Create`.

## Tarea 2 — Endpoints de tipologías (A3, backend)

**Archivos:** `backend/internal/handlers/rule_template_handler.go` (nuevo), `backend/internal/router/router.go` (rutas), tests en `backend/internal/handlers/`.

1. `GET /api/v1/rule-templates` → catálogo (autenticado).
2. `POST /api/v1/monitors/:id/rules/from-template` → body `{templateId, fieldMap, params, name}`; valida monitor, valida esquema detectado (columnas de `fieldMap` existen en el schema del monitor), llama `BindTemplate`, persiste vía el servicio de reglas existente con rol `compliance`+.
3. Tests: instanciación feliz contra monitor con esquema; 400 por columna inexistente; 400 por template inexistente; 403 por rol viewer.

## Tarea 3 — Galería de plantillas (A3, frontend)

**Archivos:** `frontend/src/components/rules/template-gallery.tsx` (nuevo), `frontend/src/lib/api/rules.ts` (clientes API), integración mínima en `rules/page.tsx` (botón "Desde plantilla" + render del componente).

1. Modal de galería: tarjeta por tipología (nombre, descripción, severidad sugerida).
2. Al elegir: formulario de instanciación — un select de columna por placeholder (opciones = columnas del esquema del monitor elegido), inputs numéricos para umbrales con defaults, nombre de la regla.
3. Submit → `from-template` → refresh de la lista de reglas + toast de éxito.
4. Componente aislado; nada de la lógica entra a `rules/page.tsx` salvo el botón.

## Tarea 4 — Config de notificaciones (A4, backend)

**Archivos:** `backend/internal/services/notification_service.go` (nuevo), `backend/internal/services/system_config_repo.go` (campos nuevos), tests.

1. Extend `system_config`: `smtp{host,port,username,password,from}`, `webhooks[]{url,secret,enabled}`.
2. `NotificationService`: `NotifyRedFlag(ctx, rf)` → arma payload, para cada canal habilitado encola job `notify` en la cola Redis existente.
3. Firmador HMAC-SHA256 (`X-Thureos-Signature`) con el secreto del webhook.
4. Render del email (plantilla de texto: monitor, regla, severidad, resumen, link).

## Tarea 5 — Worker de notificaciones + disparo (A4, backend)

**Archivos:** `backend/internal/services/worker.go`, `backend/internal/services/red_flag_service.go` (o donde nazca la red flag), `backend/internal/services/job_queue.go`, tests.

1. Nuevo tipo de job en la cola: `notify` con payload (canales ya resueltos + datos del red flag).
2. El worker procesa `notify`: envía email y/o POST webhook, respeta retries (3) y dead queue.
3. Disparo solo cuando `Upsert` crea red flag **nueva** (el bool `isNew` que ya devuelve) — cablear después de la creación.
4. Tests: job feliz; fallo SMTP → reintento; agotamiento → dead queue; upsert de red flag existente NO encola.

## Tarea 6 — UI de notificaciones (A4, frontend)

**Archivos:** `frontend/src/app/(dashboard)/settings/page.tsx` (sección nueva), `frontend/src/lib/api/settings.ts`.

1. Sección "Notificaciones": SMTP (host, puerto, usuario, from) + lista de webhooks (URL, secreto, enabled) con agregar/quitar.
2. Botón "Probar" que encola una notificación de prueba.
3. Solo visible para admin.

## Tarea 7 — Modelo de casos (A1, backend)

**Archivos:** `backend/internal/models/red_flag.go` (campos), `backend/internal/repository/red_flag_repo.go`, `backend/internal/repository/red_flag_note_repo.go` (nuevo), tests de repo.

1. Nuevos campos en el modelo: `assignee_id`, `priority`, `sla_due_at`, `disposition`, `closed_at`, `closed_by`.
2. `red_flag_notes`: colección nueva, Create + List por `red_flag_id` ordenado por `created_at`. Sin Update ni Delete (inmutabilidad).
3. Índices: `red_flag_id` + `created_at` en notas; `assignee_id` en red flags.
4. Cálculo de `sla_due_at` desde severidad al crear (config por severidad en system_config).

## Tarea 8 — Workflow y API de casos (A1, backend)

**Archivos:** `backend/internal/services/case_service.go` (nuevo), `backend/internal/handlers/red_flag_handler.go` (endpoints), `backend/internal/router/router.go`, tests.

1. Máquina de estados: `nuevo → en_investigacion → escalada → cerrada` (transiciones legales codificadas; todo lo demás es 400).
2. Endpoints: `POST /red-flags/:id/assign`, `POST /red-flags/:id/notes`, `POST /red-flags/:id/transition` (con `disposition` obligatoria al cerrar), `GET /red-flags/:id` con notas incluidas.
3. Cada transición escribe en el activity log existente.
4. Tests: transiciones legales/ilegales; cierre sin disposición rechazado; nota en estado `nuevo` permitida; auditoría escrita.

## Tarea 9 — Vista de caso (A1, frontend)

**Archivos:** `frontend/src/app/(dashboard)/red-flags/[id]/page.tsx` (nuevo), `frontend/src/lib/api/red-flags.ts`, ajustes en `red-flags/page.tsx` (links + filtros).

1. Detalle: datos del red flag + timeline de notas + acciones (asignar a mí/otro, transicionar, cerrar con disposición en modal).
2. Lista: columna de asignado, badge de SLA vencido, filtros "mías" y "SLA vencido".
3. Rutas protegidas por rol (compliance+ para transicionar; viewer solo lee).

## Criterio de cierre del lote

- Gate `procoder check` sin bloqueantes; suites Go y vitest en verde.
- Recorrido manual: crear regla desde tipología → provocar red flag → recibir webhook/email → asignar → investigar con notas → cerrar con disposición.
