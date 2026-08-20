# Auditoría del proyecto · 2026-08-20

Verificación completa del proyecto y estado de la línea gráfica Thureos, previa a la
integración de marca. Los hallazgos marcados **[resuelto]** se corrigieron en la rama
`feat/linea-grafica-thureos`; el resto queda documentado para decisión posterior.

## 1. Verificación de compilación

| Comprobación | Antes | Ahora |
|---|---|---|
| `go build ./...` | ✅ | ✅ |
| `go vet ./...` | ✅ | ✅ |
| `gofmt -l .` | ❌ 16 archivos | ❌ 16 archivos (fuera de alcance) |
| `npx next build` | ✅ 16 rutas | ✅ 16 rutas |
| `npm run lint` | ❌ ESLint sin configurar | ❌ sin cambios (fuera de alcance) |
| Tests | ❌ cero | ❌ cero (fuera de alcance) |

## 2. Hallazgos

### F1 · Puertos desalineados entre compose y configuración — **[resuelto]**
`docker-compose.yml` publicaba Mongo en 27019 y Redis en 6381; `.env.example` y
`config.go` apuntaban a 27017/6379. Recién clonado, el backend no alcanzaba los servicios.

Al verificarlo se descubrió además que **6381 ya estaba ocupado** por `neuralhr-redis`,
otro proyecto de la misma máquina, así que el compose tampoco levantaba. Redis se movió
a **6383** y `.env.example` se alineó. Verificado: el backend conecta y arranca los workers.

### F2 · `JWT_SECRET` con fallback silencioso — **[resuelto]**
`config.go` caía a `"dev-secret-change-me"` sin avisar: firmaba tokens en producción con
un secreto público del repositorio. Ahora, mediante la variable nueva `APP_ENV`:

| Situación | Comportamiento |
|---|---|
| `JWT_SECRET` definida | se usa, sin ruido |
| ausente y `APP_ENV=production` | **aborta** con mensaje explícito (código 1) |
| ausente en cualquier otro caso | advertencia visible en arranque, usa el valor de desarrollo |

Los tres casos están verificados en ejecución.

### F3 · Sin rate limiting ni bloqueo de cuenta — *pendiente*
No existe `limiter` en el backend. `/api/v1/auth/login` acepta intentos ilimitados.
Relevante para una plataforma de cumplimiento.

### F4 · 16 archivos Go sin formatear — *pendiente*
`gofmt -l backend/` los lista. No afecta a la ejecución.

### F5 · ESLint sin configurar — *pendiente*
`package.json` declara `"lint": "next lint"` pero no existe `eslint.config.*`. El script
abre un asistente interactivo: inutilizable en CI.

### F6 · Sin cobertura de pruebas — *pendiente*
Vitest y `@testing-library/react` instalados, cero archivos de test. Backend igual.
`rule_engine.go` (718 líneas) es el núcleo del producto y no tiene una sola prueba.

### F7 · Puerto por defecto del cliente API incorrecto — **[resuelto]**
`frontend/src/lib/api/client.ts` caía a `http://localhost:8082/api/v1`, pero el backend
escucha en 8080. Sin un `.env.local`, el frontend recién clonado no encontraba la API.
Detectado en la verificación visual, no por inspección estática. Alineado a 8080.

### F8 · El seed no actualiza instalaciones existentes — *comportamiento, no bug*
`CountryRepository.Seed` y su equivalente de MCC salen temprano si la colección ya tiene
documentos (`count > 0 → return nil`). Es idempotencia deliberada: evita pisar ediciones
manuales de un administrador.

**Consecuencia operativa:** cualquier corrección futura a `internal/seed/*.go` —ortografía,
nivel de riesgo, catálogo nuevo— **nunca llegará a una instalación ya arrancada**. En esta
ocasión se resolvió vaciando `countries` y `mccs` para forzar el re-sembrado, algo seguro
porque no había ediciones manuales. En una instalación con datos editados hace falta una
migración explícita.

## 3. Aspectos correctos verificados

- El auto-registro asigna siempre `viewer`; solo un admin promueve.
- Complejidad de contraseña validada (mayúscula, minúscula, dígito, símbolo).
- bcrypt con `DefaultCost` y comparación en tiempo constante.
- RBAC aplicado por ruta con `RequireComplianceOrAbove()` / `RequireAdmin()`.
- La cola Redis con workers descrita en `CLAUDE.md` existe y funciona.

## 4. Deriva documental corregida

`CLAUDE.md` afirmaba cinco cosas falsas, todas corregidas: roles `analyst` (el real es
`compliance`), auth con NextAuth (es Zustand + JWT propio), una carpeta `docker/` que no
existe, `docs/` con otro contenido, y los puertos estándar de Mongo y Redis.

Las variables `NEXTAUTH_SECRET` y `NEXTAUTH_URL` de `frontend/.env.local.example` se
eliminaron: no las lee ninguna línea del código.

## 5. Estado de la línea gráfica

Al empezar: **0 % implementada**. El frontend usaba el tema por defecto de shadcn, grises
neutros y azul genérico `hsl(221 83% 53%)`.

Al terminar, verificado en ejecución sobre los dos temas y los dos ejes de marca:

| Comprobación | Resultado |
|---|---|
| `--bg-canvas` en oscuro | `#02101F` · navy Thureos |
| `--accent-solid` compliance | `#0B5FD6` oscuro · `#0048B0` claro |
| `--accent-solid` con `data-brand="fraud"` | `#DB8408` · ámbar |
| `--risk-critical-bg` en ambas marcas | `#8E1F1B` · idéntico, como exige el manual |
| Colores literales en `frontend/src` | **cero** |
| Tipografía | Inter 400/500/600/700 + JetBrains Mono 400/500/600 |

## 6. Activos gráficos: limitación pendiente

**No existe ningún SVG del logotipo.** El favicon y el icono de aplicación se generaron
reduciendo `icon-cropped.png`, centrado sobre navy plano con resguardo de 112 px sobre un
mínimo exigido de 71 px. A 16 px el detalle del circuito se pierde.

`logo2.png` —el lockup horizontal— tiene halos de recorte visibles y sobre fondo claro se
ve sucio. Por eso la aplicación usa el isotipo, que está limpio, y no el lockup. El manual
prohíbe de cuatro formas distintas retocar, recolorear o revectorizar el arte, así que no
se editó ninguna imagen.

**Conviene pedir el vector al diseñador.**
