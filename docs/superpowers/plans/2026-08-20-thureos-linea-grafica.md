# Línea gráfica Thureos · Plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Sustituir el tema por defecto de shadcn por la línea gráfica Thureos Compliance, sin que ningún componente escriba un color literal.

**Architecture:** Cuatro capas. Los CSS de tokens se copian sin modificar desde la fuente de verdad externa; un puente `@theme inline` los expone como utilidades de Tailwind 4; los componentes consumen solo tokens semánticos. El tema y la marca se cambian con dos atributos en `<html>`, sin tocar componentes.

**Tech Stack:** Next.js 15 (App Router), React 19, Tailwind CSS 4, shadcn/ui, Recharts, Go 1.22 + Fiber (dos tareas de backend).

**Spec:** `docs/superpowers/specs/2026-08-20-thureos-linea-grafica-design.md`

## Global Constraints

- **Ningún componente escribe un color literal.** Siempre el token semántico. Criterio de aceptación del manual de marca, no preferencia de estilo.
- **El acento de producto no comunica severidad.** El azul identifica a Compliance; el riesgo usa `--risk-*`, que es un eje independiente.
- **Los archivos de `src/styles/tokens/` no se editan jamás.** Son copias de la fuente de verdad. Cualquier ajuste va en la capa puente.
- **El arte del logotipo no se modifica, recolorea, redibuja ni revectoriza.** Prohibido en cuatro puntos del manual (p. 7).
- **Pesos tipográficos:** Inter 400/500/600/700 (prohibidos 300 y 800). JetBrains Mono 400/500/600.
- **Ortografía del español completa**, con todas las tildes. El manual es normativo sobre la voz.
- **Contraste WCAG 2.1 AA** en los dos temas. Un cambio de color que rompa un umbral no se aprueba, aunque se vea mejor.
- **Nomenclatura:** «Thureos Compliance» en texto visible. Nunca «Compliance» solo como marca, y ninguna traza del nombre de producto anterior.
- **Foco visible** de 2 px con 2 px de separación en todo elemento interactivo (lo aporta la capa de tokens).

## Notas de ejecución que condicionan todos los pasos

**1. El repositorio no es git.** `monitors-main` no tiene `.git`. Los pasos `git commit` de cada tarea **no son ejecutables** hasta que se inicialice. La Tarea 0 lo resuelve y **requiere confirmación explícita del usuario**; si la rechaza, sáltate todos los pasos de commit y usa los pasos de verificación como cierre de tarea.

**2. No hay TDD porque no hay tests.** El proyecto no tiene un solo archivo de prueba y crear la infraestructura quedó fuera de alcance (`[S1]`). Para CSS y tokens el ciclo rojo-verde no aplica de forma natural. Cada tarea cierra con verificación **ejecutable y objetiva**: compilación, grep de regresión con salida esperada vacía, e inspección visual con criterios concretos. Cuando un paso dice «Esperado», es el resultado literal que debes obtener; si no coincide, la tarea no está hecha.

**3. Rutas.** Todas relativas a `/Users/slacker/monitors-main`. Las páginas viven bajo `frontend/src/app/(dashboard)/` — los paréntesis son parte del nombre real de la carpeta (grupo de rutas de Next.js) y hay que escaparlos o entrecomillarlos en la shell.

**4. Fuente de los tokens.** `/Users/slacker/Downloads/tokens/`. Existe una copia idéntica en `/Users/slacker/thureos_monitoreo/packages/design-tokens/tokens/`; si la primera falta, usa la segunda.

**5. El puente está verificado empíricamente, no supuesto.** Antes de escribir este plan se compiló una sonda contra `@tailwindcss/cli` 4.3.3 con el mismo `@theme inline` de la Tarea 1. Resultados confirmados:

- `--color-x: hsl(var(--y))` → genera `background-color: hsl(var(--y))`, con la función intacta. El puente funciona.
- Lo mismo para `text-*`, `border-*` y `border-l-*`.
- El modificador de opacidad sobre un token indirecto (`bg-x/15`) genera `color-mix(in oklab, var(--y) 15%, transparent)` con respaldo `@supports`. `CATEGORY_CLASSES` es válido.
- `sips -s format ico` sí produce un `.ico` en esta máquina (9854 bytes desde el isotipo).

Si algo de esto falla durante la ejecución, la causa será otra —orden de los `@import`, una ruta mal escrita— y no el mecanismo.

---

## Estructura de archivos

**Se crean:**

| Archivo | Responsabilidad |
|---|---|
| `frontend/src/styles/tokens/thureos-tokens.css` | Primitivas y semánticos por tema y marca. Copia intacta. |
| `frontend/src/styles/tokens/shadcn-theme.css` | Tripletes HSL de shadcn. Copia intacta. |
| `frontend/src/styles/tokens/thureos-tokens.json` | Fuente neutra. Referencia, no la consume el build. |
| `frontend/src/styles/tokens/README.md` | Documentación de los tokens. Copia intacta. |
| `frontend/src/components/theme-provider.tsx` | Único dueño del estado de tema. |
| `frontend/src/components/brand/logo.tsx` | Único punto que referencia arte de marca. |
| `frontend/src/lib/semantic-colors.ts` | Mapas de clases por riesgo, estado y categoría. Evita repetir cadenas en nueve páginas. |
| `frontend/public/brand/isotipo.png` | Copia de `icon-cropped.png`. |
| `frontend/public/brand/lockup-horizontal.png` | Copia de `logo2.png`. |
| `frontend/public/favicon.ico`, `frontend/public/icon.png` | Derivados del isotipo. |

**Se modifican:** `globals.css`, `layout.tsx`, `providers.tsx`, `header.tsx`, `sidebar.tsx`, `badge.tsx`, nueve páginas de `(dashboard)/`, `login/page.tsx`, `register/page.tsx`, `use-toast.tsx`, `backend/.env.example`, `backend/internal/config/config.go`, `CLAUDE.md`.

**No se copia:** `tailwind.preset.js`. Es el archivo que `@config` cargaría y que el spec descarta; dejarlo en el árbol crearía un archivo que nada lee.

---

### Task 0: Inicializar el repositorio git

**Requiere confirmación del usuario.** Si la rechaza, salta esta tarea y omite todos los pasos de commit del plan.

**Files:**
- Create: `.gitignore` (ya existe en la raíz; verificar cobertura)

**Interfaces:**
- Produces: un repositorio git con un commit base, para que el resto de tareas puedan aislar sus cambios.

- [ ] **Step 1: Confirmar con el usuario**

Pregunta literal: «¿Inicializo git en `monitors-main`? Sin él no puedo commitear por tarea y perdemos la capacidad de revertir un paso concreto.» No continúes sin un sí.

- [ ] **Step 2: Verificar que el .gitignore cubre lo necesario**

Run: `cat /Users/slacker/monitors-main/.gitignore`
Debe incluir `node_modules`, `.next`, `.env`. Si falta alguno, añádelo:

```
node_modules/
.next/
.env
.DS_Store
bin/
```

- [ ] **Step 3: Inicializar y hacer el commit base**

```bash
cd /Users/slacker/monitors-main
git init
git add -A
git commit -m "chore: commit base antes de integrar la línea gráfica Thureos"
```

- [ ] **Step 4: Verificar**

Run: `git -C /Users/slacker/monitors-main log --oneline`
Esperado: un commit. `git status` limpio salvo lo ignorado.

---

### Task 1: Instalar la capa de tokens y el puente de Tailwind 4

Es la tarea de mayor riesgo del plan: cambia el mecanismo de tema de clase a atributo, del que depende toda utilidad `dark:` del código.

