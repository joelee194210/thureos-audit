# Screening de sanciones (OFAC/ONU/UE/UK) vía Watchman

**Fecha:** 2026-09-07
**Estado:** Aprobado para desarrollo
**Contexto:** Tarea 7 del roadmap P1 (`2026-09-07-roadmap-p1-analitica-design.md`), que había quedado marcada como "solo descubrimiento" porque dependía de decisiones de negocio no tomadas. Ya están tomadas: el proveedor es **Moov Watchman**, ya desplegado en el VPS de producción (`kyc.benered.com`) y conectado a la misma red Docker (`thureos-compliance-net`) que `docker-compose.prod.yml` de este proyecto ya declara — no hay que levantar infraestructura nueva, es un cliente HTTP a un servicio interno que ya corre con 7 listas cargadas (OFAC SDN, US CSL, US Non-SDN, UK, ONU, FinCEN 311, UE vía OpenSanctions — ~44k entidades) y se actualiza solo por cron en el VPS.

Referencia de implementación: `thureos-main` (producto hermano de la misma familia) ya integra este mismo Watchman contra su propio modelo de `Person` (onboarding KYC). Este spec porta ese patrón — cliente HTTP, clasificación CLEAR/MATCH/REVIEW fail-closed, workflow de descarte con expiración — adaptado a que Thureos Monitors no tiene un modelo `Person`: acá se screenea texto extraído de datos transaccionales con schema variable por monitor.

## Objetivo

Que una red flag que involucra una contraparte identificable dispare automáticamente un screening contra las listas de sanciones, y que un analista pueda además buscar un nombre a mano — sin construir nada de infraestructura de listas, que ya existe y se opera fuera de esta aplicación.

## Fuera de alcance

- Cualquier pipeline de descarga/actualización de listas — Watchman y el cron del VPS ya lo hacen; esta app solo lo consume.
- Proveedores alternativos (Sanctions API, u otro) — Watchman es el único proveedor de v1; el patrón de abstracción de `thureos-main` (`screenPersonWithProvider` con switch) queda documentado como precedente pero no se implementa un segundo proveedor sin necesidad real.
- Un modelo `Person`/KYC — Thureos Monitors sigue siendo un producto de monitoreo transaccional; el screening cuelga de reglas y red flags, no de un onboarding de clientes.

## A — Cliente Watchman y clasificación (backend)

**Diseño:**

- `internal/services/watchman_client.go`: `Search(ctx, name string, opts SearchOptions) (WatchmanSearchResult, error)` — un `GET` a `{watchmanURL}/v2/search?name=&limit=&minMatch=`, con `dateOfBirth` opcional para desambiguar homónimos si el dato está disponible (no lo está en datos transaccionales típicos; el parámetro se deja disponible para cuando sí lo esté). `watchmanURL` sale de `system_config` (nuevo `WatchmanConfig{URL string}` dentro de `SystemConfig`), no de una env var — mismo patrón que SMTP/webhooks: un admin la carga en Settings, no un deploy. Cliente HTTP simple (no el `newSafeHTTPClient` con guard anti-SSRF de `monitor_puller.go`): esa URL la carga un admin de confianza una vez, no la manda un usuario por request como `PullURL` de un monitor — es la misma categoría de config que el host SMTP.
- `internal/services/screening_classify.go`: `ClassifyScreeningOutcome(matches []NormalizedMatch, err error) ScreeningOutcome{Status, StrongestMatch}` — pura, sin Mongo, testeable directo (mismo patrón que `ComputeEffectiveness`). Reglas de clasificación, calcadas de la referencia:
  - error del proveedor → **REVIEW** (fail-closed: un Watchman caído nunca debe leerse como "sin problema")
  - sin matches → **CLEAR**
  - el match más fuerte es EXACT o STRONG (score ≥ 0.85) → **MATCH**
  - el match más fuerte es MEDIUM o WEAK (score < 0.85) → **REVIEW**

**Testing:** unit tests de `ClassifyScreeningOutcome` cubriendo los 4 casos (sin red/Mongo).

## B — Modelo de datos y disparo automático

**Diseño:**

