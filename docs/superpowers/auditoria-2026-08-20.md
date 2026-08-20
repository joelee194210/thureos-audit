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

---

## 7. Verificación de cierre de rama · 2026-08-20

Ejecutada sobre `feat/linea-grafica-thureos` antes de decidir la integración.

| Comprobación | Resultado |
|---|---|
| `go build ./...` | ✅ |
| `go vet ./...` | ✅ |
| `npx next build` | ✅ 16 rutas |
| Colores literales en `frontend/src` | ✅ sin salida |
| Nombre de producto anterior en todo el repo | ✅ sin salida |
| `gofmt -l backend/` | ❌ 10 archivos (F4, sigue pendiente) |
| Clon limpio compila `./cmd/server` | ✅ tras corregir F11 |

Recuento de `gofmt` medido por comparación directa entre `master` y la rama:
15 archivos en `master`, 12 en la rama. La rama reformateó de paso cinco
(`mcc_handler.go`, `country_repo.go`, `mcc_repo.go`, `country_data.go`,
`mcc_data.go`) y dejó **dos nuevos** sin formatear (`cmd/server/main.go`,
`scheduler.go`), regresión propia corregida en `d2ca2a3`. Quedan 10, que son
F4 y siguen fuera de alcance. La auditoría original decía 16; el recuento
se hizo entonces sobre otro árbol y no se ha podido reproducir.

### F9 · El renombrado de identificadores cambia la base y la clave de cola

`21027e0` renombró la base Mongo `datawatch` → `thureos_compliance` y las claves
`datawatch:queue:*` → `thureos:queue:*`. El plan lo listaba como **fuera de alcance**,
así que entró sin la nota de migración que le correspondía.

Sobre una instalación previa que ya hubiera arrancado, esto la apunta en silencio a
una base vacía y abandona lo que quedara en la cola antigua. Es más grave que F8:
allí el cambio *no llega*, aquí *redirige sin avisar*.

**Verificado en esta máquina:** no existe la base `datawatch` ni ninguna clave
`datawatch:*`. El volumen se creó ya con el nombre nuevo —coherente con F1, donde
el compose nunca llegó a levantar por el choque de puertos—. La base viva es
`thureos_compliance` con 194 países, 320 MCC, 1 usuario. **Aquí no hay nada que migrar.**

Queda como advertencia para cualquier otra instalación: antes de desplegar esta rama
hay que comprobar si existe una base `datawatch` y, si existe, renombrarla
(`db.adminCommand({renameCollection…})` por colección, o `mongodump`/`mongorestore`).

### F10 · Artefactos de verificación visual rastreados — **[resuelto]**

`21027e0` commiteó `.playwright-mcp/` y `login-oscuro.png`. La regla de `.gitignore`
para ese directorio se añadió un commit más tarde (`0e177ac`), demasiado tarde para
alcanzar archivos ya rastreados. Desrastreados en `27925c5`; los archivos siguen en
disco. `.gitignore` cubre ahora también las capturas `*-oscuro.png` / `*-claro.png`.

### Nota sobre `mongo.go`

`ConnectMongo` fija `dbName := "thureos_compliance"` en código e **ignora la base que
venga en `MONGO_URI`**. No es una regresión de esta rama —ya era así— pero significa
que la ruta del URI es decorativa: apuntar a otra base exige recompilar.

### F11 · El punto de entrada del backend no estaba en el repositorio — **[resuelto]**

`backend/.gitignore` declaraba `server` en la línea 4 para ignorar el binario
compilado. Sin barra inicial, un patrón sin `/` intermedia coincide con **cualquier
archivo o directorio con ese nombre a cualquier profundidad**, así que la regla
excluía `backend/cmd/server/` entero. `main.go` —las 149 líneas que arrancan Fiber,
conectan Mongo y Redis, siembran el administrador y lanzan los workers— **nunca
estuvo rastreado**, ni en el commit base ni en ningún commit de la rama.

Consecuencia: un clon del repositorio no compila el servidor. Verificado sobre un
clon real en `b3208dc`:

```
$ go build ./cmd/server
stat backend/cmd/server: directory not found
```

No lo detectó ninguna verificación anterior porque todas se ejecutaron sobre el
árbol de trabajo, donde el archivo sí está. Sólo aparece al clonar.

Corregido en `7ad85fe`: regla anclada a `/server` y `cmd/server/main.go` añadido,
ya formateado. El mismo clon compila ahora `./cmd/server`. `main.go` no contiene
secretos: el administrador y la clave de API se siembran desde entorno.

El glob `*-oscuro.png` que este mismo informe introdujo en `27925c5` tenía el defecto
idéntico —habría tragado en silencio cualquier activo de marca en tema oscuro bajo
`frontend/public/brand/`— y se ancló a `/login-oscuro.png` en el mismo commit.

Barrido posterior de fuentes ignoradas por error: sólo queda `frontend/next-env.d.ts`,
que Next regenera en cada build. Benigno.