**Files:**
- Create: `frontend/src/styles/tokens/{thureos-tokens.css,shadcn-theme.css,thureos-tokens.json,README.md}`
- Create: `frontend/src/components/theme-provider.tsx`
- Modify: `frontend/src/app/globals.css` (reescritura completa)
- Modify: `frontend/src/app/layout.tsx`
- Modify: `frontend/src/components/providers.tsx`
- Modify: `frontend/src/components/layout/header.tsx:12-31`

**Interfaces:**
- Produces:
  - Utilidades Tailwind: `bg-canvas`, `bg-surface`, `bg-surface-2`, `bg-elevated`, `text-ink`, `text-ink-muted`, `text-ink-subtle`, `border-line`, `border-line-subtle`, `bg-accent-soft`, `text-accent-fg`
  - Utilidades de riesgo: `bg-risk-{none,low,medium,high,critical}-bg` y `text-risk-{…}-fg`
  - Utilidades de estado: `bg-{success,warning,danger,info}-bg`, `text-{…}-fg`, `border-{…}-border`
  - `useTheme(): { theme: "light" | "dark"; toggleTheme: () => void }` exportado de `theme-provider.tsx`
  - Las utilidades shadcn existentes (`bg-primary`, `text-muted-foreground`, `border-input`…) siguen funcionando con valores Thureos

- [ ] **Step 1: Copiar los tokens sin modificarlos**

```bash
mkdir -p /Users/slacker/monitors-main/frontend/src/styles/tokens
cp /Users/slacker/Downloads/tokens/thureos-tokens.css \
   /Users/slacker/Downloads/tokens/shadcn-theme.css \
   /Users/slacker/Downloads/tokens/thureos-tokens.json \
   /Users/slacker/Downloads/tokens/README.md \
   /Users/slacker/monitors-main/frontend/src/styles/tokens/
```

- [ ] **Step 2: Verificar que la copia es fiel**

Run: `diff -r /Users/slacker/Downloads/tokens /Users/slacker/monitors-main/frontend/src/styles/tokens`
Esperado: solo la línea `Only in /Users/slacker/Downloads/tokens: tailwind.preset.js`. Cualquier otra diferencia significa que algo se modificó; corrígelo antes de seguir.

- [ ] **Step 3: Reescribir globals.css**

Reemplaza el contenido completo de `frontend/src/app/globals.css`:

```css
@import "tailwindcss";
@import "../styles/tokens/thureos-tokens.css";
@import "../styles/tokens/shadcn-theme.css";

/* El tema vive en un atributo del elemento raíz, no en una clase.
   Toda utilidad dark: existente sigue funcionando: solo cambia la condición. */
@custom-variant dark (&:is([data-theme="dark"] *));

@theme inline {
  /* --- Puente shadcn ---------------------------------------------------
     shadcn-theme.css emite tripletes sin función (--primary: 214 100% 35%).
     Se envuelven aquí para que shadcn-theme.css quede intacto como fuente
     de verdad. Sin el hsl() Tailwind emite CSS inválido que se ignora en
     silencio: los colores no se aplican y no hay error en consola.        */
  --color-background: hsl(var(--background));
  --color-foreground: hsl(var(--foreground));
  --color-card: hsl(var(--card));
  --color-card-foreground: hsl(var(--card-foreground));
  --color-popover: hsl(var(--popover));
  --color-popover-foreground: hsl(var(--popover-foreground));
  --color-primary: hsl(var(--primary));
  --color-primary-foreground: hsl(var(--primary-foreground));
  --color-secondary: hsl(var(--secondary));
  --color-secondary-foreground: hsl(var(--secondary-foreground));
  --color-muted: hsl(var(--muted));
  --color-muted-foreground: hsl(var(--muted-foreground));
  --color-accent: hsl(var(--accent));
  --color-accent-foreground: hsl(var(--accent-foreground));
  --color-destructive: hsl(var(--destructive));
  --color-destructive-foreground: hsl(var(--destructive-foreground));
  --color-border: hsl(var(--border));
  --color-input: hsl(var(--input));
  --color-ring: hsl(var(--ring));

  /* --- Tokens Thureos ---------------------------------------------------
     Ya son colores completos en thureos-tokens.css: NO se envuelven.      */
  --color-canvas: var(--bg-canvas);
  --color-surface: var(--bg-surface);
  --color-surface-2: var(--bg-surface-2);
  --color-elevated: var(--bg-elevated);
  --color-inset: var(--bg-inset);

  --color-line: var(--border-default);
  --color-line-subtle: var(--border-subtle);
  --color-line-strong: var(--border-strong);

  --color-ink: var(--fg-default);
  --color-ink-muted: var(--fg-muted);
  --color-ink-subtle: var(--fg-subtle);
  --color-ink-disabled: var(--fg-disabled);
  --color-ink-inverse: var(--fg-inverse);

  --color-accent-solid: var(--accent-solid);
  --color-accent-fg: var(--accent-fg);
  --color-accent-soft: var(--accent-soft-bg);
  --color-accent-on: var(--fg-on-accent);

  --color-success-fg: var(--success-fg);
  --color-success-bg: var(--success-bg);
  --color-success-border: var(--success-border);
  --color-warning-fg: var(--warning-fg);
  --color-warning-bg: var(--warning-bg);
  --color-warning-border: var(--warning-border);
  --color-danger-fg: var(--danger-fg);
  --color-danger-bg: var(--danger-bg);
  --color-danger-border: var(--danger-border);
  --color-info-fg: var(--info-fg);
  --color-info-bg: var(--info-bg);
  --color-info-border: var(--info-border);

  /* Escala de riesgo: eje independiente del acento de producto. */
  --color-risk-none-fg: var(--risk-none-fg);
  --color-risk-none-bg: var(--risk-none-bg);
  --color-risk-low-fg: var(--risk-low-fg);
  --color-risk-low-bg: var(--risk-low-bg);
  --color-risk-medium-fg: var(--risk-medium-fg);
  --color-risk-medium-bg: var(--risk-medium-bg);
  --color-risk-high-fg: var(--risk-high-fg);
  --color-risk-high-bg: var(--risk-high-bg);
  --color-risk-critical-fg: var(--risk-critical-fg);
  --color-risk-critical-bg: var(--risk-critical-bg);

  /* Series de gráfico: verificadas para deuteranopía y protanopía. */
  --color-chart-1: var(--chart-1);
  --color-chart-2: var(--chart-2);
  --color-chart-3: var(--chart-3);
  --color-chart-4: var(--chart-4);
  --color-chart-5: var(--chart-5);
  --color-chart-6: var(--chart-6);
  --color-chart-7: var(--chart-7);
  --color-chart-8: var(--chart-8);

  --font-sans: var(--font-inter), "Inter", system-ui, sans-serif;
  --font-mono: var(--font-jetbrains-mono), "JetBrains Mono", ui-monospace, monospace;

  --radius-sm: var(--thu-radius-sm);
  --radius-md: var(--thu-radius-md);
  --radius-lg: var(--thu-radius-lg);
  --radius-xl: var(--thu-radius-xl);
}

@layer base {
  * {
    @apply border-border;
  }
  body {
    @apply bg-background text-foreground;
    font-feature-settings: "rlig" 1, "calt" 1;
  }
  /* Cifras tabulares en datos: el manual las exige en tablas y KPIs. */
  .tabular {
    font-variant-numeric: tabular-nums;
  }
}
```

- [ ] **Step 4: Crear el ThemeProvider**

El estado de tema vive hoy en `header.tsx`, que no es su sitio: es estado global. Crea `frontend/src/components/theme-provider.tsx`:

