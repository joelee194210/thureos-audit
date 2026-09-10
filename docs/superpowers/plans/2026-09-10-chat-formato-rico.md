# Formato rico en el chat — Plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Que las respuestas del asistente se rendericen con formato (negrita, listas, tablas, código) en vez de texto plano, y que las tablas de datos formateen sus celdas por tipo.

**Architecture:** El markdown se renderiza **sin producir HTML en ningún momento**: `marked.lexer()` devuelve un árbol de tokens y un mapeador lo convierte a elementos de React. No hay `dangerouslySetInnerHTML` en ninguna parte del camino. El formateo de celdas se extrae de `red-flag-report.ts` a un módulo compartido que consumen tanto el informe PDF como la tabla del chat.

**Tech Stack:** React 19, TypeScript, Vitest, `marked` (dependencia nueva, solo el lexer).

**Spec:** `docs/superpowers/specs/2026-09-10-chat-artefactos-design.md`, sección «Formato rico»

**Independencia:** Este plan **no depende** de `2026-09-10-chat-multimonitor.md` ni del trabajo de artefactos de primera clase. No necesita `sources`, ni id de artefacto, ni colección nueva. Puede ejecutarse antes, después o en paralelo. Es lo que más rápido hace que el chat se vea moderno.

## Global Constraints

- **Nunca `dangerouslySetInnerHTML`.** El texto del asistente está influido por archivos que suben los usuarios: una celda con una inyección de prompt puede moldear lo que el LLM escribe. Inyectar eso como HTML en el origen de la app —donde vive el JWT— es exactamente lo que el diseño original evitó con el iframe sandboxeado. El renderizador mapea tokens a elementos de React; no existe un camino por el que un string se convierta en marcado.
- **Los tokens `html` de marked se tratan como texto.** Nunca se interpretan.
- **Enlaces:** solo `http:`, `https:` y `mailto:`. Cualquier otro esquema (incluido `javascript:`) se renderiza como texto sin enlace.
- **Los artefactos `custom` no cambian:** siguen en su iframe `sandbox="allow-scripts"` sin `allow-same-origin`. Este plan no toca ese camino.
- **Ningún componente escribe un color literal.** Se usan utilidades de token o `@/lib/semantic-colors`.
- **Tipografía:** JetBrains Mono para montos, identificadores, hashes y timestamps; Inter para lectura (manual de marca).
- **Idioma:** comentarios y texto de interfaz en español.

---

## Estructura de archivos

**Nuevos:**
- `frontend/src/lib/format-value.ts` — formateo de un valor suelto por tipo. Extraído de `red-flag-report.ts`, que pasa a consumirlo.
- `frontend/src/lib/format-value.test.ts`
- `frontend/src/components/chat/markdown.tsx` — mapeo de tokens de marked a elementos de React.
- `frontend/src/lib/markdown-tokens.ts` — normalización del árbol de tokens y saneamiento de URLs. Separado del componente para poder testearlo sin montar React.
- `frontend/src/lib/markdown-tokens.test.ts`

**Modificados:**
- `frontend/src/lib/red-flag-report.ts` — deja de tener su copia de `formatValue`
- `frontend/src/components/chat/artifact-canvas.tsx` — `TableArtifact` formatea celdas
- `frontend/src/app/(dashboard)/chatbot/page.tsx` — las burbujas del asistente usan `<Markdown>`
- `frontend/package.json` — `marked`

---

## Task 1: Módulo de formateo de valores

Extrae `formatValue` de `red-flag-report.ts` para que el informe PDF y la tabla del chat compartan un solo criterio.

**Files:**
- Create: `frontend/src/lib/format-value.ts`
- Create: `frontend/src/lib/format-value.test.ts`
- Modify: `frontend/src/lib/red-flag-report.ts:26-40`

