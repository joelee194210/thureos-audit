# Centro de Ayuda — Plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Una ayuda en producto con una ficha por pantalla, agrupada como el menú, con capturas reales y un proceso para regenerarlas.

**Architecture:** Dos rutas de Next.js y un módulo de datos estático. Sin backend: el contenido es un arreglo tipado que versiona con el código. Los datos de navegación se extraen a un módulo propio para que la ayuda y el menú compartan una sola fuente de verdad, y para que el test que exige una ficha por pantalla pueda leerla sin importar un componente de React.

**Tech Stack:** Next.js 16 (App Router) · React 19 · TypeScript · Tailwind 4 · shadcn/ui · lucide-react · Vitest

**Spec:** `docs/superpowers/specs/2026-09-10-centro-de-ayuda-design.md`

## Global Constraints

- **Todo el texto visible en español**, con acentos correctos. Es contenido de ayuda: la ortografía es parte del producto.
- **Ningún color literal.** Se usan las utilidades de token del proyecto (`bg-card`, `text-muted-foreground`, `bg-primary/10`, `text-primary`, `border-border`) o `@/lib/semantic-colors`. Nada de `text-gray-900` ni `bg-white`.
- **Funciona en tema oscuro**, que es el que está por defecto, y en claro.
- **Nombres de tests y comentarios en español**, como el resto del repositorio.
- Los íconos salen de `lucide-react`. El ícono de cada ficha es **el mismo que usa esa pantalla en el menú superior**.
- Cada ficha abre como pestaña y su etiqueta es `Ayuda: <título>`, nunca el título solo.
- La estructura retórica de las secciones se respeta: toda ficha abre con **"¿Para qué sirve?"** y sigue con **"¿Quién puede usarlo?"**. Los procedimientos van como lista numerada; las explicaciones como párrafo.
- **No usar `git stash`**: el stack está compartido con otros checkouts y otras sesiones.

---

## Estructura de archivos

| Archivo | Responsabilidad | Tarea |
|---|---|---|
| `frontend/src/lib/nav.ts` | **Nuevo.** Los datos del menú superior, extraídos del componente | 1 |
| `frontend/src/components/layout/top-nav.tsx` | Consume `nav.ts` en vez de su propio arreglo; suma el ítem Ayuda | 1 |
| `frontend/src/lib/help/types.ts` | **Nuevo.** `HelpArticle`, `HelpSection`, `HelpGroup` | 1 |
| `frontend/src/lib/help/articles.ts` | **Nuevo.** El contenido, un registro por módulo | 1, 2, 3 |
| `frontend/src/lib/help/articles.test.ts` | **Nuevo.** Valida los datos y la cobertura del menú | 1 |
| `frontend/src/app/(dashboard)/ayuda/page.tsx` | **Nuevo.** El índice | 1 |
| `frontend/src/app/(dashboard)/ayuda/[slug]/page.tsx` | **Nuevo.** La ficha | 1 |
| `frontend/src/lib/tab-labels.ts` | Suma `ayuda: "Ayuda"` a `SEGMENT_LABELS` | 1 |
| `frontend/scripts/capturar-ayuda.ts` | **Nuevo.** Regenera las capturas | 4 |
| `frontend/public/ayuda/*.png` | **Nuevas.** Las capturas | 4 |

La tarea 1 monta la estructura completa con dos fichas reales. Las tareas 2 y 3 solo agregan entradas a `articles.ts`. La 4 es herramienta y activos.

---

### Task 1: El formato, con dos fichas reales

Al terminar esta tarea la ayuda se puede abrir, recorrer y juzgar. Si el formato no convence, se corrige antes de escribir las otras catorce fichas.

**Files:**
- Create: `frontend/src/lib/nav.ts`
- Modify: `frontend/src/components/layout/top-nav.tsx`
- Create: `frontend/src/lib/help/types.ts`
- Create: `frontend/src/lib/help/articles.ts`
- Create: `frontend/src/lib/help/articles.test.ts`
- Create: `frontend/src/app/(dashboard)/ayuda/page.tsx`
- Create: `frontend/src/app/(dashboard)/ayuda/[slug]/page.tsx`
- Modify: `frontend/src/lib/tab-labels.ts`

**Interfaces:**
- Produces: `NAV_AREAS`, `type NavArea`, `type NavItem` desde `@/lib/nav`; `HelpArticle`, `HelpSection`, `HelpGroup`, `HELP_GROUPS` desde `@/lib/help/types`; `HELP_ARTICLES: HelpArticle[]` y `articuloPorSlug(slug)` desde `@/lib/help/articles`.
- Las tareas 2 y 3 solo agregan elementos a `HELP_ARTICLES`. No cambian ningún tipo ni ningún componente.

