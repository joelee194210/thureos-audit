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

### F3 · Sin rate limiting por IP — **[resuelto]**

**El enunciado original de este hallazgo era incorrecto.** Decía «sin rate limiting
ni bloqueo de cuenta»; la segunda mitad es falsa. El bloqueo por cuenta ya estaba
implementado de extremo a extremo —`MaxFailedAttempts = 5` y `LockoutDuration = 15m`
en `auth_service.go`, campos `failed_login_attempts` y `locked_until` en el modelo,
`IncrementFailedLogin` y `ResetFailedLogin` en el repositorio—. Lo que faltaba era
el límite por IP, y eso sí: no existía ningún `limiter`.

Añadido `middleware.AuthRateLimiter`, que envuelve el limitador que ya trae Fiber,
sobre las dos rutas públicas:

| Ruta | Cupo por IP | Qué cuenta |
|---|---|---|
| `POST /api/v1/auth/login` | 10 / 5 min | sólo los intentos **fallidos** |
| `POST /api/v1/auth/register` | 5 / 1 h | **todas** las peticiones, altas correctas incluidas |

La asimetría es deliberada. En el login lo que se encarece es adivinar contraseñas,
así que un inicio de sesión correcto no consume cupo. En el registro lo que se frena
es justamente la creación masiva de cuentas, que devuelve 201: ahí las peticiones
correctas son el ataque, y contarlas es el punto.

Verificado en ejecución contra el servidor real, con un usuario temporal creado y
borrado para la prueba:

| Prueba | Resultado |
|---|---|
| 15 inicios de sesión **correctos** seguidos | 15 × 200 — ninguno consume cupo |
| 12 intentos **fallidos** | 10 × 401, luego 429 |
| Inicio correcto con el cupo ya agotado | 429 |
| Registro, 7 intentos | 5 pasan, el sexto corta |
| `/health` y rutas protegidas | intactas |

La tercera fila es el matiz que conviene tener presente: no acumular cupo no es lo
mismo que quedar exento. Una vez que los fallos agotan la ventana, esa IP queda
cortada durante los cinco minutos aunque quien llegue después traiga la contraseña
correcta, porque el limitador responde antes de que la petición alcance el handler.
Es el comportamiento que se quiere frente a un ataque, y el coste que se paga si el
ataque viene desde la misma IP que los usuarios legítimos.

**Tres decisiones y sus contrapartidas**, todas deliberadas:

- **Almacén en memoria, no Redis.** Redis es opcional en esta aplicación: `main.go`
  advierte y continúa si falla. Un limitador que dependiera de él podría dejar el
  login inaccesible por una caída de la caché. La contrapartida es que **el cupo es
  por proceso**: detrás de más de una instancia hay que revisarlo.
- **Una dependencia indirecta nueva.** El limitador vive dentro del módulo Fiber que
  ya se importaba, así que no hay dependencia directa nueva, pero arrastra
  `tinylib/msgp` y `philhofer/fwd` a `go.mod` y `go.sum`.
- **`c.IP()` es el par del socket.** `fiber.Config` no declara proxies de confianza,
  lo cual es correcto en un solo host. Detrás de un proxy inverso habría que configurar
  `TrustedProxies` antes que nada; leer `X-Forwarded-For` sin eso sería peor, porque se
  falsifica con una cabecera.

  **Pero el problema de la IP compartida no espera a que haya un proxy.** Una oficina
  con NAT presenta una sola IP pública para todo el mundo, que es el despliegue probable
  de una plataforma de cumplimiento. Por eso los inicios de sesión correctos no consumen
  cupo: si lo hicieran, diez analistas entrando bien dejarían fuera al undécimo. Con la
  configuración actual sólo se acumulan los fallos, así que hacen falta diez errores de
  contraseña en cinco minutos entre todo el equipo para agotar el cupo. Si un equipo
  grande empieza a ver 429 legítimos, la palanca es subir `LoginMaxAttempts`, no
  desactivar el límite.

El mensaje de cuenta bloqueada se deja como está —revela que la cuenta existe y a qué
hora se recupera el acceso—. Es enumeración de usuarios, pero esto es un panel interno
de cumplimiento, no un servicio público: el dato vale poco al atacante y mucho a quien
se quedó fuera.

### B1 · El bloqueo de cuenta se podía usar como arma — **[resuelto]**

Encontrado al implementar F3. `ResetFailedLogin` se invocaba en un único sitio: tras
un login correcto. Nada limpiaba el contador cuando el bloqueo **expiraba**, así que
seguía en el umbral:

```
cuenta bloqueada → pasan 15 min → un intento fallido
  → $inc deja el contador en 6 → 6 >= 5 → otros 15 min de bloqueo
```

Cualquiera que conociera el correo de un usuario lo mantenía fuera del sistema
indefinidamente con **una petición cada cuarto de hora**. El límite por IP no lo
arregla: el ataque no necesita volumen. En una plataforma de cumplimiento, dejar
fuera a un analista a voluntad pesa más que la fuerza bruta que el bloqueo frenaba.

Corregido distinguiendo bloqueo vigente de bloqueo vencido: al vencer se limpian
contador y `locked_until` antes de evaluar la contraseña. Verificado contra Mongo
real —un usuario con 5 intentos y bloqueo vencido queda en 1 intento y sin bloqueo
tras un fallo, donde antes quedaba en 6 y bloqueado de nuevo—.

### F4 · 16 archivos Go sin formatear — *pendiente*
`gofmt -l backend/` los lista. No afecta a la ejecución.

### F5 · ESLint sin configurar — *pendiente*
`package.json` declara `"lint": "next lint"` pero no existe `eslint.config.*`. El script
abre un asistente interactivo: inutilizable en CI.

### F6 · Sin cobertura de pruebas — *pendiente, con la primera mella*
Vitest y `@testing-library/react` instalados, cero archivos de test en el frontend.
`rule_engine.go` (718 líneas) es el núcleo del producto y sigue sin una sola prueba.

El backend ya no está a cero: F3 y B1 trajeron los **primeros tests en Go del
repositorio**, cuatro casos que corren con `go test ./...` sin Mongo, sin Redis y sin
levantar nada. Para poder ejercitar el flujo de login sin base de datos se declaró
`userStore`, una interfaz de cuatro métodos en el consumidor
(`internal/services/auth_service.go`); `*repository.UserRepository` la satisface tal
cual y `main.go` no cambió. Ese es el patrón a repetir para atacar `rule_engine.go`.

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
| Clon limpio: `go build ./cmd/server` | ✅ tras corregir F11 |
| Clon limpio: `npm ci && npx next build` | ✅ 16 rutas |

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

### Nota sobre el método de verificación

F11 estuvo cuatro commits sin detectarse porque toda comprobación anterior corrió
sobre el árbol de trabajo, donde el archivo ausente sí existe. `git status --ignored`
tampoco lo delata: colapsa un directorio ignorado entero en una sola línea
(`!! backend/cmd/`), sin decir qué contiene.

La comprobación que sí discrimina es clonar y compilar. Hecha en ambas mitades sobre
`58fa3f9`: el backend compila `./cmd/server`, y el frontend levanta las 16 rutas tras
`npm ci`, con `src/styles/tokens/` y `public/brand/` completos en el clon.

**Conviene repetirla antes de cualquier despliegue**, no la verificación sobre el
árbol local.