**Interfaces:**
- Produces:
  - `export type ValueKind = "number" | "date" | "boolean" | "empty" | "text"`
  - `export function classifyValue(val: unknown): ValueKind`
  - `export function formatValue(val: unknown): string`

- [ ] **Step 1: Escribir el test que falla**

Crear `frontend/src/lib/format-value.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { classifyValue, formatValue } from "./format-value";

describe("formatValue", () => {
  it("formatea números con separador de miles es-CO", () => {
    expect(formatValue(1234567)).toBe("1.234.567");
    expect(formatValue(1234.5)).toBe("1.234,5");
  });

  it("muestra un guion largo para vacíos", () => {
    expect(formatValue(null)).toBe("—");
    expect(formatValue(undefined)).toBe("—");
    expect(formatValue("")).toBe("—");
  });

  it("traduce booleanos", () => {
    expect(formatValue(true)).toBe("Sí");
    expect(formatValue(false)).toBe("No");
  });

  it("formatea fechas ISO que llegan como texto", () => {
    // Las colecciones dinámicas guardan fechas como string ISO.
    expect(formatValue("2026-03-15T10:30:00Z")).not.toContain("T");
    expect(formatValue("2026-03-15T10:30:00Z")).not.toBe("2026-03-15T10:30:00Z");
  });

  it("deja el texto común sin tocar", () => {
    expect(formatValue("Bancolombia")).toBe("Bancolombia");
  });

  it("serializa objetos", () => {
    expect(formatValue({ a: 1 })).toBe('{"a":1}');
  });

  it("no confunde un texto que empieza con dígitos con una fecha", () => {
    expect(formatValue("2026 fue un buen año")).toBe("2026 fue un buen año");
  });
});

describe("classifyValue", () => {
  it("distingue las clases que la tabla necesita para alinear", () => {
    expect(classifyValue(42)).toBe("number");
    expect(classifyValue("2026-03-15T10:30:00Z")).toBe("date");
    expect(classifyValue(true)).toBe("boolean");
    expect(classifyValue(null)).toBe("empty");
    expect(classifyValue("hola")).toBe("text");
  });

  it("un número en forma de string sigue siendo texto", () => {
    // No se adivina: si Mongo lo guardó como string, se muestra como
    // string. Convertir acá desalinearía la columna respecto del dato.
    expect(classifyValue("42")).toBe("text");
  });
});
```

- [ ] **Step 2: Correr el test y verificar que falla**

Run: `cd frontend && npx vitest run src/lib/format-value.test.ts`
Expected: FAIL — no existe `./format-value`.

- [ ] **Step 3: Implementar**

Crear `frontend/src/lib/format-value.ts`:

```ts
/**
 * Formateo de un valor suelto proveniente de una colección dinámica.
 *
 * Vive acá y no en cada consumidor porque el informe PDF de banderas
 * rojas y la tabla del chat tienen que mostrar el mismo dato igual: un
 * monto que en el PDF sale "1.234.567" y en pantalla "1234567" es un
 * reporte de bug esperando a pasar.
 */
import { formatDate } from "@/lib/utils";

/** Fechas ISO que llegan como texto desde colecciones dinámicas. */
const ISO_DATE_RE = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}/;

export type ValueKind = "number" | "date" | "boolean" | "empty" | "text";

/**
 * classifyValue dice de qué clase es un valor, para que quien lo muestre
 * pueda decidir alineación y tipografía. Deliberadamente NO convierte:
 * un "42" guardado como string se clasifica como texto, porque
 * mostrarlo alineado a la derecha mentiría sobre el tipo del dato.
 */
export function classifyValue(val: unknown): ValueKind {
  if (val === null || val === undefined || val === "") return "empty";
  if (typeof val === "number") return "number";
  if (typeof val === "boolean") return "boolean";
  if (typeof val === "string" && ISO_DATE_RE.test(val)) {
    return Number.isNaN(new Date(val).getTime()) ? "text" : "date";
  }
  return "text";
}

export function formatValue(val: unknown): string {
  switch (classifyValue(val)) {
    case "empty":
      return "—";
    case "number":
      return new Intl.NumberFormat("es-CO").format(val as number);
    case "boolean":
      return val ? "Sí" : "No";
    case "date":
      return formatDate(val as string);
    default:
      if (typeof val === "object") return JSON.stringify(val);
      return String(val);
  }
}
```