```tsx
"use client";

import { createContext, useContext, useEffect, useState } from "react";

type Theme = "light" | "dark";

interface ThemeContextValue {
  theme: Theme;
  toggleTheme: () => void;
}

const ThemeContext = createContext<ThemeContextValue>({
  theme: "dark",
  toggleTheme: () => {},
});

export function useTheme() {
  return useContext(ThemeContext);
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  // El script inline del layout ya fijó data-theme antes de la primera
  // pintura. Aquí solo se lee, nunca se decide: decidir de nuevo
  // reintroduciría el parpadeo que el script existe para evitar.
  const [theme, setTheme] = useState<Theme>("dark");

  useEffect(() => {
    const current = document.documentElement.getAttribute("data-theme");
    setTheme(current === "light" ? "light" : "dark");
  }, []);

  function toggleTheme() {
    const next: Theme = theme === "dark" ? "light" : "dark";
    setTheme(next);
    document.documentElement.setAttribute("data-theme", next);
    try {
      localStorage.setItem("theme", next);
    } catch {
      // Modo privado: el tema no persiste, pero la app no se rompe.
    }
  }

  return (
    <ThemeContext.Provider value={{ theme, toggleTheme }}>
      {children}
    </ThemeContext.Provider>
  );
}
```

- [ ] **Step 5: Reescribir layout.tsx con las fuentes, la marca y el script anti-parpadeo**

Reemplaza el contenido completo de `frontend/src/app/layout.tsx`:

```tsx
import type { Metadata } from "next";
import { Inter, JetBrains_Mono } from "next/font/google";
import { Providers } from "@/components/providers";
import "./globals.css";

// Interfaz y cuerpo. Pesos autorizados por el manual: 400/500/600/700.
const inter = Inter({
  variable: "--font-inter",
  subsets: ["latin"],
  weight: ["400", "500", "600", "700"],
  display: "swap",
});

// Todo lo que se verifica: identificadores, montos, hashes y timestamps.
const jetbrainsMono = JetBrains_Mono({
  variable: "--font-jetbrains-mono",
  subsets: ["latin"],
  weight: ["400", "500", "600"],
  display: "swap",
});

export const metadata: Metadata = {
  title: "Thureos Compliance",
  description:
    "Plataforma de monitoreo transaccional con motor de reglas dinámicas",
};

// Se ejecuta antes de la primera pintura: sin él, la página aparece en claro
// y salta a navy. Con un tema oscuro el salto es muy visible.
const themeScript = `
(function(){
  try {
    var t = localStorage.getItem('theme');
    if (t !== 'light' && t !== 'dark') t = 'dark';
    document.documentElement.setAttribute('data-theme', t);
  } catch (e) {
    document.documentElement.setAttribute('data-theme', 'dark');
  }
})();
`;

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html
      lang="es"
      data-theme="dark"
      data-brand="compliance"
      suppressHydrationWarning
    >
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeScript }} />
      </head>
      <body
        className={`${inter.variable} ${jetbrainsMono.variable} font-sans antialiased`}
      >
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
```

- [ ] **Step 6: Montar el ThemeProvider en providers.tsx**

Lee `frontend/src/components/providers.tsx` y envuelve el árbol existente con `<ThemeProvider>`, dejando intactos los proveedores que ya haya (TanStack Query, toasts). El import es:

```tsx
import { ThemeProvider } from "@/components/theme-provider";
```

`ThemeProvider` debe ser el más externo, para que cualquier componente pueda leer el tema.

- [ ] **Step 7: Migrar el toggle del header**

En `frontend/src/components/layout/header.tsx`, elimina el estado local de tema y sus dos `useEffect` (líneas 12-31 del archivo actual) y consume el contexto. El bloque de tema queda así:

```tsx
import { useTheme } from "@/components/theme-provider";

// dentro del componente, sustituyendo useState/useEffect/toggleTheme locales:
const { theme, toggleTheme } = useTheme();
```

El `useEffect` que carga `alertsApi.stats()` **se conserva**: no tiene relación con el tema. El JSX del botón no cambia.

- [ ] **Step 8: Verificar que compila**

Run: `cd /Users/slacker/monitors-main/frontend && npx next build`
Esperado: `✓ Compiled successfully`, 16 rutas. Si falla con un error de CSS sobre `@custom-variant` o `@theme`, revisa que los `@import` estén todos al principio del archivo: es una regla de CSS, no de Tailwind.

- [ ] **Step 9: Verificar visualmente que los tokens se aplican**

Levanta la aplicación:
```bash
cd /Users/slacker/monitors-main && docker compose up -d
cd backend && go run cmd/server/main.go &
cd ../frontend && npm run dev
```

Abre `http://localhost:3000/login`. Criterios objetivos:
1. El fondo es navy profundo (`#02101F`), no blanco ni gris.
2. Al recargar **no hay destello blanco** antes de que aparezca el navy.
3. El botón primario es azul eléctrico, no el azul genérico anterior.
4. El toggle del header alterna a tema claro y el fondo pasa a blanco.
5. En DevTools, `<html>` tiene `data-theme` y `data-brand="compliance"`.

Si los colores no cambian y tampoco hay error: casi con seguridad falta el `hsl()` del puente. Inspecciona un elemento y busca `background-color: 214 100% 35%` sin función — ese es el síntoma exacto.

- [ ] **Step 10: Commit**

```bash
cd /Users/slacker/monitors-main
git add frontend/src/styles/tokens frontend/src/app/globals.css frontend/src/app/layout.tsx frontend/src/components/theme-provider.tsx frontend/src/components/providers.tsx frontend/src/components/layout/header.tsx
git commit -m "feat(marca): instalar tokens Thureos y puente de Tailwind 4"
```

---

### Task 2: Activos de marca y componente de logotipo

**Files:**
- Create: `frontend/public/brand/isotipo.png`, `frontend/public/brand/lockup-horizontal.png`
- Create: `frontend/public/icon.png`, `frontend/public/favicon.ico`
- Create: `frontend/src/components/brand/logo.tsx`
- Modify: `frontend/src/components/layout/sidebar.tsx:78-86`
- Modify: `frontend/src/app/login/page.tsx`, `frontend/src/app/register/page.tsx`

**Interfaces:**
- Consumes: las utilidades de token de la Tarea 1.
- Produces: `<Logo variant="isotipo" | "lockup" size={number} />` desde `@/components/brand/logo`.

- [ ] **Step 1: Copiar los activos**

El arte no se edita: solo se copia y se reescala. Reescalar no es redibujar.

```bash
mkdir -p /Users/slacker/monitors-main/frontend/public/brand
cp "/Users/slacker/monitors-main/linea_grafica/icon-cropped.png" \
   /Users/slacker/monitors-main/frontend/public/brand/isotipo.png
cp "/Users/slacker/monitors-main/linea_grafica/logo2.png" \
   /Users/slacker/monitors-main/frontend/public/brand/lockup-horizontal.png
```

- [ ] **Step 2: Generar el favicon**

```bash
cd /Users/slacker/monitors-main/frontend/public
sips -z 512 512 brand/isotipo.png --out icon.png
sips -s format ico -z 48 48 brand/isotipo.png --out favicon.ico
```

Verifica: `sips -g pixelWidth -g pixelHeight icon.png` → 512×512.

Si `sips` no genera `.ico` en esta versión de macOS, deja solo `icon.png`: Next.js 15 lo toma como icono de la aplicación desde `public/`. No inventes otro formato ni edites el arte.

- [ ] **Step 3: Crear el componente Logo**

Único punto del código que referencia arte de marca. Si falta el archivo, degrada al wordmark en texto en lugar de mostrar una imagen rota.

