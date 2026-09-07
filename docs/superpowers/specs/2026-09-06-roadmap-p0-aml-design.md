# Roadmap P0 AML — tipologías, notificaciones y case management

**Fecha:** 2026-09-06
**Estado:** Aprobado para desarrollo (secuencia A3 → A4 → A1)
**Contexto:** Análisis competitivo del rubro AML/monitoreo transaccional (enterprise: Actimize, SAS, Oracle Mantas; mid/SMB: Verafin, Napier, Tookitaki, ComplyAdvantage; LatAm: Gesintel, Snap, KYC Systems, Pirani). El consenso del mercado es un stack de 4 módulos: TM con escenarios + screening de listas + case management + reportes regulatorios. Thureos cubre TM y analítica; este lote cierra los huecos que no dependen de decidir país/regulador.

**Objetivo de negocio (decisión del dueño, 2026-09-06):** vender a bancos, fintechs y entidades supervisadas a nivel **mundial**. Implicaciones: listas de screening globales (OFAC/ONU/UE/HMT), framework regulatorio multi-jurisdicción (no un solo país), multi-tenancy y SaaS suben de prioridad en lotes siguientes.

## Objetivo

Que un oficial de cumplimiento pueda: (1) activar reglas AML estándar en minutos sin diseñarlas, (2) enterarse de las alertas fuera de la app, y (3) investigar una alerta con workflow trazable hasta su disposición.

## Fuera de alcance (lotes siguientes)

- Screening de sanciones/PEP con listas públicas (A2) — requiere decidir fuente de datos.
- Modelo cliente-céntrico y risk scoring (A5/A6) — reordena el modelo de datos; va después.
- Reportes regulatorios por jurisdicción (A7) — bloqueado por la decisión de país.
- Multi-tenancy, grafos (D1/D3).

## A3 — Librería de tipologías AML pre-armadas

**Problema:** las reglas se diseñan a mano. El mercado espera una librería OOTB (Napier 100+, Tookitaki 400+). El motor de Thureos ya soporta todo lo necesario (operadores, agregaciones, ventanas); falta el catálogo.

**Diseño:**

- Catálogo de tipologías en el backend (`internal/services/rule_templates.go`): structs Go, no DB — el catálogo evoluciona con el código, se versiona con releases.
- Cada tipología declara: id, nombre, descripción (qué patrón de lavado detecta), severidad sugerida, y una **plantilla de regla con placeholders de campo** (`$AMOUNT`, `$DATE`, `$ACCOUNT`, `$COUNTERPART`, `$COUNTRY`, `$MCC`).
- Tipologías v1 (9) — cada una es puramente fila o puramente agregada, porque el engine evalúa ambos por separado y cada artefacto dispara su propia red flag: velocidad de transacciones (count 24h por cuenta), suma acumulada (sum 7d por cuenta), franja de estructuración (monto entre piso y umbral de reporte, por fila), transacción de alto monto, ticket promedio anómalo (avg 30d), pico de máximo (max 30d), país de alto riesgo (lista FATF por defecto), MCC de alto riesgo (lista por defecto), concentración en contraparte (sum 30d por contraparte). Las tipologías que exigen correlación filtrada (estructuración clásica, cuenta dormida→activa) quedan fuera hasta que el engine soporte agregados con filtro.
- **Instanciación:** `POST /api/v1/monitors/:id/rules/from-template` recibe `{templateId, fieldMap, params}` donde `fieldMap` une cada placeholder a una columna del esquema detectado del monitor y `params` ajusta umbrales. El handler valida que todos los placeholders estén mapeados a columnas existentes y crea una `Rule` normal (la tipología es azúcar de creación; el runtime no cambia).
- `GET /api/v1/rule-templates` lista el catálogo.
- **Frontend:** galería de plantillas en la página de reglas (nuevo componente aislado, no agrandar `rules/page.tsx`): tarjeta por tipología → modal de instanciación con selects de columnas (del esquema del monitor) y inputs de parámetros con defaults sensatos.