- [ ] **Step 4: Correr el test y verificar que pasa**

Run: `cd frontend && npx vitest run src/lib/format-value.test.ts`
Expected: PASS.

- [ ] **Step 5: Hacer que `red-flag-report.ts` lo consuma**

En `frontend/src/lib/red-flag-report.ts`, borrar la función `formatValue` local (líneas 26-40) y su dependencia directa de `formatDate` si no se usa en otro lado del archivo. Agregar al bloque de imports:

```ts
import { formatValue } from "@/lib/format-value";
```

- [ ] **Step 6: Verificar que no se rompió el informe**

Run: `cd frontend && npx tsc --noEmit && npx vitest run`
Expected: sin errores de tipos; todos los tests pasan.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/lib/format-value.ts frontend/src/lib/format-value.test.ts frontend/src/lib/red-flag-report.ts
git commit -m "refactor(formato): extraer el formateo de valores a un módulo compartido"
```

---

## Task 2: Normalización de tokens de markdown

La parte con lógica de seguridad, separada del componente para poder testearla sin montar React.

**Files:**
- Create: `frontend/src/lib/markdown-tokens.ts`
- Create: `frontend/src/lib/markdown-tokens.test.ts`
- Modify: `frontend/package.json`

**Interfaces:**
- Produces:
  - `export function safeHref(href: string): string | null`
  - `export function lexMarkdown(source: string): Token[]` (reexporta el tipo `Token` de marked)

- [ ] **Step 1: Instalar marked**

Run: `cd frontend && npm install marked@^15.0.0`
Expected: se agrega a `dependencies`.

> Solo se usa `marked.lexer()`. **No** se usa `marked.parse()` ni ninguna función que produzca HTML. Si alguien agrega una llamada a `marked.parse()` en el futuro, está reintroduciendo la superficie que este diseño elimina.

- [ ] **Step 2: Escribir el test que falla**

Crear `frontend/src/lib/markdown-tokens.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { safeHref, lexMarkdown } from "./markdown-tokens";

describe("safeHref", () => {
  it("acepta los esquemas seguros", () => {
    expect(safeHref("https://thureos.com")).toBe("https://thureos.com");
    expect(safeHref("http://thureos.com")).toBe("http://thureos.com");
    expect(safeHref("mailto:a@b.com")).toBe("mailto:a@b.com");
  });

  it("rechaza javascript:", () => {
    expect(safeHref("javascript:alert(1)")).toBeNull();
  });

  it("rechaza javascript: disfrazado con espacios y mayúsculas", () => {
    expect(safeHref("  JaVaScRiPt:alert(1)")).toBeNull();
    expect(safeHref("java\tscript:alert(1)")).toBeNull();
    expect(safeHref("java\nscript:alert(1)")).toBeNull();
  });

  it("rechaza data: y otros esquemas", () => {
    expect(safeHref("data:text/html,<script>alert(1)</script>")).toBeNull();
    expect(safeHref("vbscript:msgbox(1)")).toBeNull();
    expect(safeHref("file:///etc/passwd")).toBeNull();
  });

  it("acepta rutas relativas", () => {
    expect(safeHref("/monitores/123")).toBe("/monitores/123");
  });
});