- [ ] **Step 1: Extraer los datos del menú**

`top-nav.tsx` declara `const areas: NavArea[]` sin exportarlo, junto con sus tipos. El test de la ayuda necesita esa lista, y no puede importar un componente `"use client"` en un entorno de Vitest en node.

Crear `frontend/src/lib/nav.ts` moviendo **tal cual** los tipos `NavItem` y `NavArea` y el arreglo `areas` que hoy viven en `top-nav.tsx` (líneas 55-90), renombrando el arreglo a `NAV_AREAS` y exportando los tres:

```ts
import type { LucideIcon } from "lucide-react";
import {
  LayoutDashboard, Bell, Upload, Monitor, ShieldCheck, ScanSearch,
  BarChart3, Bot, CreditCard, Globe, Users, ScrollText, Settings,
  CircleHelp,
} from "lucide-react";
import type { Role } from "@/lib/types";

export interface NavItem {
  name: string;
  href: string;
  icon: LucideIcon;
  roles?: Role[];
}

export interface NavArea {
  label: string;
  items: NavItem[];
  roles?: Role[];
}

export const NAV_AREAS: NavArea[] = [ /* el contenido actual de `areas`, sin cambios */ ];
```

**Copiar el arreglo desde el archivo, no reescribirlo de memoria**: cualquier `href` o ícono que se desvíe rompe el menú.

Después, en `top-nav.tsx`: borrar los tipos y el arreglo, e importar `NAV_AREAS` (usándolo donde antes decía `areas`) y los tipos desde `@/lib/nav`. Quitar los imports de íconos que queden sin uso — el compilador los marca.

- [ ] **Step 2: Agregar el ítem Ayuda al menú**

En `frontend/src/lib/nav.ts`, al final de los `items` del área **Operación**:

```ts
      { name: "Ayuda", href: "/ayuda", icon: CircleHelp },
```

Sin `roles`: la ayuda la ve cualquiera con sesión iniciada, incluido un viewer.

- [ ] **Step 3: Los tipos del contenido**

Crear `frontend/src/lib/help/types.ts`:

```ts
import type { LucideIcon } from "lucide-react";

/** Los grupos del índice, en el orden en que se muestran — los cuatro del
 *  menú superior más uno para los conceptos que atraviesan varias pantallas. */
export const HELP_GROUPS = [
  "Operación",
  "Monitoreo",
  "Catálogos",
  "Administración",
  "Conceptos",
] as const;

export type HelpGroup = (typeof HELP_GROUPS)[number];

/**
 * Una sección tiene párrafo O lista, nunca las dos. La unión lo vuelve
 * imposible por tipos: es el defecto que aparece cuando el contenido lo
 * escriben varias manos y una sección termina con un párrafo suelto arriba
 * de una lista que dice lo mismo.
 */
export type HelpSection =
  | { heading: string; body: string }
  | { heading: string; items: string[] };

export interface HelpArticle {
  /** Segmento de la URL: /ayuda/<slug>. Único. */
  slug: string;
  title: string;
  /** El mismo ícono con el que esa pantalla aparece en el menú superior. */
  icon: LucideIcon;
  group: HelpGroup;
  /** Ruta de la pantalla que documenta, o null para las fichas de concepto.
   *  El test la usa para comprobar que ninguna pantalla del menú quedó sin
   *  documentar. */
  route: string | null;
  /** En español llano, no un código de permiso: "Cualquier usuario con
   *  sesión iniciada", "Solo administradores". */
  quienPuede: string;
  /** Una o dos líneas. Se muestra en la tarjeta del índice y bajo el título
   *  de la ficha, así que se escribe corto en vez de truncarse. */
  summary: string;
  /** Archivos dentro de public/ayuda/. Vacío es válido. */
  images: string[];
  sections: HelpSection[];
}
```

- [ ] **Step 4: El contenido, con dos fichas**

Crear `frontend/src/lib/help/articles.ts`. Las otras catorce fichas se agregan en las tareas 2 y 3; la forma queda fijada acá.