```tsx
"use client";

import Image from "next/image";
import { useState } from "react";

interface LogoProps {
  /** isotipo: escudo solo, para sidebar y avatares.
   *  lockup: escudo + wordmark, para login y cabeceras. */
  variant?: "isotipo" | "lockup";
  /** Altura en px. Mínimos del manual: isotipo 24, lockup 180 de ancho. */
  size?: number;
  className?: string;
}

export function Logo({ variant = "isotipo", size = 32, className }: LogoProps) {
  const [failed, setFailed] = useState(false);

  if (failed) {
    return (
      <span
        className={`font-semibold uppercase tracking-[0.18em] text-ink ${className ?? ""}`}
        style={{ fontSize: size * 0.4 }}
      >
        Thureos
      </span>
    );
  }

  if (variant === "lockup") {
    // Proporción original 2172×724 = 3:1.
    return (
      <Image
        src="/brand/lockup-horizontal.png"
        alt="Thureos Compliance"
        width={size * 3}
        height={size}
        className={className}
        onError={() => setFailed(true)}
        priority
      />
    );
  }

  // Proporción original 494×618.
  return (
    <Image
      src="/brand/isotipo.png"
      alt="Thureos"
      width={Math.round(size * 0.8)}
      height={size}
      className={className}
      onError={() => setFailed(true)}
      priority
    />
  );
}
```

- [ ] **Step 4: Sustituir el logotipo falso del sidebar**

En `frontend/src/components/layout/sidebar.tsx`, reemplaza el bloque del logotipo (hoy un icono `Monitor` de Lucide dentro de un cuadro más el nombre de producto anterior en texto):

```tsx
<div className="flex h-14 items-center border-b border-line-subtle px-6">
  <Link href="/" className="flex items-center gap-2.5">
    <Logo variant="isotipo" size={28} />
    <span className="text-sm font-semibold uppercase tracking-[0.14em] text-ink">
      Thureos
    </span>
  </Link>
</div>
```

Añade `import { Logo } from "@/components/brand/logo";` y elimina `Monitor` del import de `lucide-react` **solo si no se usa en otro sitio del archivo** — se usa como icono del elemento «Monitores», así que probablemente debe quedarse. Verifícalo antes de borrarlo.

- [ ] **Step 5: Poner el lockup en login y registro**

En `frontend/src/app/login/page.tsx` y `register/page.tsx`, sustituye el título de marca en texto por el lockup, centrado sobre el fondo navy:

```tsx
<div className="mb-8 flex justify-center">
  <Logo variant="lockup" size={56} />
</div>
```

El lockup solo va sobre navy: tiene halos de recorte que sobre fondo claro se ven sucios. No lo pongas sobre `bg-canvas` en tema claro.

- [ ] **Step 6: Verificar**

Run: `cd /Users/slacker/monitors-main/frontend && npx next build`
Esperado: compilación limpia.

Visualmente: el sidebar muestra el escudo real; la pestaña del navegador muestra el isotipo; login muestra el lockup sobre navy.

Run: busca el nombre de producto anterior bajo `frontend/src`
Esperado: sin coincidencias en el sidebar. Puede quedar alguna en páginas aún no migradas; se limpian en la Tarea 8.

- [ ] **Step 7: Commit**

```bash
cd /Users/slacker/monitors-main
git add frontend/public frontend/src/components/brand frontend/src/components/layout/sidebar.tsx frontend/src/app/login/page.tsx frontend/src/app/register/page.tsx
git commit -m "feat(marca): activos Thureos y componente de logotipo"
```

---

### Task 3: Mapas de color semántico y variantes de badge

Centraliza las cadenas de clases para que las cinco tareas de migración siguientes no repitan literales. Sin esto, el mismo mapeo se copiaría nueve veces y divergiría.

**Files:**
- Create: `frontend/src/lib/semantic-colors.ts`
- Modify: `frontend/src/components/ui/badge.tsx:6-23`

**Interfaces:**
- Consumes: utilidades de la Tarea 1.
- Produces:
  - `RISK_CLASSES: Record<"none"|"low"|"medium"|"high"|"critical", string>`
  - `STATUS_CLASSES: Record<"success"|"warning"|"danger"|"info"|"neutral", string>`
  - `CATEGORY_CLASSES: string[]` (8 entradas, series de gráfico)
  - `RISK_FG`, `RISK_BORDER_L`, `STATUS_FG` — variantes de un solo eje, para iconos y bordes
  - Variantes nuevas de `Badge`: `success`, `warning`, `info`, `risk-none`…`risk-critical`

- [ ] **Step 1: Crear semantic-colors.ts**

```ts
/**
 * Mapas de clase por eje semántico. Ningún componente escribe un color
 * literal: importa de aquí.
 *
 * Tres ejes distintos, que no se mezclan:
 *  - RISK_CLASSES     severidad y riesgo. Rojo es crítico en toda la
 *                     plataforma, independientemente de la marca activa.
 *  - STATUS_CLASSES   resultado de una operación: éxito, error, aviso.
 *  - CATEGORY_CLASSES distinción sin orden: tipo de fuente, tipo de acción.
 *                     Usa las series de gráfico, verificadas para daltonismo.
 */

export type RiskLevel = "none" | "low" | "medium" | "high" | "critical";

export const RISK_CLASSES: Record<RiskLevel, string> = {
  none: "bg-risk-none-bg text-risk-none-fg",
  low: "bg-risk-low-bg text-risk-low-fg",
  medium: "bg-risk-medium-bg text-risk-medium-fg",
  high: "bg-risk-high-bg text-risk-high-fg",
  critical: "bg-risk-critical-bg text-risk-critical-fg",
};

/** Solo el color de texto, para iconos sobre fondo transparente. */
export const RISK_FG: Record<RiskLevel, string> = {
  none: "text-risk-none-fg",
  low: "text-risk-low-fg",
  medium: "text-risk-medium-fg",
  high: "text-risk-high-fg",
  critical: "text-risk-critical-fg",
};

/** Borde izquierdo para tarjetas de alerta. */
export const RISK_BORDER_L: Record<RiskLevel, string> = {
  none: "border-l-risk-none-fg",
  low: "border-l-risk-low-fg",
  medium: "border-l-risk-medium-fg",
  high: "border-l-risk-high-fg",
  critical: "border-l-risk-critical-fg",
};

export type StatusKind = "success" | "warning" | "danger" | "info" | "neutral";

export const STATUS_CLASSES: Record<StatusKind, string> = {
  success: "bg-success-bg text-success-fg border-success-border",
  warning: "bg-warning-bg text-warning-fg border-warning-border",
  danger: "bg-danger-bg text-danger-fg border-danger-border",
  info: "bg-info-bg text-info-fg border-info-border",
  neutral: "bg-surface-2 text-ink-muted border-line",
};

export const STATUS_FG: Record<StatusKind, string> = {
  success: "text-success-fg",
  warning: "text-warning-fg",
  danger: "text-danger-fg",
  info: "text-info-fg",
  neutral: "text-ink-muted",
};

/**
 * Ejes categóricos: tipo de fuente, tipo de acción. Sin orden ni severidad.
 * Pintarlos con la escala de riesgo sugeriría que una carga por API es más
 * peligrosa que una por CSV, que es falso.
 */
export const CATEGORY_CLASSES: string[] = [
  "bg-chart-1/15 text-chart-1",
  "bg-chart-2/15 text-chart-2",
  "bg-chart-3/15 text-chart-3",
  "bg-chart-4/15 text-chart-4",
  "bg-chart-5/15 text-chart-5",
  "bg-chart-6/15 text-chart-6",
  "bg-chart-7/15 text-chart-7",
  "bg-chart-8/15 text-chart-8",
];

```