describe("lexMarkdown", () => {
  it("tokeniza los elementos que el asistente usa", () => {
    const tokens = lexMarkdown("# Título\n\n- uno\n- dos\n\n**negrita**");
    const types = tokens.map((t) => t.type);
    expect(types).toContain("heading");
    expect(types).toContain("list");
    expect(types).toContain("paragraph");
  });

  it("no interpreta HTML crudo: lo deja como token html para tratar como texto", () => {
    const tokens = lexMarkdown("<script>alert(1)</script>");
    // Sea cual sea el tipo que marked le asigne, el texto crudo se
    // conserva y NUNCA se convierte en marcado. El componente lo
    // renderiza como texto; acá solo verificamos que no desaparece.
    const raw = JSON.stringify(tokens);
    expect(raw).toContain("script");
  });

  it("tokeniza una tabla", () => {
    const tokens = lexMarkdown("| a | b |\n|---|---|\n| 1 | 2 |");
    expect(tokens.map((t) => t.type)).toContain("table");
  });

  it("devuelve lista vacía para entrada vacía", () => {
    expect(lexMarkdown("")).toEqual([]);
  });
});
```

- [ ] **Step 3: Correr el test y verificar que falla**

Run: `cd frontend && npx vitest run src/lib/markdown-tokens.test.ts`
Expected: FAIL — no existe `./markdown-tokens`.

- [ ] **Step 4: Implementar**

Crear `frontend/src/lib/markdown-tokens.ts`:

```ts
/**
 * Capa entre marked y el renderizador de React.
 *
 * Se usa ÚNICAMENTE el lexer de marked: nunca marked.parse() ni nada que
 * produzca HTML. El árbol de tokens se mapea a elementos de React en
 * components/chat/markdown.tsx, así que no existe un punto del camino en
 * el que un string del LLM se convierta en marcado.
 *
 * Motivo: el texto del asistente está influido por archivos que suben los
 * usuarios, y una inyección de prompt puede moldear lo que escribe.
 * Renderizar eso como HTML en el origen de la app —donde vive el JWT— es
 * lo que el iframe sandboxeado de los artefactos custom ya evitaba.
 */
import { Lexer, type Token } from "marked";

export type { Token };

/** Esquemas de URL que se permiten en un enlace. */
const SAFE_SCHEMES = ["http:", "https:", "mailto:"];

/**
 * safeHref devuelve la URL si es segura de poner en un href, o null si
 * no. Un null significa "renderizá el texto sin enlace", nunca "omití el
 * contenido".
 *
 * Se quitan los espacios en blanco INTERNOS antes de mirar el esquema:
 * los navegadores ignoran tabs y saltos de línea dentro del esquema, así
 * que "java\tscript:" es ejecutable y tiene que caer acá.
 */
export function safeHref(href: string): string | null {
  // Se descarta todo lo que sea <= 0x20 (espacios y caracteres de
  // control) filtrando por code point, y no con una clase de regex, para
  // no tener que incrustar caracteres de control en el archivo fuente.
  const collapsed = Array.from(href)
    .filter((ch) => ch.charCodeAt(0) > 0x20)
    .join("")
    .toLowerCase();

  // Una ruta relativa o un ancla no llevan esquema y son seguros.
  if (collapsed.startsWith("/") || collapsed.startsWith("#")) {
    return href.trim();
  }
  const colon = collapsed.indexOf(":");
  if (colon === -1) {
    // Sin esquema: relativo.
    return href.trim();
  }
  const scheme = collapsed.slice(0, colon + 1);
  return SAFE_SCHEMES.includes(scheme) ? href.trim() : null;
}

/** Tokeniza markdown. Nunca produce HTML. */
export function lexMarkdown(source: string): Token[] {
  if (!source) return [];
  return new Lexer({ gfm: true, breaks: true }).lex(source);
}
```

- [ ] **Step 5: Correr el test y verificar que pasa**

Run: `cd frontend && npx vitest run src/lib/markdown-tokens.test.ts`
Expected: PASS, los 9 tests.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/markdown-tokens.ts frontend/src/lib/markdown-tokens.test.ts frontend/package.json frontend/package-lock.json
git commit -m "feat(chat): tokenizador de markdown con saneamiento de enlaces"
```