**Testing:** unit tests de la lógica de binding (placeholders→columnas, params→valores, regla resultante válida y persistible); endpoint test de instanciación contra un monitor de ejemplo; la galería no deja rastro en el runtime de evaluación.

## A4 — Notificaciones de alertas

**Problema:** el analista debe vivir dentro de la app. Todos los productos del mercado empujan la alerta al canal del usuario.

**Diseño:**

- Dos canales v1: **email** (SMTP configurable) y **webhook genérico** (POST JSON del red flag).
- Configuración por sistema en `system_config` (ya existe la colección y su repo): SMTP host/port/from/auth, lista de webhooks con secreto opcional para firmar (HMAC SHA-256 en header `X-Thureos-Signature`).
- Disparo: al crear una red flag **nueva** (no en upsert de una existente — evita ruido). El envío se encola en la cola Redis existente (nuevo tipo de job) con los retries/dead-letter que ya tiene el worker.
- Contenido del email: monitor, regla, severidad, resumen del match, link al detalle. Webhook: payload completo del red flag.
- UI: sección de notificaciones en settings (admin), con botón de test.

**Testing:** unit tests del firmador HMAC y del render del payload; test del worker procesando el job de notificación (éxito, fallo SMTP → retry, agotamiento → dead queue).

## A1 — Case management sobre red flags

**Problema:** las red flags tienen status (nuevo/reconocida/resuelta) pero no investigación: el mercado exige workflow trazable hasta la disposición (10/10 de los productos analizados).

**Diseño:**

- **Modelo (decisión de diseño: extender el status existente, no duplicar máquinas):** `status` gana el valor `escalated`; el acknowledged ya existente ES "en investigación" (con `acknowledged_by` como asignado, renombrado en UI). Campos nuevos en `red_flags`: `assignee_id`, `priority` (deriva de severidad, editable), `sla_due_at` (defaults en código por severidad: critical 24h, high 72h, medium 7d, low 30d; configurabilidad llega con el framework regulatorio), `disposition` (enum al cerrar: `false_positive`, `confirmed_ros`, `no_action`), `closed_at`, `closed_by`. Nueva colección `red_flag_notes` (`red_flag_id`, `author_id`, `texto`, `created_at`) — timeline inmutable, sin edición ni borrado (trazabilidad regulatoria).
- **Workflow:** `new → acknowledged → escalated → (acknowledged o cerrada)`; cierre desde cualquier estado activo con `disposition` obligatoria; estados cerrados son terminales. Toda transición escribe en el activity log existente (quién, cuándo, qué).
- **API:** `POST /red-flags/:id/assign`, `POST /red-flags/:id/notes`, `POST /red-flags/:id/transition` (valida transiciones legales), `GET /red-flags/:id` enriquecido con notas+timeline.
- **Frontend:** vista de detalle de red flag (nueva página) con timeline de notas, panel de acción (asignar, transicionar, cerrar con disposición) y conteo de SLA vencido en la lista. Cola de trabajo: filtro "mías" + "SLA vencido".
- **Reglas de negocio:** un usuario no se reasigna un caso que otro investiga sin dejar nota; el cierre con `confirmed_ros` queda marcado como semilla del futuro módulo de reportes regulatorios (A7).

**Testing:** unit tests de la máquina de transiciones (estados legales/ilegales), tests de repo para notas (inmutabilidad, orden), endpoint tests del ciclo completo assign→note→transition→close, test de SLA (cálculo de due date por severidad).

## Decisiones de diseño

1. **Catálogo en código, no en DB:** las tipologías son producto, no dato del cliente; versionan con el binario y no requieren migración.
2. **La tipología no toca el runtime:** se instancia a una `Rule` estándar — cero cambios en el evaluador, riesgo acotado al flujo de creación.
3. **Notificación solo en red flag nueva:** el upsert por fingerprint no renota; evita tormentas de email por un mismo patrón.
4. **Notas inmutables:** en compliance el timeline es evidencia; sin editar/borrar jamás.
5. **Webhook firmado (HMAC):** cualquier consumidor externo puede validar el origen; secreto por webhook en config.