No añadas una función de hash de clave a serie: los dos mapas categóricos del
proyecto (`ACTION_COLORS` y `SOURCE_COLORS`) tienen claves fijas y conocidas, así que
la asignación se escribe explícita. Un hash haría impredecible qué color recibe cada
acción y no resolvería ningún problema real.

- [ ] **Step 2: Reescribir las variantes de badge**

En `frontend/src/components/ui/badge.tsx`, reemplaza el bloque `variants.variant` completo. Las variantes `success` y `warning` actuales usan literales `bg-emerald-100 … dark:bg-emerald-900`:

```tsx
      variant: {
        default: "border-transparent bg-primary text-primary-foreground shadow",
        secondary: "border-transparent bg-secondary text-secondary-foreground",
        destructive: "border-transparent bg-destructive text-destructive-foreground shadow",
        outline: "text-foreground",
        success: "border-success-border bg-success-bg text-success-fg",
        warning: "border-warning-border bg-warning-bg text-warning-fg",
        info: "border-info-border bg-info-bg text-info-fg",
        "risk-none": "border-transparent bg-risk-none-bg text-risk-none-fg",
        "risk-low": "border-transparent bg-risk-low-bg text-risk-low-fg",
        "risk-medium": "border-transparent bg-risk-medium-bg text-risk-medium-fg",
        "risk-high": "border-transparent bg-risk-high-bg text-risk-high-fg",
        "risk-critical": "border-transparent bg-risk-critical-bg text-risk-critical-fg",
      },
```

- [ ] **Step 3: Verificar**

Run: `cd /Users/slacker/monitors-main/frontend && npx next build`
Esperado: compilación limpia.

Run: `grep -nE '(emerald|amber)-[0-9]' /Users/slacker/monitors-main/frontend/src/components/ui/badge.tsx`
Esperado: sin coincidencias.

- [ ] **Step 4: Commit**

```bash
cd /Users/slacker/monitors-main
git add frontend/src/lib/semantic-colors.ts frontend/src/components/ui/badge.tsx
git commit -m "feat(marca): mapas de color semántico y variantes de badge por riesgo"
```

---

### Task 4: Migrar la escala de riesgo (MCC y países)

Los dos archivos comparten el patrón exacto: `bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 border-green-200 dark:border-green-800`. Seis clases para expresar «riesgo bajo»; un token lo hace en dos.

**Files:**
- Modify: `frontend/src/app/(dashboard)/mcc/page.tsx:28-32` y las tarjetas de estadísticas (~líneas 149-165)
- Modify: `frontend/src/app/(dashboard)/countries/page.tsx` (mismo patrón)

**Interfaces:**
- Consumes: `RISK_CLASSES`, `RISK_FG` de `@/lib/semantic-colors`.

- [ ] **Step 1: Migrar el mapa de riesgo de MCC**

En `mcc/page.tsx`, el objeto de configuración de riesgo pasa a:

```tsx
import { RISK_CLASSES, RISK_FG } from "@/lib/semantic-colors";

const RISK_CONFIG = {
  high: { label: "Alto", color: `${RISK_CLASSES.high} border-transparent`, icon: ShieldAlert },
  medium: { label: "Medio", color: `${RISK_CLASSES.medium} border-transparent`, icon: Shield },
  low: { label: "Bajo", color: `${RISK_CLASSES.low} border-transparent`, icon: ShieldCheck },
};
```

Conserva el nombre real de la constante que tenga el archivo; no lo renombres.

- [ ] **Step 2: Migrar las tarjetas de estadísticas de MCC**

Cada tarjeta tiene la forma `<div className="rounded-lg bg-green-100 dark:bg-green-900/30 p-2"><Icon className="h-5 w-5 text-green-600 dark:text-green-400" /></div>`. Pasa a:

```tsx
<div className="rounded-lg bg-risk-low-bg p-2">
  <ShieldCheck className="h-5 w-5 text-risk-low-fg" />
</div>
```

Mapeo por tarjeta:
- verde → `risk-low`
- amarillo → `risk-medium`
- rojo → `risk-high`
- morado (total de categorías, **no es riesgo**) → `bg-accent-soft` + `text-accent-fg`

La tarjeta morada es la trampa de este paso: cuenta categorías, no mide riesgo. Pintarla con la escala de riesgo sería un error semántico.

- [ ] **Step 3: Migrar countries/page.tsx**

Mismo patrón. Además de las clases de riesgo, este archivo usa neutros:
- `bg-gray-100 dark:bg-gray-800` → `bg-surface-2`
- `text-gray-500 dark:text-gray-400` → `text-ink-muted`
- `border-gray-200 dark:border-gray-700` → `border-line`
- `bg-purple-*` / `bg-indigo-600` / `bg-blue-700` (contadores, no riesgo) → `bg-accent-soft` + `text-accent-fg`

- [ ] **Step 4: Verificar sin regresiones**

```bash
cd /Users/slacker/monitors-main/frontend/src
grep -nE '(bg|text|border)-(red|green|amber|emerald|yellow|orange|blue|slate|gray|zinc|rose|purple|indigo)-[0-9]{2,3}' \
  'app/(dashboard)/mcc/page.tsx' 'app/(dashboard)/countries/page.tsx'
```
Esperado: **sin salida**.

Run: `cd /Users/slacker/monitors-main/frontend && npx next build`
Esperado: compilación limpia.

Visualmente, en `/mcc` y `/countries`, en los dos temas: los badges de riesgo son legibles, alto es rojo, medio ámbar, bajo verde, y los contadores que no son de riesgo van en azul de acento.

- [ ] **Step 5: Commit**

```bash
cd /Users/slacker/monitors-main
git add "frontend/src/app/(dashboard)/mcc/page.tsx" "frontend/src/app/(dashboard)/countries/page.tsx"
git commit -m "refactor(marca): escala de riesgo por tokens en MCC y países"
```

---

### Task 5: Migrar severidad (alertas y reglas)

**Files:**
- Modify: `frontend/src/app/(dashboard)/alerts/page.tsx:58-80, 207, 550-560`
- Modify: `frontend/src/app/(dashboard)/rules/page.tsx`

**Interfaces:**
- Consumes: `RISK_CLASSES`, `RISK_FG`, `RISK_BORDER_L`, `STATUS_FG`.

- [ ] **Step 1: Migrar el mapa de severidad de alertas**

El bloque actual usa `border-l-red-500 bg-red-500/5` por nivel. Pasa a:

```tsx
import { RISK_BORDER_L, RISK_CLASSES, RISK_FG } from "@/lib/semantic-colors";

const SEVERITY_CONFIG = {
  critical: {
    variant: "risk-critical" as const,
    color: `${RISK_BORDER_L.critical} bg-risk-critical-bg/20`,
    icon: ShieldAlert,
    label: "Crítica",
  },
  high: {
    variant: "risk-high" as const,
    color: `${RISK_BORDER_L.high} bg-risk-high-bg/40`,
    icon: AlertTriangle,
    label: "Alta",
  },
  medium: {
    variant: "risk-medium" as const,
    color: `${RISK_BORDER_L.medium} bg-risk-medium-bg/40`,
    icon: Shield,
    label: "Media",
  },
  low: {
    variant: "risk-low" as const,
    color: `${RISK_BORDER_L.low} bg-risk-low-bg/40`,
    icon: ShieldCheck,
    label: "Baja",
  },
};
```

Nota la tilde: «Critica» → «Crítica». Conserva los iconos y el resto de claves tal como estén en el archivo; arriba se muestran los valores probables, verifica los reales.

- [ ] **Step 2: Migrar el punto del calendario**

Línea 207, hoy `const dotColor = data?.critical ? "bg-red-500" : data?.high ? "bg-orange-500" : "bg-yellow-500";`