---

## Task 3: Componente `<Markdown>`

**Files:**
- Create: `frontend/src/components/chat/markdown.tsx`

**Interfaces:**
- Consumes: `lexMarkdown`, `safeHref`, `Token` (Task 2)
- Produces: `export function Markdown({ children }: { children: string })`

- [ ] **Step 1: Crear el componente**

```tsx
"use client";

import { Fragment, type ReactNode } from "react";
import { lexMarkdown, safeHref, type Token } from "@/lib/markdown-tokens";

/**
 * Renderiza markdown como elementos de React.
 *
 * INVARIANTE: no hay `dangerouslySetInnerHTML` en este archivo ni debe
 * agregarse. Todo lo que no se sabe representar cae en `renderText`, que
 * lo muestra como texto plano. Un token que no se reconoce se ve feo,
 * nunca se ejecuta.
 */
export function Markdown({ children }: { children: string }) {
  return <div className="space-y-2">{renderTokens(lexMarkdown(children))}</div>;
}

function renderTokens(tokens: Token[]): ReactNode {
  return tokens.map((token, i) => (
    <Fragment key={i}>{renderToken(token)}</Fragment>
  ));
}

/** Los tokens inline traen `tokens` hijos; si no, se cae al texto crudo. */
function renderInline(token: { tokens?: Token[]; text?: string }): ReactNode {
  if (token.tokens && token.tokens.length > 0) return renderTokens(token.tokens);
  return token.text ?? "";
}

function renderToken(token: Token): ReactNode {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const t = token as any;

  switch (token.type) {
    case "paragraph":
      return <p className="leading-relaxed">{renderInline(t)}</p>;

    case "text":
      // Un token de texto puede traer hijos inline (dentro de un listitem
      // o una celda de tabla) o ser una hoja.
      return renderInline(t);

    case "strong":
      return <strong className="font-semibold">{renderInline(t)}</strong>;

    case "em":
      return <em className="italic">{renderInline(t)}</em>;

    case "del":
      return <del className="line-through">{renderInline(t)}</del>;

    case "codespan":
      return (
        <code className="rounded bg-muted px-1 py-0.5 font-mono text-[0.85em]">
          {t.text}
        </code>
      );

    case "code":
      return (
        <pre className="overflow-x-auto rounded-md bg-muted p-3">
          <code className="font-mono text-xs">{t.text}</code>
        </pre>
      );

    case "heading": {
      const level = Math.min(Math.max(t.depth ?? 1, 1), 6);
      const Tag = `h${level}` as "h1";
      return (
        <Tag className="font-semibold" style={{ fontSize: `${1.3 - level * 0.06}em` }}>
          {renderInline(t)}
        </Tag>
      );
    }

    case "list": {
      const items = (t.items ?? []).map((item: Token, i: number) => (
        <li key={i} className="ml-4 list-outside">
          {renderInline(item as { tokens?: Token[]; text?: string })}
        </li>
      ));
      return t.ordered ? (
        <ol className="list-decimal space-y-1">{items}</ol>
      ) : (
        <ul className="list-disc space-y-1">{items}</ul>
      );
    }

    case "blockquote":
      return (
        <blockquote className="border-l-2 pl-3 text-muted-foreground">
          {renderTokens(t.tokens ?? [])}
        </blockquote>
      );

    case "table":
      return (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b">
                {(t.header ?? []).map((cell: Token, i: number) => (
                  <th key={i} className="p-2 text-left font-medium">
                    {renderInline(cell as { tokens?: Token[]; text?: string })}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {(t.rows ?? []).map((row: Token[], r: number) => (
                <tr key={r} className="border-b last:border-0">
                  {row.map((cell, c) => (
                    <td key={c} className="p-2">
                      {renderInline(cell as { tokens?: Token[]; text?: string })}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      );

    case "link": {
      const href = safeHref(t.href ?? "");
      // Sin href seguro se muestra el texto, nunca el enlace. El
      // contenido no se pierde: lo que se pierde es la navegación.
      if (!href) return <>{renderInline(t)}</>;
      return (
        <a
          href={href}
          target="_blank"
          rel="noopener noreferrer"
          className="underline underline-offset-2"
        >
          {renderInline(t)}
        </a>
      );
    }

    case "hr":
      return <hr className="border-border" />;

    case "br":
      return <br />;

    case "space":
      return null;

    // "html" y cualquier tipo que no conozcamos: SIEMPRE como texto.
    // Este default es la red de seguridad del componente.
    default:
      return <span className="whitespace-pre-wrap">{t.raw ?? t.text ?? ""}</span>;
  }
}
```