```ts
import { Monitor, ShieldCheck } from "lucide-react";
import type { HelpArticle } from "./types";

export const HELP_ARTICLES: HelpArticle[] = [
  {
    slug: "monitores",
    title: "Monitores",
    icon: Monitor,
    group: "Monitoreo",
    route: "/monitors",
    quienPuede:
      "Ver los monitores, cualquier usuario con sesión iniciada. Crearlos, editarlos y cargarles datos, los roles compliance y admin.",
    summary:
      "Cada monitor es una fuente de datos: define de dónde vienen las transacciones, qué columnas traen y cómo se interpretan.",
    images: ["monitores.png"],
    sections: [
      {
        heading: "¿Para qué sirve?",
        body:
          "Un monitor es el contenedor de una fuente de datos. Al cargar el primer archivo, el sistema detecta las columnas y arma el esquema: qué campos hay y de qué tipo es cada uno. A partir de ahí todo lo demás —las reglas, los tableros, el analista de IA— trabaja sobre ese esquema.",
      },
      {
        heading: "¿Quién puede usarlo?",
        body:
          "Cualquier usuario con sesión iniciada puede ver la lista y el detalle de un monitor. Crear uno nuevo, editar su esquema o cargarle datos requiere el rol compliance o admin. Un viewer ve los monitores pero no los modifica.",
      },
      {
        heading: "Cómo crear un monitor",
        items: [
          "Entrá a Monitores y presioná «Nuevo monitor».",
          "Ponele nombre y descripción, y elegí el tipo de fuente: CSV, TXT, Excel, JSON o API.",
          "Si es un archivo delimitado, indicá el separador (coma, punto y coma, pipe o tabulador) y si la primera fila es el encabezado.",
          "Guardá, entrá al monitor y cargá el primer archivo. El esquema se detecta solo.",
        ],
      },
      {
        heading: "Las cuatro pestañas de un monitor",
        items: [
          "Cargar datos: subís archivos. Antes de ingerir, el sistema compara la estructura con el esquema guardado y avisa si no coinciden.",
          "Datos: los registros ya ingeridos, con búsqueda.",
          "Esquema: el tipo de cada campo, los decimales implícitos y el timestamp derivado. Es la pestaña que más conviene entender — mirá la ficha «El esquema de un monitor».",
          "Reglas: las reglas que evalúan este monitor, con acceso directo a ejecutarlas.",
        ],
      },
      {
        heading: "Por qué una carga puede ser rechazada",
        body:
          "Si el archivo trae columnas distintas de las del esquema guardado, la carga se rechaza entera y se informa qué campos faltan o sobran. Es deliberado: aceptar un archivo con la estructura cambiada es la forma más silenciosa de arruinar un histórico. Si el cambio es legítimo, primero ajustá el esquema.",
      },
    ],
  },
  {
    slug: "reglas",
    title: "Reglas",
    icon: ShieldCheck,
    group: "Monitoreo",
    route: "/rules",
    quienPuede:
      "Ver las reglas, cualquier usuario con sesión iniciada. Crearlas, editarlas y ejecutarlas, los roles compliance y admin.",
    summary:
      "Las condiciones que se evalúan sobre los datos de un monitor. Cuando una se cumple, se genera una bandera roja.",
    images: ["reglas.png"],
    sections: [
      {
        heading: "¿Para qué sirve?",
        body:
          "Una regla describe un patrón que hay que detectar. Se evalúa automáticamente cada vez que entran datos nuevos, y también a pedido con «Ejecutar ahora» o de forma programada. Cada coincidencia genera una bandera roja que alguien tiene que investigar.",
      },
      {
        heading: "¿Quién puede usarlo?",
        body:
          "Cualquier usuario con sesión iniciada puede ver las reglas y sus resultados. Crear, editar, activar, desactivar o ejecutar una regla requiere el rol compliance o admin.",
      },
      {
        heading: "Los tres tipos de condición",
        items: [
          "Condiciones simples: se evalúan fila por fila. «El importe es mayor a 5000», «el MCC es 7995».",
          "Condiciones agregadas: agrupan y suman, cuentan o promedian sobre una ventana de tiempo anclada a ahora. «Más de 3 transacciones por tarjeta en 24 horas».",
          "Condiciones de velocidad: miden el tiempo entre una transacción y la anterior de la misma entidad. «Dos cargos de la misma tarjeta separados por menos de 35 segundos». Mirá la ficha «Condiciones de velocidad».",
        ],
      },
      {
        heading: "Cómo crear una regla",
        items: [
          "Entrá a Reglas y presioná «Nueva regla».",
          "Elegí el monitor sobre el que se evalúa y ponele un nombre que describa el patrón, no la implementación.",
          "Agregá las condiciones que necesites: simples, agregadas, de velocidad, o una combinación.",
          "Elegí la severidad y las acciones. Antes de guardar, «Probar contra histórico» te dice cuántas coincidencias habría producido sobre los datos ya cargados.",
        ],
      },
      {
        heading: "Por qué probar contra el histórico",
        body:
          "Una regla demasiado amplia genera cientos de banderas rojas que nadie alcanza a revisar, y el efecto práctico es que se dejan de mirar todas, incluidas las buenas. El botón «Probar contra histórico» corre la regla sobre los datos ya ingeridos sin guardar nada, para que puedas juzgar señal contra ruido antes de activarla.",
      },
      {
        heading: "Si una regla no dispara",
        body:
          "Revisá que el campo de tiempo de un agregado o de una condición de velocidad sea de tipo fecha: si es un número, la comparación no encuentra nada y la regla no avisa de eso. Al guardar, el sistema rechaza las condiciones que no podrían dispararse nunca y explica el motivo.",
      },
    ],
  },
];

/** La ficha de un slug, o undefined si no existe. */
export function articuloPorSlug(slug: string): HelpArticle | undefined {
  return HELP_ARTICLES.find((a) => a.slug === slug);
}
```

