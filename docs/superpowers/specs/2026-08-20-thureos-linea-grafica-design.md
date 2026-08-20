# Integración de la línea gráfica Thureos en monitors-main

Fecha: 2026-08-20 · Estado: pendiente de aprobación

> `monitors-main` no es un repositorio git, así que este documento no se pudo commitear.
> Queda como archivo suelto en `docs/superpowers/specs/`.

## Suposiciones declaradas

El usuario no estaba disponible al momento de redactar. Estas decisiones se tomaron con
criterio propio y son revisables; cada una está marcada como **[S1]**…**[S3]** en el texto.

- **[S1] Alcance.** Línea gráfica completa más los dos hallazgos de auditoría que impiden
  arrancar el proyecto (puertos y `JWT_SECRET`). Quedan fuera: tests, configuración de
  ESLint, `gofmt`, rate limiting. Están documentados en la auditoría para decisión aparte.
- **[S2] Nomenclatura. — AMPLIADA POR EL USUARIO.** La suposición original era renombrar
  solo los textos visibles. El usuario pidió después eliminar toda referencia al nombre de
  producto anterior, así que el alcance incluye los identificadores internos:

  | Elemento | Nombre actual |
  |---|---|
  | módulo Go | `github.com/thureos/compliance` |
  | base Mongo | `thureos_compliance` |
  | claves de cola Redis | `thureos:queue:rule_eval` · `thureos:queue:dead` |
  | contenedores | `thureos-mongo` · `thureos-redis` |
  | paquete del frontend | `thureos-compliance-frontend` |
  | autoría del historial git | `Thureos <dev@thureos.local>` |

  Renombrar la base era seguro porque solo contenía catálogos semilla (194 países,
  320 MCC, cero usuarios y cero alertas) que `main.go` vuelve a sembrar al arrancar.

- **[S3] Marca única.** Se instala la capa de tokens con soporte para las dos marcas
  (`compliance` y `fraud`), pero la aplicación fija `data-brand="compliance"`. No se
  construyen vistas de Fraud & Behavior.

## Problema

El frontend usa el tema por defecto de shadcn: grises neutros y azul genérico
`hsl(221 83% 53%)`. La marca Thureos está implementada al 0 %. El manual de marca
(`linea_grafica/Thureos-Manual-de-Marca_1.pdf`) es normativo y exige un mecanismo de tema
—`data-theme` más `data-brand` en el elemento raíz— incompatible con el modo oscuro por
clase que usa la aplicación hoy.

## Fuentes de verdad

Los cuatro archivos de código que lista la p. 16 del manual existen fuera del repositorio,
en dos copias idénticas byte a byte (`diff -rq` solo difiere en el README):

```
/Users/slacker/Downloads/tokens/
/Users/slacker/thureos_monitoreo/packages/design-tokens/tokens/
```

Se copian **sin modificar** a `frontend/src/styles/tokens/`. No se transcribe ni se
reescribe ningún valor de color: los tokens ya traen ratios de contraste medidos anotados
en línea, bloque `prefers-reduced-motion` y una rampa de series verificada para
deuteranopía y protanopía.

El patrón de integración se replica de `/Users/slacker/thureos-main`, que es Next.js 15 +
Tailwind 4 + shadcn y ya resolvió este problema en producción.

## Enfoque: por qué no `@config`

Tailwind 4 puede cargar el `tailwind.preset.js` v3 mediante `@config`. Se descarta por dos
razones: mezclaría configuración v3 con el bloque `@theme` que ya existe en `globals.css`,
y el preset expone escalas crudas (`navy-700`, `electric-500`) que los componentes no deben
consumir directamente —el manual exige que todo componente lea el token semántico—.

Se adopta el patrón de `thureos-main`: importar los CSS de tokens tal cual y escribir a mano
solo la capa puente `@theme inline`.

`tailwind.preset.js` **no se copia al repositorio**. Es el archivo que `@config` cargaría y
que aquí se descarta; copiarlo dejaría en el árbol un archivo que nada lee.

### Divergencia deliberada respecto a `thureos-main`, y por qué importa

`thureos-main` **no importa `shadcn-theme.css`**: tradujo sus tripletes a colores completos
en su propio `globals.css` (`--primary: hsl(214 100% 35%)`) y su puente pasa el valor sin
envolver (`--color-background: var(--background)`).