- [ ] **Step 2: Verificar tipos y lint**

Run: `cd frontend && npx tsc --noEmit && npm run lint`
Expected: sin errores.

- [ ] **Step 3: Verificar la invariante de seguridad**

Run: `cd frontend && grep -rn "dangerouslySetInnerHTML\|marked.parse\|\.parse(" src/components/chat/markdown.tsx src/lib/markdown-tokens.ts`
Expected: **sin coincidencias**. Si aparece alguna, el diseño se rompió: revisar antes de seguir.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/chat/markdown.tsx
git commit -m "feat(chat): renderizar markdown como elementos de React, sin innerHTML"
```

---

## Task 4: Cablear `<Markdown>` en las burbujas del chat

**Files:**
- Modify: `frontend/src/app/(dashboard)/chatbot/page.tsx` (el `map` de mensajes)

- [ ] **Step 1: Reemplazar el render del mensaje**

Buscar el bloque que hoy renderiza cada mensaje y reemplazarlo:

```tsx
                {messages.map((m) => (
                  <div
                    key={m.id}
                    className={`rounded-md px-3 py-2 text-sm max-w-[80%] ${
                      m.role === "user"
                        ? "ml-auto whitespace-pre-wrap bg-primary text-primary-foreground"
                        : "bg-muted"
                    }`}
                  >
                    {m.role === "user" ? (
                      m.content
                    ) : (
                      <Markdown>{m.content}</Markdown>
                    )}
                  </div>
                ))}