- `models.ScreeningResult`: `Query` (nombre buscado), `RedFlagID` (`*primitive.ObjectID`, nulo si viene de búsqueda manual), `Status` (`CLEAR|MATCH|REVIEW|DISMISSED|FALSE_POSITIVE`), `StrongestMatch`, `Matches []NormalizedMatch` (array embebido — acotado a los ~20 resultados que devuelve Watchman, no necesita colección aparte), `ReviewedBy`/`ReviewedAt`/`ReviewNotes`, `WhitelistExpiresAt *time.Time`, `CreatedAt`. Colección nueva `screening_results`.
- **Descarte con expiración (H-19 de la referencia, confirmado para v1):** un MATCH o REVIEW se puede descartar (`DISMISSED` o `FALSE_POSITIVE`) con nota obligatoria — mismo principio que cerrar un caso exige `disposition` (Tarea 7 del lote P0). El descarte fija `WhitelistExpiresAt = now + 180 días`. Un descarte no es "nunca más" — vence.
- **Reevaluación del descarte vencido:** reusa el patrón exacto de `SLAEscalationJob` (Tarea 5 del P1) — un cron por minuto busca `ScreeningResult` con `status` en (`DISMISSED`,`FALSE_POSITIVE`) y `whitelistExpiresAt < now`, re-corre `Search()` con el mismo `Query`, reclasifica, y si vuelve a dar MATCH/REVIEW lo notifica por el mismo `NotifyPayload` (`Type: "screening_stale"`) y limpia el estado a lo que salió. Sin esto, "expira" no significa nada — nadie vuelve a mirarlo.
- **Disparo automático:** `Rule` gana `ScreeningFields []string` (nombres de columnas del schema del monitor). En `rule_engine.go`, en el mismo punto donde ya se llama `triggerReportGeneration`/`triggerNotification` (justo después de confirmar que el red flag es nuevo), si `rule.ScreeningFields` no está vacío: por cada campo, extrae el valor de `matchedData`, dispara `screenAndPersist(redFlag, field, value)` en su propia goroutine (no bloquea la creación de la alerta, mismo criterio que el reporte PDF), corre `Search()` + `ClassifyScreeningOutcome()`, persiste el `ScreeningResult` linkeado al `red_flag_id`.

**Testing:** unit tests de la extracción de valores desde `matchedData` (campo ausente, valor vacío, tipos no-string) — pura. El disparo end-to-end (Mongo + HTTP a Watchman) se verifica manualmente contra la infra real, mismo criterio que se usó en Tareas 4-6.

## C — Endpoints y búsqueda manual

**Diseño:**

- `POST /api/v1/screening/search` (compliance+): `{name, dateOfBirth?}` → corre `Search()` + clasifica, **no persiste** — es una herramienta de investigación ad-hoc, igual que en la referencia (el `searchWatchman` de `thureos-main` tampoco persiste; solo el screening de un submission lo hace).
- `GET /api/v1/red-flags/:id/screening` — los `ScreeningResult` linkeados a ese caso.
- `POST /api/v1/screening/:id/dismiss` (compliance+): `{disposition: "dismissed"|"false_positive", notes}` — nota obligatoria, fija `WhitelistExpiresAt`.
- `GET/PUT /api/v1/settings/screening` (admin only): URL de Watchman — mismo patrón que `/settings/ai`.

## D — Frontend

**Diseño:**

- Página nueva de búsqueda manual (entrada en el nav lateral, sección "Monitoreo" o "Catálogos" — a definir en el plan de implementación).
- Sección en `red-flags/[id]/page.tsx`: si el caso tiene `ScreeningResult` linkeados, se muestran con badge de status y acción de descarte (nota obligatoria) — mismo patrón visual que el timeline de notas ya existente.
- Tarjeta en Settings ("Screening de sanciones"), admin only, mismo patrón que `notifications-card.tsx`.
- En el form de creación/edición de regla: multi-select "Campos a screenear" poblado con las columnas del schema del monitor elegido.

## Decisiones de diseño

1. **Sin pipeline de listas propio:** Watchman ya está desplegado y en la misma red Docker que este proyecto — el trabajo es un cliente HTTP + clasificación, no infraestructura de datos.
2. **Cliente HTTP simple, no el guard anti-SSRF de `monitor_puller.go`:** la URL de Watchman la configura un admin de confianza una vez (como SMTP), no llega por request de un usuario como `PullURL` — no es la misma superficie de ataque.
3. **`ScreeningFields` por regla, no por monitor:** reusa el patrón de placeholders tipados del catálogo de tipologías (`$COUNTERPART`) en vez de inventar un concepto nuevo de "campos de nombre" a nivel de schema.
4. **Búsqueda manual no persiste:** es una herramienta de investigación suelta, igual que en la referencia — solo el screening disparado por una regla queda linkeado a un caso.
5. **Descarte con expiración de 180 días + reevaluación automática:** confirmado para v1 porque sin la reevaluación (que reusa el cron de SLA de la Tarea 5) el campo de expiración no cumple ninguna función real — quedaría ahí sin que nada lo lea.
6. **Fail-closed:** un error de Watchman clasifica como REVIEW, nunca CLEAR — mismo principio regulatorio que la referencia documenta explícitamente (Acuerdo SBP 10-2015 citado en el código de `thureos-main`).