- [ ] **Step 5: Escribir el test que falla**

Crear `frontend/src/lib/help/articles.test.ts`:

```ts
import { existsSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { NAV_AREAS } from "@/lib/nav";
import { HELP_ARTICLES, articuloPorSlug } from "./articles";
import { HELP_GROUPS } from "./types";

describe("contenido de la ayuda", () => {
  it("no repite slugs", () => {
    const slugs = HELP_ARTICLES.map((a) => a.slug);
    expect(new Set(slugs).size).toBe(slugs.length);
  });

  it("usa solo grupos declarados", () => {
    for (const a of HELP_ARTICLES) {
      expect(HELP_GROUPS).toContain(a.group);
    }
  });

  it("no deja secciones vacías", () => {
    for (const a of HELP_ARTICLES) {
      expect(a.sections.length).toBeGreaterThan(0);
      for (const s of a.sections) {
        expect(s.heading.trim()).not.toBe("");
        if ("body" in s) expect(s.body.trim()).not.toBe("");
        else expect(s.items.length).toBeGreaterThan(0);
      }
    }
  });

  // Toda ficha abre igual: es la estructura que hace la ayuda predecible.
  it("abre con para qué sirve y quién puede usarlo", () => {
    for (const a of HELP_ARTICLES) {
      expect(a.sections[0].heading).toBe("¿Para qué sirve?");
      expect(a.sections[1].heading).toMatch(/^¿Quién puede usar/);
    }
  });

  it("encuentra por slug y devuelve undefined si no existe", () => {
    expect(articuloPorSlug("monitores")?.title).toBe("Monitores");
    expect(articuloPorSlug("no-existe")).toBeUndefined();
  });

  // El que atrapa el error real: renombrar una captura y olvidar la referencia.
  it("referencia solo imágenes que existen", () => {
    for (const a of HELP_ARTICLES) {
      for (const img of a.images) {
        const ruta = path.join(process.cwd(), "public", "ayuda", img);
        expect(existsSync(ruta), `falta ${img} (ficha ${a.slug})`).toBe(true);
      }
    }
  });
});
```

**Este archivo todavía NO lleva el test de cobertura del menú.** Con solo dos fichas escritas, ese test fallaría por diseño; se agrega en la tarea 2, cuando las trece pantallas estén documentadas. Dejar en su lugar este comentario, para que quien lea el archivo sepa que falta a propósito:

```ts
  // La cobertura del menú (una ficha por pantalla) se verifica en un test
  // aparte que se agrega cuando estén escritas las trece fichas — ver la
  // tarea 2 del plan. Con dos fichas fallaría por diseño.
```

- [ ] **Step 6: Correr el test y ver qué falla**

Run: `cd frontend && npx vitest run src/lib/help/articles.test.ts`
Expected: pasa todo menos `referencia solo imágenes que existen`, que falla porque `public/ayuda/` todavía no existe. Es correcto: las capturas son la tarea 4.

Para que la suite quede verde mientras tanto, **crear los dos PNG como marcadores de posición**: `frontend/public/ayuda/monitores.png` y `reglas.png`, imágenes de 1×1 píxel. La tarea 4 las reemplaza por las capturas reales. Anotarlo con un `README.md` en esa carpeta que diga que son provisorias y que se regeneran con `capturar-ayuda.ts`.

- [ ] **Step 7: El índice**

Crear `frontend/src/app/(dashboard)/ayuda/page.tsx`:

```tsx
"use client";

import Link from "next/link";
import { CircleHelp } from "lucide-react";
import { Header } from "@/components/layout/header";
import { Card, CardContent } from "@/components/ui/card";
import { HELP_ARTICLES } from "@/lib/help/articles";
import { HELP_GROUPS } from "@/lib/help/types";

export default function AyudaPage() {
  return (
    <>
      <Header title="Ayuda" />
      <div className="p-6 space-y-8">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-bold">
            <CircleHelp className="h-6 w-6 text-primary" />
            Centro de Ayuda
          </h1>
          <p className="mt-1 text-muted-foreground">
            Para qué sirve cada pantalla del sistema y cómo hacer las tareas
            más comunes.
          </p>
        </div>

        {HELP_GROUPS.map((grupo) => {
          const fichas = HELP_ARTICLES.filter((a) => a.group === grupo);
          if (fichas.length === 0) return null;
          return (
            <section key={grupo} className="space-y-3">
              <h2 className="text-lg font-semibold">{grupo}</h2>
              <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
                {fichas.map((ficha) => {
                  const Icono = ficha.icon;
                  return (
                    <Link key={ficha.slug} href={`/ayuda/${ficha.slug}`}>
                      <Card className="h-full transition-colors hover:border-primary/50">
                        <CardContent className="flex items-start gap-3 p-5">
                          <div className="shrink-0 rounded-lg bg-primary/10 p-2">
                            <Icono className="h-5 w-5 text-primary" />
                          </div>
                          <div>
                            <h3 className="font-semibold">{ficha.title}</h3>
                            <p className="mt-1 text-sm text-muted-foreground">
                              {ficha.summary}
                            </p>
                          </div>
                        </CardContent>
                      </Card>
                    </Link>
                  );
                })}
              </div>
            </section>
          );
        })}
      </div>
    </>
  );
}
```

- [ ] **Step 8: La ficha**

Crear `frontend/src/app/(dashboard)/ayuda/[slug]/page.tsx`:

```tsx
"use client";

import { use, useEffect } from "react";
import Link from "next/link";
import Image from "next/image";
import { ArrowLeft, CircleHelp } from "lucide-react";
import { Header } from "@/components/layout/header";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { articuloPorSlug } from "@/lib/help/articles";
import { useTabStore } from "@/stores/tab-store";

export default function AyudaFichaPage({
  params,
}: {
  params: Promise<{ slug: string }>;
}) {
  const { slug } = use(params);
  const ficha = articuloPorSlug(slug);
  const registerEntityLabel = useTabStore((s) => s.registerEntityLabel);

  // Sin esto, labelForPath convierte /ayuda/monitores en una pestaña
  // llamada «Monitores», indistinguible de la pantalla real.
  useEffect(() => {
    if (ficha) registerEntityLabel(ficha.slug, `Ayuda: ${ficha.title}`);
  }, [ficha, registerEntityLabel]);

  if (!ficha) {
    return (
      <>
        <Header title="Ayuda" />
        <div className="p-6">
          <Card>
            <CardContent className="flex flex-col items-center gap-3 p-10 text-center">
              <CircleHelp className="h-8 w-8 text-muted-foreground" />
              <p className="font-medium">No existe una ficha para «{slug}»</p>
              <p className="text-sm text-muted-foreground">
                Puede que el enlace esté mal escrito o que la pantalla todavía
                no esté documentada.
              </p>
              <Button asChild variant="outline" className="mt-2">
                <Link href="/ayuda">Volver a la ayuda</Link>
              </Button>
            </CardContent>
          </Card>
        </div>
      </>
    );
  }

  const Icono = ficha.icon;

  return (
    <>
      <Header title="Ayuda" />
      <div className="p-6 space-y-6">
        <div className="flex items-start justify-between gap-4">
          <div>
            <nav className="mb-2 text-sm text-muted-foreground">
              <Link href="/ayuda" className="hover:text-foreground">
                Ayuda
              </Link>
              <span className="mx-1">/</span>
              <span className="text-foreground">{ficha.title}</span>
            </nav>
            <h1 className="flex items-center gap-2 text-2xl font-bold">
              <Icono className="h-6 w-6 text-primary" />
              {ficha.title}
            </h1>
            <p className="mt-1 max-w-2xl text-muted-foreground">
              {ficha.summary}
            </p>
          </div>
          <Button asChild variant="outline">
            <Link href="/ayuda">
              <ArrowLeft className="mr-2 h-4 w-4" />
              Volver a la ayuda
            </Link>
          </Button>
        </div>

        <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
          <div className="space-y-6 lg:col-span-2">
            {ficha.sections.map((seccion, i) => (
              <Card key={i}>
                <CardHeader>
                  <CardTitle className="text-base">{seccion.heading}</CardTitle>
                </CardHeader>
                <CardContent>
                  {"body" in seccion ? (
                    <p className="leading-relaxed text-muted-foreground">
                      {seccion.body}
                    </p>
                  ) : (
                    <ol className="list-inside list-decimal space-y-2 text-muted-foreground">
                      {seccion.items.map((item, j) => (
                        <li key={j}>{item}</li>
                      ))}
                    </ol>
                  )}
                </CardContent>
              </Card>
            ))}
          </div>

          <div className="space-y-6">
            <Card>
              <CardHeader>
                <CardTitle className="text-sm uppercase tracking-wide text-muted-foreground">
                  Quién puede usarlo
                </CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-sm leading-relaxed">{ficha.quienPuede}</p>
              </CardContent>
            </Card>

            {ficha.images.map((img) => (
              <Card key={img} className="overflow-hidden">
                <Image
                  src={`/ayuda/${img}`}
                  alt={`Captura de ${ficha.title}`}
                  width={1440}
                  height={900}
                  className="h-auto w-full"
                />
              </Card>
            ))}
          </div>
        </div>
      </div>
    </>
  );
}
```