El archivo original emite tripletes sin coma y sin función: `--primary:214 100% 35%`. Si se
importa tal cual y el puente no envuelve, Tailwind genera `background-color: 214 100% 35%`,
que es inválido y **se ignora en silencio**: los colores simplemente no se aplican, sin
error en consola ni fallo de compilación.

Se elige envolver en el puente —`--color-primary: hsl(var(--primary))`— en lugar de traducir
los valores. Así `shadcn-theme.css` permanece intacto como fuente de verdad, que es la regla
que gobierna todo este trabajo, y la traducción no hay que rehacerla cada vez que el archivo
de tokens se actualice. El coste es una diferencia de una función `hsl()` respecto al patrón
de `thureos-main`; conviene tenerla presente al comparar los dos proyectos.

## Arquitectura de la solución

Cuatro capas, de abajo hacia arriba:

```
1. thureos-tokens.css   primitivas + semánticos por [data-theme] y [data-brand]
                        (--bg-canvas, --fg-default, --accent-*, --risk-*, --chart-1..8)
2. shadcn-theme.css     tripletes HSL de shadcn por tema y marca
                        (--primary:214 100% 35%, --border, --ring…)
3. @theme inline        puente: --color-primary: hsl(var(--primary)) …
                        expone los tokens como utilidades de Tailwind 4
4. componentes          consumen bg-primary, text-ink-muted, bg-risk-high…
                        sin escribir un solo color literal
```

El cambio de tema es un atributo en `<html>`; el de marca, otro. Ningún componente cambia.

### Por qué se importa la capa 1 completa

`thureos-main` integró solo la capa shadcn. Aquí se importa `thureos-tokens.css` entero
porque este producto tiene escalas de riesgo reales —nivel de riesgo de MCC, riesgo país,
severidad de alertas— y los tokens `--risk-none` … `--risk-critical` existen exactamente
para eso, en un eje deliberadamente separado del acento de producto. El manual es explícito:
«el acento de producto no comunica severidad».

## Componentes y archivos

### Nuevos

| Archivo | Propósito |
|---|---|
| `frontend/src/styles/tokens/thureos-tokens.css` | Copia sin modificar |
| `frontend/src/styles/tokens/shadcn-theme.css` | Copia sin modificar |
| `frontend/src/styles/tokens/thureos-tokens.json` | Fuente neutra, referencia |
| `frontend/src/styles/tokens/README.md` | Copia sin modificar |
| `frontend/public/favicon.ico` + `icon.png` | Derivados de `icon-cropped.png` |
| `frontend/public/brand/isotipo.png` | Copia de `icon-cropped.png` |
| `frontend/public/brand/lockup-horizontal.png` | Copia de `logo2.png` |
| `frontend/src/components/brand/logo.tsx` | Componente único de logotipo |
| `frontend/src/components/theme-provider.tsx` | Gestiona `data-theme`, sin parpadeo |

### Modificados

| Archivo | Cambio |
|---|---|
| `src/app/globals.css` | Reescrito: importa tokens, `@custom-variant dark` a `[data-theme="dark"]`, `@theme inline` |
| `src/app/layout.tsx` | `data-brand="compliance"`, Inter + JetBrains Mono, metadata «Thureos Compliance» |
| `src/components/layout/header.tsx` | El toggle escribe `data-theme`, no la clase `.dark` |
| `src/components/layout/sidebar.tsx` | Logotipo real; tildes en las etiquetas |
| `src/components/ui/badge.tsx` | Variantes `success`/`warning` a tokens; nuevas variantes de riesgo |
| `src/components/providers.tsx` | Monta el `ThemeProvider` |
| 9 páginas de `(dashboard)/` | 235 ocurrencias de colores literales → tokens |
| `src/app/(dashboard)/dashboards/[id]/page.tsx` | `CHART_COLORS` → `--chart-1..8`; corrige el `stroke` inválido |
| `src/app/login/page.tsx`, `register/page.tsx` | Marca y textos |
| `backend/.env.example`, `docker-compose.yml` | Puertos alineados (F1) |
| `backend/internal/config/config.go` | `JWT_SECRET` sin fallback silencioso (F2) |
| `CLAUDE.md` | Corrige la deriva documental |

## Detalle de las decisiones no obvias

### Cambio de mecanismo de tema

Hoy: `@custom-variant dark (&:is(.dark *))` más
`document.documentElement.classList.toggle("dark", …)` en `header.tsx:20,29`.