```tsx
const dotColor = data?.critical
  ? "bg-risk-critical-fg"
  : data?.high
    ? "bg-risk-high-fg"
    : "bg-risk-medium-fg";
```

Se usa `-fg` y no `-bg` porque es un punto sólido de señalización, no un fondo de badge: necesita el color saturado.

- [ ] **Step 3: Migrar las tarjetas de estadísticas de alertas**

Las tarjetas usan el patrón `bg-<color>-500/10` para el cuadro del icono y `text-<color>-500` para el icono. Mapeo exacto:

| Actual | Nuevo |
|---|---|
| `bg-red-500/10` + `text-red-500` | `bg-risk-critical-bg` + `text-risk-critical-fg` |
| `bg-orange-500/10` + `text-orange-500` | `bg-risk-high-bg` + `text-risk-high-fg` |
| `bg-yellow-500/5` + `text-yellow-500` | `bg-risk-medium-bg` + `text-risk-medium-fg` |
| `bg-emerald-500` + `text-emerald-500` | `bg-risk-low-bg` + `text-risk-low-fg` |
| `bg-gray-400` | `bg-risk-none-bg` |
| `bg-blue-500/10` + `text-blue-500` | `bg-accent-soft` + `text-accent-fg` |

El azul es la excepción deliberada: la tarjeta de campana cuenta alertas nuevas, que no es un nivel de severidad. Por eso va con el acento de producto y no con la escala de riesgo.

- [ ] **Step 4: Migrar rules/page.tsx**

Este archivo mezcla dos ejes:
- Severidad de regla (`bg-red-100 dark:bg-red-900/30` y equivalentes) → `RISK_CLASSES`
- Estado activa/inactiva (`text-emerald-500` / `text-red-*`) → `STATUS_FG.success` / `STATUS_FG.danger`
- `text-blue-400 dark:text-blue-600` (informativo) → `text-accent-fg`
- `text-amber-500` (aviso) → `STATUS_FG.warning`

- [ ] **Step 5: Verificar**

```bash
cd /Users/slacker/monitors-main/frontend/src
grep -nE '(bg|text|border)-(red|green|amber|emerald|yellow|orange|blue|slate|gray|zinc|rose|purple|indigo)-[0-9]{2,3}' \
  'app/(dashboard)/alerts/page.tsx' 'app/(dashboard)/rules/page.tsx'
```
Esperado: **sin salida**.

Run: `cd /Users/slacker/monitors-main/frontend && npx next build` → limpio.

Visualmente en `/alerts`: una alerta crítica se distingue de una alta en los dos temas, y el borde izquierdo de la tarjeta conserva el color de severidad.

- [ ] **Step 6: Commit**

```bash
cd /Users/slacker/monitors-main
git add "frontend/src/app/(dashboard)/alerts/page.tsx" "frontend/src/app/(dashboard)/rules/page.tsx"
git commit -m "refactor(marca): severidad por tokens de riesgo en alertas y reglas"
```

---

### Task 6: Migrar ejes categóricos (bitácora y cargas)

Aquí el color distingue, no ordena. El error a evitar es tratarlo como severidad.

**Files:**
- Modify: `frontend/src/app/(dashboard)/activity-logs/page.tsx:38-48, 134`
- Modify: `frontend/src/app/(dashboard)/uploads/page.tsx:52-57`

**Interfaces:**
- Consumes: `CATEGORY_CLASSES`, `STATUS_FG` de `@/lib/semantic-colors`.

- [ ] **Step 1: Migrar ACTION_COLORS**

Nueve tipos de acción, nueve colores literales con pares `dark:`. Hay ocho series disponibles, así que dos acciones comparten serie; se agrupan las dos que menos coexisten en pantalla.

```tsx
import { CATEGORY_CLASSES } from "@/lib/semantic-colors";

const ACTION_COLORS: Record<ActivityType, string> = {
  login: CATEGORY_CLASSES[0],
  alert_action: CATEGORY_CLASSES[2],
  rule_create: CATEGORY_CLASSES[1],
  rule_update: CATEGORY_CLASSES[3],
  rule_delete: CATEGORY_CLASSES[5],
  rule_execute: CATEGORY_CLASSES[4],
  upload: CATEGORY_CLASSES[6],
  user_manage: CATEGORY_CLASSES[7],
  mcc_update: CATEGORY_CLASSES[2],
};
```

- [ ] **Step 2: Corregir el respaldo de la línea 134**

Hoy: `const colorClass = ACTION_COLORS[log.action] || "bg-gray-100 text-gray-700";`

```tsx
const colorClass = ACTION_COLORS[log.action] || "bg-surface-2 text-ink-muted";
```

- [ ] **Step 3: Migrar SOURCE_COLORS**

```tsx
import { CATEGORY_CLASSES } from "@/lib/semantic-colors";

const SOURCE_COLORS: Record<SourceType, string> = {
  csv: CATEGORY_CLASSES[0],
  excel: CATEGORY_CLASSES[1],
  json: CATEGORY_CLASSES[2],
  api: CATEGORY_CLASSES[3],
};
```

- [ ] **Step 4: Migrar el resto de uploads/page.tsx**

Quedan `text-red-500` (error de carga) → `STATUS_FG.danger`, y pares `text-*-600 dark:text-*-400` sueltos en iconos, que siguen el mismo mapeo categórico de arriba.

- [ ] **Step 5: Verificar**

```bash
cd /Users/slacker/monitors-main/frontend/src
grep -nE '(bg|text|border)-(red|green|amber|emerald|yellow|orange|blue|slate|gray|zinc|rose|purple|indigo|violet|cyan)-[0-9]{2,3}' \
  'app/(dashboard)/activity-logs/page.tsx' 'app/(dashboard)/uploads/page.tsx'
```
Esperado: **sin salida**. Nota que este grep añade `violet` y `cyan`, que `ACTION_COLORS` usaba y los otros archivos no.

Run: `cd /Users/slacker/monitors-main/frontend && npx next build` → limpio.

- [ ] **Step 6: Commit**

```bash
cd /Users/slacker/monitors-main
git add "frontend/src/app/(dashboard)/activity-logs/page.tsx" "frontend/src/app/(dashboard)/uploads/page.tsx"
git commit -m "refactor(marca): ejes categóricos por series de gráfico"
```

---

### Task 7: Migrar estados y notificaciones

**Files:**
- Modify: `frontend/src/app/(dashboard)/monitors/[id]/page.tsx`
- Modify: `frontend/src/app/(dashboard)/settings/page.tsx`
- Modify: `frontend/src/app/(dashboard)/page.tsx`
- Modify: `frontend/src/lib/use-toast.tsx`

**Interfaces:**
- Consumes: `STATUS_CLASSES`, `STATUS_FG`.

- [ ] **Step 1: Migrar monitors/[id]/page.tsx**

- `bg-emerald-50 dark:bg-emerald-950` + `text-emerald-800 dark:text-emerald-200` → `STATUS_CLASSES.success`
- `bg-red-50 dark:bg-red-950` + `text-red-800 dark:text-red-200` → `STATUS_CLASSES.danger`
- `bg-yellow-100 dark:bg-yellow-900/30` → `STATUS_CLASSES.warning`

- [ ] **Step 2: Migrar settings/page.tsx**

Solo dos: `text-emerald-500` → `text-success-fg`, `text-red-500` → `text-danger-fg`.

- [ ] **Step 3: Migrar el dashboard principal**

`text-emerald-500` → `text-success-fg`; `text-amber-500` → `text-warning-fg`; `text-blue-500` → `text-accent-fg`.

- [ ] **Step 4: Migrar use-toast.tsx**