> **Nota sobre `params`:** en Next.js 16 los `params` de una página son una promesa y se desenvuelven con `use()`. Verificar cómo lo hace otra ruta dinámica del proyecto (`monitors/[id]`) y seguir ese patrón; si difiere, manda el del proyecto.

- [ ] **Step 9: La etiqueta de la pestaña**

En `frontend/src/lib/tab-labels.ts`, agregar a `SEGMENT_LABELS`:

```ts
  ayuda: "Ayuda",
```

- [ ] **Step 10: Verificar**

```bash
cd frontend && npx tsc --noEmit && npm run lint && npm run test:run
```
Expected: sin errores; los tests nuevos pasan junto con los 30 existentes.

Verificación manual, con el frontend levantado:

1. El ítem **Ayuda** aparece al final del menú Operación y abre el índice.
2. El índice muestra el grupo **Monitoreo** con las dos tarjetas; los demás grupos no aparecen porque todavía no tienen fichas.
3. Al entrar a una ficha, la pestaña dice **«Ayuda: Monitores»**, no «Monitores».
4. La ficha se ve bien en **tema oscuro y en claro** (el conmutador está en la barra superior).
5. En una ventana angosta las columnas se apilan y las capturas quedan **después** del texto.
6. `/ayuda/loquesea` muestra el estado vacío con el botón de volver, no una pantalla de error.

- [ ] **Step 11: Commit**

```bash
git add frontend/src/lib/nav.ts frontend/src/components/layout/top-nav.tsx frontend/src/lib/help frontend/src/lib/tab-labels.ts "frontend/src/app/(dashboard)/ayuda" frontend/public/ayuda
git commit -m "feat(ayuda): centro de ayuda con el formato y las dos primeras fichas"
```

---

### Task 2: Las once fichas de pantalla restantes

Solo contenido: se agregan entradas a `HELP_ARTICLES` y se cierra el test de cobertura. No se toca ningún componente.

**Files:**
- Modify: `frontend/src/lib/help/articles.ts`
- Modify: `frontend/src/lib/help/articles.test.ts`
- Create: `frontend/public/ayuda/*.png` (marcadores de 1×1, como en la tarea 1)

**Interfaces:**
- Consumes: `HelpArticle`, `HelpGroup` (tarea 1). No los modifica.

- [ ] **Step 1: Escribir las once fichas**

Una entrada por pantalla, con el ícono que esa pantalla usa en `NAV_AREAS` y la ruta en `route`:

| slug | title | group | route | ícono |
|---|---|---|---|---|
| `dashboard` | Dashboard | Operación | `/` | `LayoutDashboard` |
| `banderas-rojas` | Banderas rojas | Operación | `/red-flags` | `Bell` |
| `cargas` | Cargas | Operación | `/uploads` | `Upload` |
| `screening` | Screening | Monitoreo | `/screening` | `ScanSearch` |
| `dashboards` | Dashboards | Monitoreo | `/dashboards` | `BarChart3` |
| `analista-ia` | Analista IA | Monitoreo | `/chatbot` | `Bot` |
| `mcc` | MCC | Catálogos | `/mcc` | `CreditCard` |
| `paises` | Países | Catálogos | `/countries` | `Globe` |
| `usuarios` | Usuarios | Administración | `/users` | `Users` |
| `bitacora-de-acceso` | Bitácora de acceso | Administración | `/activity-logs` | `ScrollText` |
| `configuracion` | Configuración | Administración | `/settings` | `Settings` |

**Sobre el contenido:** este plan no dicta el texto de las once fichas, pero sí lo que cada una tiene que responder, en este orden:

1. **¿Para qué sirve?** — qué problema resuelve la pantalla, en dos o tres oraciones, sin describir botones.
2. **¿Quién puede usarlo?** — en lenguaje llano. Los roles son `admin`, `compliance` y `viewer`; el menú Administración es solo `admin`, así que sus tres fichas lo dicen explícitamente.
3. **Al menos una sección de procedimiento**, como lista numerada, con la tarea más común de esa pantalla.
4. **Al menos una sección que anticipe una confusión real**: por qué algo se rechaza, qué significa un estado, qué hacer si no aparece lo que se espera.

Escribir mirando la pantalla, no imaginándola: abrir cada ruta en el navegador antes de redactar su ficha. Una ayuda que describe una pantalla que no existe es peor que no tener ayuda.

Para cada ficha, crear también su PNG marcador de 1×1 en `public/ayuda/` con el nombre del slug, para que el test de imágenes siga pasando.

- [ ] **Step 2: Cerrar el test de cobertura del menú**

Reemplazar el comentario que dejó la tarea 1 por el test, y borrar el comentario:

```ts
  // La única defensa contra que la ayuda se quede atrás del producto: si
  // alguien agrega una pantalla al menú y no la documenta, esto lo dice.
  it("tiene una ficha por cada pantalla del menú", () => {
    const rutasDocumentadas = new Set(
      HELP_ARTICLES.map((a) => a.route).filter((r): r is string => r !== null),
    );
    const sinFicha = NAV_AREAS.flatMap((area) => area.items)
      .filter((item) => item.href !== "/ayuda")
      .filter((item) => !rutasDocumentadas.has(item.href))
      .map((item) => `${item.name} (${item.href})`);

    expect(sinFicha, `pantallas sin ficha de ayuda: ${sinFicha.join(", ")}`).toEqual([]);
  });
```

- [ ] **Step 3: Verificar y commitear**

```bash
cd frontend && npx tsc --noEmit && npm run lint && npm run test:run
```
Expected: verde, con el test de cobertura ahora incluido.

Manual: el índice muestra los cuatro grupos con sus trece tarjetas, y cada tarjeta abre su ficha.

```bash
git add frontend/src/lib/help frontend/public/ayuda
git commit -m "docs(ayuda): fichas de las once pantallas restantes"
```

---

### Task 3: Las tres fichas de conceptos

Las de más valor: no describen una pantalla sino un mecanismo que atraviesa varias, y son donde la gente se traba.

**Files:**
- Modify: `frontend/src/lib/help/articles.ts`

**Interfaces:**
- Consumes: `HelpArticle` (tarea 1). `route: null` en las tres, así que el test de cobertura del menú las ignora.

- [ ] **Step 1: Escribir las tres fichas**

Grupo `"Conceptos"`, `route: null`, `images: []` salvo que una captura aporte algo real.

**`esquema-de-un-monitor`** — *El esquema de un monitor* (ícono `FileText`). Tiene que explicar:

- Que el esquema se detecta al cargar el primer archivo y define el tipo de cada campo.
- **Decimales implícitos**: el archivo escribe `500000` y quiere decir `5000,00`. Al marcar el campo con 2 decimales, la ingesta divide por 100 **al guardar**, así que de ahí en más los umbrales de las reglas se escriben en la escala convertida: `5000`, no `500000`. El ejemplo numérico va en el texto; es lo que hace entendible el concepto.
- Que cambiar los decimales de un campo **que ya tiene datos** ofrece convertir los registros existentes, y que esa conversión no se deshace sola. Que si se rechaza la conversión, no se guarda nada.
- **Timestamp derivado**: cuando la fecha y la hora vienen en dos columnas numéricas separadas, se combinan en un campo de fecha real. Por qué hace falta: sin un campo de tipo fecha no se puede medir tiempo entre transacciones. Mencionar que la hora `21517` se reconstruye como `02:15:17` — el cero de la izquierda se pierde al guardarse como número, y el sistema lo repone.
- Que los decimales implícitos solo aplican a fuentes de archivo (CSV, TXT, Excel); en JSON y API los valores se guardan tal como vienen.

**`condiciones-de-velocidad`** — *Condiciones de velocidad* (ícono `Timer`). Tiene que explicar:

- La diferencia con un agregado, que es el punto entero: en un agregado la ventana está anclada a «ahora» y se cuenta cuántos eventos caen dentro; en una condición de velocidad se mide la distancia entre un evento y el inmediatamente anterior de la misma entidad.
- El ejemplo concreto: dos cargos de la misma tarjeta separados por menos de 35 segundos.
- Que el campo de tiempo tiene que ser de tipo fecha, y que si el monitor no tiene ninguno hay que configurar el timestamp derivado primero.
- Qué hace el filtro previo: reduce el universo **antes** de comparar, para expresar «solo transacciones de casino» sin mezclarlas con el resto.
- Que por ahora se miden pares, no rachas más largas.