Pasa a: `@custom-variant dark (&:is([data-theme="dark"] *))` más
`document.documentElement.setAttribute("data-theme", …)`.

Toda utilidad `dark:` existente sigue funcionando sin tocarse, porque solo cambia la
condición del variant. Es el punto de mayor riesgo del trabajo y la razón por la que esto
es arquitectónico y no un ajuste acotado.

### Parpadeo de tema

`header.tsx` lee `localStorage` dentro de un `useEffect`, así que la primera pintura siempre
ocurre en claro y luego salta. Con un tema oscuro navy el salto es mucho más visible que
con el gris actual. Se resuelve con un script inline bloqueante en `<head>` que fija
`data-theme` antes de la primera pintura. El estado del tema se mueve de `header.tsx` a
`ThemeProvider`, que es donde corresponde: hoy el header es dueño de un estado global.

### Tema por defecto

Oscuro. El manual (p. 12) define para Compliance «tema base: oscuro; claro para exportar».
El tema claro se conserva completo y accesible desde el toggle.

### Tipografía

Inter ya está cargada pero sin restringir pesos. Se limita a 400/500/600/700 —el manual
prohíbe 300 y 800— y se añade JetBrains Mono en 400/500/600, obligatoria para
identificadores, montos y timestamps. Ambas por `next/font/google` con `variable:`, según
el patrón de `thureos-main`.

Aplicación de la mono: columnas de ID, importes, fechas y hashes en las tablas de alertas,
cargas, bitácora y datos de monitor. Cifras tabulares siempre.

### Colores literales: 235 ocurrencias en 11 archivos

Es el grueso del trabajo. La regla del manual es «ningún componente escribe un color
literal». El mapeo:

| Uso actual | Token |
|---|---|
| `bg-emerald-*`, `text-green-*` (éxito) | `--success-fg` / `--success-bg` |
| `bg-amber-*`, `text-yellow-*` (advertencia) | `--warning-fg` / `--warning-bg` |
| `bg-red-*`, `text-rose-*` (error) | `--danger-fg` / `--danger-bg` |
| Niveles de riesgo de MCC y país | `--risk-none` … `--risk-critical` |
| Severidad de alertas | `--risk-*`, nunca el acento |
| `bg-gray-*`, `bg-slate-*` (neutros) | `--fg-muted` / `--neutral-bg` |

Distinción crítica que hay que respetar página por página: el acento azul identifica al
producto, no la severidad. Un badge crítico es rojo en Compliance y rojo en Fraud.

Cada archivo tiene semántica propia y **ninguno se puede migrar con una sustitución
automática**. El plan de implementación debe tratarlos como tareas separadas:

| Archivo | Qué codifica el color |
|---|---|
| `alerts/page.tsx` | Severidad de alerta → `--risk-*` |
| `mcc/page.tsx` | Nivel de riesgo de MCC → `--risk-*` |
| `countries/page.tsx` | Nivel de riesgo país → `--risk-*` |
| `activity-logs/page.tsx` | `ACTION_COLORS` (línea 38): tipo de acción, categórico → `--chart-*` o neutros, **no** riesgo |
| `uploads/page.tsx` | `SOURCE_COLORS` (línea 52): tipo de fuente, categórico → mismo criterio |
| `monitors/[id]/page.tsx` | Estado de monitor y de ingesta → estados semánticos |
| `rules/page.tsx` | Severidad de regla → `--risk-*` |
| `settings/page.tsx` | Estados de sistema → estados semánticos |
| `(dashboard)/page.tsx` | Tarjetas de resumen, mezcla de ambos |
| `components/ui/badge.tsx` | Variantes `success`/`warning` con literales y pares `dark:` |
| `lib/use-toast.tsx` | Variantes de notificación → estados semánticos |

La confusión a evitar: `ACTION_COLORS` y `SOURCE_COLORS` son ejes **categóricos**, no de
severidad. Pintarlos con la escala de riesgo sugeriría que una carga por API es más
peligrosa que una por CSV.

### Gráficos

Los ocho valores de `CHART_COLORS` (`dashboards/[id]/page.tsx:79-88`) se sustituyen por
`var(--chart-1)` … `var(--chart-8)`, que cambian con el tema y están verificados para
daltonismo.