`bg-emerald-600` → `bg-success-bg text-success-fg border border-success-border`
`bg-red-600` → `bg-danger-bg text-danger-fg border border-danger-border`

Los tokens de estado son fondo suave con texto saturado, no fondo saturado con texto blanco. Si el diseño del toast necesita fondo sólido, usa `bg-accent-solid text-accent-on` para el primario y mantén los estados en la variante suave: es la combinación con contraste verificado.

- [ ] **Step 5: Verificación global de regresión**

Esta es la comprobación que cierra toda la migración de color:

```bash
cd /Users/slacker/monitors-main/frontend/src
grep -rnE '(bg|text|border|ring|from|to)-(red|green|amber|emerald|yellow|orange|blue|slate|gray|zinc|rose|purple|indigo|violet|cyan|pink|teal|lime|sky|fuchsia)-[0-9]{2,3}' .
```
Esperado: **sin salida en todo `src/`**. Si aparece algo, no está terminado.

Run: `cd /Users/slacker/monitors-main/frontend && npx next build` → limpio.

- [ ] **Step 6: Commit**

```bash
cd /Users/slacker/monitors-main
git add "frontend/src/app/(dashboard)/monitors/[id]/page.tsx" "frontend/src/app/(dashboard)/settings/page.tsx" "frontend/src/app/(dashboard)/page.tsx" frontend/src/lib/use-toast.tsx
git commit -m "refactor(marca): estados y notificaciones por tokens semánticos"
```

---

### Task 8: Gráficos, ortografía y nomenclatura

**Files:**
- Modify: `frontend/src/app/(dashboard)/dashboards/[id]/page.tsx:79-88, 811`
- Modify: `frontend/src/components/layout/sidebar.tsx:38-70`
- Modify: varios (barrido de tildes)

**Interfaces:**
- Consumes: tokens `--chart-1..8` y `--bg-canvas` de la Tarea 1.

- [ ] **Step 1: Migrar la paleta de gráficos**

Sustituye el array `CHART_COLORS` (líneas 79-88):

```tsx
// Series de los tokens Thureos: cambian con el tema y están verificadas
// para deuteranopía y protanopía.
const CHART_COLORS = [
  "var(--chart-1)",
  "var(--chart-2)",
  "var(--chart-3)",
  "var(--chart-4)",
  "var(--chart-5)",
  "var(--chart-6)",
  "var(--chart-7)",
  "var(--chart-8)",
];
```

Recharts acepta `var(...)` en `fill` y `stroke` porque los emite como atributos SVG que el navegador resuelve. Los `stopColor` de los gradientes también.

- [ ] **Step 2: Corregir el stroke inválido**

Línea 811, hoy `stroke="hsl(var(--background))"`. La variable `--background` no existía en el `@theme` anterior, así que ese stroke es CSS inválido desde siempre. Y ahora sería peor: tras la Tarea 1 sí existe `--background`, pero como triplete crudo, con lo que resolvería a un color equivocado en vez de a nada.

```tsx
stroke="var(--bg-canvas)"
```

- [ ] **Step 3: Corregir las tildes del sidebar**

En `sidebar.tsx`, las etiquetas de sección y de elementos:

| Actual | Correcto |
|---|---|
| `"Operacion"` | `"Operación"` |
| `"Catalogos"` | `"Catálogos"` |
| `"Paises"` | `"Países"` |
| `"Administracion"` | `"Administración"` |
| `"Configuracion"` | `"Configuración"` |
| `"Bitacora de acceso"` | `"Bitácora de acceso"` |

- [ ] **Step 4: Barrido de tildes en el resto del frontend**

```bash
cd /Users/slacker/monitors-main/frontend/src
grep -rnE '"[^"]*(Operacion|Catalogos|Paises|Administracion|Configuracion|Bitacora|Critica|Analisis|Codigo|Descripcion|Ultimo|Numero|Accion|Periodo|Metrica|Titulo|Categoria|Region|Deteccion|Ejecucion|Version|Sesion|Validacion|Transaccion|Informacion|Auditoria)[^"]*"' .
```

Corrige cada coincidencia: Análisis, Código, Descripción, Último, Número, Acción, Período, Métrica, Título, Categoría, Región, Detección, Ejecución, Versión, Sesión, Validación, Transacción, Información, Auditoría. Revisa cada una en contexto — «Region» puede ser un identificador de código y no texto visible; solo se corrige lo que ve el usuario.

- [ ] **Step 5: Nomenclatura de marca**

```bash
cd /Users/slacker/monitors-main/frontend/src
```

Cada coincidencia en texto visible pasa a «Thureos Compliance», o a «Thureos» a secas cuando es el wordmark del sidebar. El descriptor nunca va solo: nunca escribas «Compliance» como nombre de marca.

- [ ] **Step 6: Verificar**

Run: `cd /Users/slacker/monitors-main/frontend && npx next build` → limpio.

Run: busca el nombre de producto anterior bajo `frontend/src`
Esperado: **sin salida**.

Visualmente en `/dashboards/[id]`: los gráficos usan la paleta Thureos, cambian de color al alternar el tema, y los sectores del gráfico circular tienen un borde del color del lienzo.

- [ ] **Step 7: Commit**

```bash
cd /Users/slacker/monitors-main
git add frontend/src
git commit -m "feat(marca): paleta de gráficos, ortografía y nomenclatura Thureos"
```

---

### Task 9: Los dos hallazgos de auditoría que bloquean el arranque

Backend. No depende de ninguna tarea anterior; se puede ejecutar en paralelo.

**Files:**
- Modify: `backend/.env.example`
- Modify: `backend/internal/config/config.go:23-40`

**Interfaces:**
- Produces: variable de entorno `APP_ENV` con valor por defecto `development`.

- [ ] **Step 1: Alinear los puertos (F1)**

`docker-compose.yml` publica Mongo en 27019 y Redis en 6381; `.env.example` apunta a los puertos por defecto. Se ajusta el `.env.example`, **no el compose**: esos puertos altos evitan choques con otros contenedores de la máquina, y hay varios corriendo.

Reemplaza `backend/.env.example`:

```
PORT=8080
APP_ENV=development

# Puertos publicados por docker-compose.yml, no los estándar:
# el compose remapea para no chocar con otras instancias locales.
MONGO_URI=mongodb://localhost:27019/thureos_compliance
REDIS_URL=redis://localhost:6381

JWT_SECRET=change-this-to-a-secure-secret
ANTHROPIC_API_KEY=sk-ant-xxxxx
ALLOWED_ORIGINS=http://localhost:3000
```

- [ ] **Step 2: Eliminar el fallback silencioso de JWT_SECRET (F2)**

En `backend/internal/config/config.go`, dentro de `Load()`, sustituye la línea `JWTSecret: getEnv("JWT_SECRET", "dev-secret-change-me"),` por una llamada a una función nueva, y añádela al final del archivo:

```go
// loadJWTSecret evita el peor fallo silencioso posible: arrancar en
// producción firmando tokens con un secreto que está en el repositorio.
func loadJWTSecret(appEnv string) string {
	if v := os.Getenv("JWT_SECRET"); v != "" {
		return v
	}
	if appEnv == "production" {
		log.Fatal("FATAL: JWT_SECRET no está definida y APP_ENV=production. " +
			"Defina JWT_SECRET con un valor secreto antes de arrancar.")
	}
	log.Println("ADVERTENCIA: JWT_SECRET no está definida. Usando el secreto de " +
		"desarrollo, que es público. No usar fuera de desarrollo local.")
	return "dev-secret-change-me"
}
```

En `Load()`, el campo pasa a resolverse a partir de `APP_ENV`:

```go
func Load() *Config {
	_ = godotenv.Load()
	appEnv := getEnv("APP_ENV", "development")
	return &Config{
		Port:           getEnv("PORT", "8080"),
		MongoURI:       getEnv("MONGO_URI", "mongodb://localhost:27017/thureos_compliance"),
		RedisURL:       getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:      loadJWTSecret(appEnv),
		AnthropicKey:   getEnv("ANTHROPIC_API_KEY", ""),
		AllowedOrigins: getEnv("ALLOWED_ORIGINS", "http://localhost:3000"),
		SessionTimeout: getEnvDuration("SESSION_TIMEOUT_HOURS", 8),
		AdminEmail:     getEnv("ADMIN_EMAIL", ""),
		AdminPassword:  getEnv("ADMIN_PASSWORD", ""),
	}
}
```

Añade el campo `AppEnv string` a la struct `Config` y asígnalo (`AppEnv: appEnv,`): otros puntos del código querrán consultarlo, y dejarlo fuera obliga a releer la variable de entorno por ahí.

`log` ya está importado en este archivo (lo usa `getEnvDuration`), así que no hace falta tocar los imports.

- [ ] **Step 3: Verificar que compila**

```bash
cd /Users/slacker/monitors-main/backend
go build ./... && go vet ./...
```
Esperado: sin salida en ninguno de los dos.

- [ ] **Step 4: Verificar los dos comportamientos**

```bash
cd /Users/slacker/monitors-main/backend
# Sin JWT_SECRET en desarrollo: advierte y arranca
env -u JWT_SECRET APP_ENV=development go run cmd/server/main.go 2>&1 | head -5
```
Esperado: aparece la línea `ADVERTENCIA: JWT_SECRET no está definida…`. Corta con Ctrl-C.

```bash
# Sin JWT_SECRET en producción: aborta
env -u JWT_SECRET APP_ENV=production go run cmd/server/main.go 2>&1 | head -3
```
Esperado: `FATAL: JWT_SECRET no está definida y APP_ENV=production…` y salida inmediata con código distinto de cero.

- [ ] **Step 5: Verificar la conexión con los puertos del compose**

```bash
cd /Users/slacker/monitors-main
docker compose up -d
cp backend/.env.example backend/.env   # solo si backend/.env no existe
cd backend && go run cmd/server/main.go
```
Esperado: arranca sin errores de conexión a Mongo ni a Redis, y los workers se inician.

En otra terminal: `curl -s http://localhost:8080/health` → `{"status":"ok"}`.

- [ ] **Step 6: Commit**

```bash
cd /Users/slacker/monitors-main
git add backend/.env.example backend/internal/config/config.go
git commit -m "fix(config): alinear puertos con docker-compose y exigir JWT_SECRET en producción"
```

---

### Task 10: Corregir la deriva de CLAUDE.md y verificación final

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Corregir las cinco afirmaciones falsas**

| Dice | Debe decir |
|---|---|
| Roles `admin`, `analyst`, `viewer` | `admin`, `compliance`, `viewer` |
| «Auth: NextAuth.js with role-based access» | Zustand (`stores/auth-store.ts`) + JWT propio del backend |
| Carpeta `docker/` en el árbol | El `docker-compose.yml` está en la raíz |
| Carpeta `docs/` con «Architecture decisions, API specs» | `docs/superpowers/` con specs y planes |
| `MONGO_URI=…27017`, `REDIS_URL=…6379` | 27019 y 6381, los que publica el compose |

- [ ] **Step 2: Documentar el sistema de marca**

Añade una sección nueva a `CLAUDE.md`, después de «Coding Conventions»:

```markdown
## Sistema de marca Thureos

La aplicación es **Thureos Compliance**. «Compliance» es un descriptor funcional,
nunca una marca independiente: no se usa solo.

- Tokens en `frontend/src/styles/tokens/`. **No se editan**: son copias de la
  fuente de verdad en `/Users/slacker/Downloads/tokens/`. Cualquier ajuste va en
  la capa puente `@theme inline` de `globals.css`.
- Tema y marca son atributos de `<html>`: `data-theme="dark|light"` y
  `data-brand="compliance"`. Cambiar de producto es cambiar un atributo.
- **Ningún componente escribe un color literal.** Importa de
  `@/lib/semantic-colors` o usa las utilidades de token.
- Tres ejes de color que no se mezclan: riesgo (`--risk-*`), estado
  (`--success-*`, `--danger-*`…) y categoría (`--chart-1..8`). El acento azul
  identifica al producto y **nunca** comunica severidad.
- Tipografía: Inter 400/500/600/700 para lectura; JetBrains Mono 400/500/600
  para identificadores, montos, hashes y timestamps.
- El arte del logotipo no se modifica, recolorea, redibuja ni revectoriza.
- Manual normativo: `linea_grafica/Thureos-Manual-de-Marca_1.pdf`.
```

- [ ] **Step 3: Verificación final completa**

```bash
# 1. Backend
cd /Users/slacker/monitors-main/backend && go build ./... && go vet ./...

# 2. Frontend
cd /Users/slacker/monitors-main/frontend && npx next build

# 3. Cero colores literales en todo el frontend
cd /Users/slacker/monitors-main/frontend/src
grep -rnE '(bg|text|border|ring|from|to)-(red|green|amber|emerald|yellow|orange|blue|slate|gray|zinc|rose|purple|indigo|violet|cyan|pink|teal|lime|sky|fuchsia)-[0-9]{2,3}' .

# 4. Cero menciones al nombre de producto anterior en texto visible
```

Esperado: 1 y 2 limpios; 3 y 4 **sin salida**.

- [ ] **Step 4: Verificación visual de las 16 rutas**

Con la aplicación levantada, recorre en **los dos temas**: `/`, `/alerts`, `/uploads`, `/monitors`, `/monitors/[id]`, `/rules`, `/dashboards`, `/dashboards/[id]`, `/mcc`, `/countries`, `/users`, `/activity-logs`, `/settings`, `/login`, `/register`.

Por cada una:
1. Ningún texto queda ilegible por bajo contraste.
2. Ningún resto del gris neutro anterior: el fondo es navy en oscuro, blanco en claro.
3. Los badges de riesgo se distinguen entre sí.
4. El foco por teclado es visible con 2 px de separación (recorre con Tab).
5. Los montos, IDs y fechas usan la mono.

- [ ] **Step 5: Verificar el contraste contra los umbrales del manual**

Los tokens traen los ratios medidos, así que no hay que recalcularlos: basta confirmar que ningún componente los elude. El paso 3 ya lo demuestra —si no hay colores literales, todo el color proviene de tokens verificados—.

Comprobación puntual con DevTools sobre `/alerts` en tema oscuro: inspecciona un badge crítico y confirma en el panel de accesibilidad que el contraste supera 4,5:1.

- [ ] **Step 6: Commit**

```bash
cd /Users/slacker/monitors-main
git add CLAUDE.md
git commit -m "docs: corregir deriva de CLAUDE.md y documentar el sistema de marca"
```

---

## Fuera de alcance

Registrado en la auditoría, no se aborda aquí: tests (F6), configuración de ESLint (F5), `gofmt` sobre 16 archivos (F4), rate limiting en el login (F3), vistas de Fraud & Behavior Analytics, y el renombrado de identificadores internos (módulo Go, base Mongo, clave de cola).

## Pendiente con el usuario

Los PNG de marca no tienen versión vectorial. El favicon se genera reduciendo el isotipo y a 16 px pierde el detalle del circuito. Conviene pedir el SVG al diseñador; hasta entonces el lockup horizontal solo se usa sobre navy, porque sus halos de recorte se notan sobre fondo claro.