**`generar-reglas-con-ia`** — *Generar reglas con IA* (ícono `Sparkles`). Tiene que explicar:

- Qué hace: se describe el criterio en castellano y el sistema propone reglas armadas sobre el esquema del monitor.
- **El modo guiado**: elegir primero los campos sobre los que va la regla. Por qué conviene — es lo que evita que el modelo invente nombres de campo, que es el motivo más común de que no salga ninguna sugerencia.
- Que las sugerencias que no se podrían guardar se descartan, y que la pantalla dice cuáles y por qué. Que ese aviso es información útil, no un error.
- Que una sugerencia aplicada crea una regla normal, editable como cualquier otra, y que conviene probarla contra el histórico antes de dejarla activa.

- [ ] **Step 2: Verificar y commitear**

```bash
cd frontend && npx tsc --noEmit && npm run lint && npm run test:run
```
Expected: verde. El grupo **Conceptos** aparece al final del índice con sus tres tarjetas.

```bash
git add frontend/src/lib/help
git commit -m "docs(ayuda): fichas de decimales, velocidad y generación con IA"
```

---

### Task 4: El script de capturas, y las capturas

**Files:**
- Create: `frontend/scripts/capturar-ayuda.ts`
- Replace: `frontend/public/ayuda/*.png` (los marcadores, por capturas reales)
- Modify: `frontend/package.json` (un script npm)

**Interfaces:**
- Consumes: `HELP_ARTICLES` — la lista de rutas a capturar sale del propio contenido, no de una lista aparte que se desincronice.

- [ ] **Step 1: El script**

`frontend/scripts/capturar-ayuda.ts`, con Playwright. Qué tiene que hacer:

1. Recorrer `HELP_ARTICLES`, quedarse con las que tienen `route !== null` **y** al menos una imagen, y para cada una abrir su ruta.
2. Ventana de 1440 de ancho.
3. Autenticarse poniendo un token en `localStorage` bajo la clave `token`, tomado de una variable de entorno (`AYUDA_TOKEN`). **Nunca una contraseña en el script.**
4. Esperar a que la pantalla tenga contenido antes de capturar — no un `sleep` fijo, sino la aparición del encabezado de la página.
5. Guardar en `public/ayuda/<primera imagen de la ficha>.png`.
6. **Fallar ruidosamente** si una ruta no responde, si la pantalla queda vacía, o si la captura sale en blanco. Un script de capturas que falla en silencio produce documentación falsa, que es peor que ninguna.

Al final, imprimir qué capturó y qué no.

Agregar a `package.json`: `"capturar-ayuda": "tsx scripts/capturar-ayuda.ts"` (verificar si el proyecto ya tiene `tsx` o equivalente; si no, usar el runner que ya use).

- [ ] **Step 2: Sembrar y capturar**

Levantar backend y frontend contra una base local, con datos de prueba suficientes para que ninguna pantalla salga vacía: al menos un monitor con datos, un par de reglas, algunas banderas rojas, y entradas en los catálogos.

**Los datos de las capturas son inventados**, no un recorte de producción: son pantallas de un sistema de prevención de lavado y quedan versionadas en el repositorio. Nombres, tarjetas y montos ficticios.

Correr el script y revisar **una por una** las imágenes generadas: que no salga un estado de carga, ni una tabla vacía, ni un diálogo abierto por accidente.

- [ ] **Step 3: Verificar y commitear**

```bash
cd frontend && npm run test:run
```
Expected: el test de imágenes sigue pasando, ahora contra archivos reales.

Manual: recorrer las fichas y confirmar que cada captura corresponde a su pantalla y se lee a ese tamaño.

```bash
git add frontend/scripts/capturar-ayuda.ts frontend/package.json frontend/public/ayuda
git commit -m "feat(ayuda): script de capturas y las capturas de cada ficha"
```

---

## Verificación final

Con todo junto, recorrer una vez el índice y tres fichas —una de pantalla, una de administración y una de concepto— en tema oscuro y claro, en escritorio y en una ventana angosta, comprobando que:

- Los cinco grupos aparecen en el orden del menú, con Conceptos al final.
- Ninguna captura desborda su tarjeta.
- En angosto, las columnas se apilan y el texto queda antes que las imágenes.
- Cada pestaña de ficha dice «Ayuda: …».