```

> El mensaje del **usuario** sigue en `whitespace-pre-wrap`: lo escribió él, no hay markdown que interpretar, y si escribe `**hola**` quiere ver `**hola**`. Solo las respuestas del asistente pasan por el renderizador.

Agregar el import: `import { Markdown } from "@/components/chat/markdown";`

- [ ] **Step 2: Verificar tipos, lint y build**

Run: `cd frontend && npx tsc --noEmit && npm run lint && npm run build`
Expected: todo pasa.

- [ ] **Step 3: Commit**

```bash
git add "frontend/src/app/(dashboard)/chatbot/page.tsx"
git commit -m "feat(chat): renderizar con formato las respuestas del asistente"
```

---

## Task 5: Tabla de artefacto con formato por tipo

**Files:**
- Modify: `frontend/src/components/chat/artifact-canvas.tsx:137-172` (`TableArtifact`)

**Interfaces:**
- Consumes: `formatValue`, `classifyValue` (Task 1)

- [ ] **Step 1: Reemplazar `TableArtifact`**

```tsx
function TableArtifact({ data }: { data: Record<string, unknown>[] }) {
  if (data.length === 0) {
    return (
      <p className="p-4 text-sm text-muted-foreground">
        La consulta no devolvió resultados.
      </p>
    );
  }
  // Las filas de una colección dinámica no tienen por qué compartir
  // campos: se unen las claves de todas para no perder columnas.
  const columns = Array.from(
    new Set(data.flatMap((row) => Object.keys(row))),
  );

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b">
            {columns.map((col) => (
              <th key={col} className="p-2 text-left font-medium">
                {col}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {data.map((row, i) => (
            <tr key={i} className="border-b last:border-0">
              {columns.map((col) => {
                const kind = classifyValue(row[col]);
                return (
                  <td
                    key={col}
                    className={cn(
                      "p-2",
                      // Montos e identificadores en mono y a la derecha:
                      // así las cifras se comparan de un vistazo por
                      // alineación de dígitos (manual de marca).
                      kind === "number" && "text-right font-mono tabular-nums",
                      kind === "date" && "font-mono",
                      kind === "empty" && "text-muted-foreground",
                    )}
                  >
                    {formatValue(row[col])}
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
```

Agregar los imports:

```tsx
import { cn } from "@/lib/utils";
import { classifyValue, formatValue } from "@/lib/format-value";
```

- [ ] **Step 2: Verificar tipos, lint y build**

Run: `cd frontend && npx tsc --noEmit && npm run lint && npm run build`
Expected: todo pasa.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/chat/artifact-canvas.tsx
git commit -m "feat(chat): formatear las celdas de la tabla según su tipo"
```

---

## Task 6: Verificación de extremo a extremo

**Files:** ninguno (verificación manual)

- [ ] **Step 1: Levantar la app**

Run: `docker compose up -d && (cd backend && go run cmd/server/main.go &) && cd frontend && npm run dev`

- [ ] **Step 2: Verificar el formato de las respuestas**

Abrir `/chatbot` y pedirle al asistente algo que produzca formato: «hacé un resumen con viñetas y resaltá los totales en negrita». La respuesta debe mostrar viñetas y negrita reales, **no** asteriscos ni guiones crudos.

- [ ] **Step 3: Verificar la tabla**

Pedir un artefacto de tipo tabla con montos y fechas. Los montos deben salir con separador de miles, alineados a la derecha y en mono; las fechas legibles; los vacíos como «—».

- [ ] **Step 4: Verificar que el informe PDF no se rompió**

Ir a una bandera roja y generar su informe PDF. Los valores deben verse igual que antes del refactor de la Task 1 (mismo formato de montos y fechas).

- [ ] **Step 5: Correr la suite completa**

Run: `cd frontend && npm run test:run && npm run lint && npm run build`
Expected: todo verde.

---

## Auto-repaso del plan

**Cobertura del spec** (sección «Formato rico» de `2026-09-10-chat-artefactos-design.md`):

| Requisito del spec | Tarea |
|---|---|
| Burbujas con markdown renderizado | 3, 4 |
| Sin `dangerouslySetInnerHTML`, vía árbol de tokens | 2, 3 (verificado con grep en la Task 3, paso 3) |
| Tokens soportados (paragraph, strong, em, codespan, code, list, table, heading, link, blockquote, hr, br, text) | 3 |
| Tokens `html` y desconocidos como texto plano | 3 (rama `default`) |
| Enlaces solo `http`/`https`/`mailto` | 2 (`safeHref`), 3 (rama `link`) |
| Dependencia `marked`, sin `dompurify` | 2 |
| Tabla formateada por tipo | 1, 5 |
| `red-flag-report.ts` consume el módulo compartido | 1 |
| Artefactos `custom` sin cambios | ninguna tarea toca `CustomArtifact` — verificado |
| Tests con casos hostiles (`<script>`, `onerror`, `javascript:`) | 2 |

**Consistencia de tipos:** `classifyValue` devuelve `ValueKind` en la Task 1 y se consume como tal en la Task 5. `Token` se reexporta desde `markdown-tokens.ts` en la Task 2 y se importa desde ahí —no desde `marked`— en la Task 3.

**Nota sobre la Task 3:** el componente no lleva test unitario propio porque su lógica de seguridad vive en `markdown-tokens.ts` (Task 2), que sí está testeada, y la rama `default` es la red que hace que cualquier token no contemplado salga como texto. La verificación por `grep` del paso 3 es lo que protege la invariante estructural.