Además se corrige `stroke="hsl(var(--background))"` (línea 811): `--background` no existe en
el `@theme` actual —está definido `--color-background`, ya envuelto en `hsl()`—, así que hoy
ese stroke es CSS inválido. Se sustituye por `stroke="var(--bg-canvas)"`, que la capa de
tokens sí define y que sigue al tema. Conviene no dejarlo como está: al introducir
`shadcn-theme.css` aparecerá un `--background` con triplete crudo y esa línea pasaría a
resolver a un color equivocado en vez de a nada, que es peor porque no se nota.

### Logotipo, con su limitación

No existe ningún SVG. `icon-cropped.png` (494×618, con alfa) está limpio y sirve para
favicon, isotipo y sidebar colapsado. `logo2.png` (2172×724, con alfa) es el lockup
horizontal, pero tiene halos de recorte visibles y wordmark navy con resplandor: sobre
fondo claro se verá sucio.

Decisión: el componente `Logo` usa el isotipo en el sidebar —fondo navy, donde los halos no
se notan— y el lockup horizontal solo en login y registro, también sobre navy. El manual
prohíbe de cuatro formas distintas retocar, recolorear o revectorizar el arte, así que **no
se edita ninguna imagen**. Queda pendiente pedir el vector; hasta entonces el favicon a
16 px se genera por reducción del PNG y perderá el detalle fino del circuito.

### Ortografía

Las etiquetas de UI van sin tildes: «Operacion», «Catalogos», «Paises», «Administracion»,
«Configuracion», «Bitacora», «dinamicas». Se corrigen. Entra en alcance de marca porque el
manual es normativo sobre la voz y la tipografía en español.

### Los dos hallazgos bloqueantes

**F1 · Puertos.** `docker-compose.yml` publica Mongo en 27019 y Redis en 6381;
`.env.example` y `config.go` apuntan a 27017/6379. Se alinea `.env.example` a los puertos
que el compose ya publica, dejando el compose intacto —esos puertos altos probablemente
evitan un choque con otros contenedores de la máquina, y hay varios corriendo—.

**F2 · JWT_SECRET.** `config.go:31` cae a `"dev-secret-change-me"` sin avisar. Pasa a:
si `JWT_SECRET` está definida, se usa; si falta y `APP_ENV` vale `production`, el proceso
aborta con un mensaje explícito; si falta en cualquier otro caso, se conserva el valor de
desarrollo pero se registra una advertencia visible en arranque. `APP_ENV` es una variable
nueva con valor por defecto `development`, documentada en `.env.example`.

## Manejo de errores

Sin rutas de error nuevas. Dos consideraciones:

- El script de tema es defensivo: si `localStorage` no está disponible (modo privado),
  cae a `prefers-color-scheme` dentro de un `try/catch` y nunca rompe la pintura inicial.
- Los PNG de marca se sirven desde `public/`; si faltan, el componente `Logo` degrada al
  wordmark en texto en lugar de mostrar una imagen rota.

## Verificación

Ninguna de estas comprobaciones es opcional; el manual convierte el contraste en criterio
de aceptación explícito.

1. `npx next build` limpio.
2. Inspección visual de las 16 rutas en los dos temas. La aplicación necesita el backend en
   marcha: `docker compose up -d` más `go run cmd/server/main.go`.
3. Contraste: verificar los pares del sistema contra los umbrales de la p. 15 (texto
   principal, secundario, terciario, acento sobre fondo, texto sobre botón primario,
   estados semánticos ≥ 4,5:1; series de gráfico ≥ 3,0:1). Los tokens vienen con los
   ratios medidos, así que basta confirmar que ningún componente los eluda con un color
   literal.
4. Búsqueda de regresión: `grep -rE '(bg|text|border)-(red|green|amber|emerald|yellow|orange|blue|slate|gray|zinc|rose|purple|indigo)-[0-9]{2,3}' src` debe volver vacío.
5. Foco visible de 2 px con 2 px de separación en todo elemento interactivo.
6. `prefers-reduced-motion` respetado (lo aporta la capa de tokens).

No hay tests automatizados que ejecutar: el proyecto no tiene ninguno. Esa carencia está
registrada en la auditoría como hallazgo F6, fuera del alcance **[S1]**.

## Fuera de alcance

Tests (F6), configuración de ESLint (F5), `gofmt` de 16 archivos (F4), rate limiting (F3),
vistas de Fraud & Behavior Analytics, renombrado de identificadores internos, y cualquier
retoque del arte del logotipo.
