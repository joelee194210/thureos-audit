# Sistema de Tabs tipo Navegador Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Pestañas persistentes tipo navegador en el dashboard — abrir varios monitores/banderas rojas/dashboards a la vez, cambiar entre ellos sin perder estado ni refetch, cerrarlos individualmente.

**Architecture:** Un store Zustand persistido (`localStorage`) es la única fuente de verdad de qué tabs están abiertas. Un solo modo de tab (a diferencia de la referencia): cualquier navegación a `/monitors/[id]`, `/red-flags/[id]` o `/dashboards/[id]` se intercepta por un listener de click, la tab correspondiente se monta oculta (`hidden`, nunca se desmonta) vía un host, y nunca refetchea al volver. El contenido de esas 3 rutas se extrae de las páginas actuales (ya son Client Components con `useParams()`) a componentes que reciben `params` como prop.

**Tech Stack:** Next.js 16 App Router, Zustand (+ `zustand/middleware` persist, ya en el proyecto sin uso previo), TypeScript. Sin dependencias nuevas.

**Spec:** `docs/superpowers/specs/2026-09-07-tabs-navegador-design.md` (leer también `docs/superpowers/specs/2026-09-07-tabs-navegador-referencia-externa.md`, citada ahí como la fuente técnica exacta)

## Global Constraints

- Un solo modo de tab — nunca la distinción SSR/cliente de la referencia (este proyecto no tiene Server Components en estas rutas).
- Colores por tokens semánticos del proyecto (`bg-accent`, `text-muted-foreground`, `bg-destructive/10`) — nunca `slate-*` ni otro color literal.
- Sin breadcrumbs — fuera de alcance de este plan.
- `MAX_TABS = 10`, eviction LRU al superar el límite (nunca evict la tab recién abierta o reactivada).
- `entityLabels` se recorta (`partialize`) antes de persistir a `localStorage` — solo los ids que sigan en el path de alguna tab abierta.
- Las 3 páginas extraídas deben seguir funcionando exactamente igual como rutas reales de Next (primera carga, refresh, link directo) — la extracción es mecánica, no debe cambiar ningún comportamiento existente de esas páginas.

---

## Task 1: Store de tabs

**Files:**
- Create: `frontend/src/stores/tab-store.ts`
- Test: `frontend/src/stores/tab-store.test.ts`

**Interfaces:**
- Produces: `Tab{id, path, label}`, `MAX_TABS`, `partialize(state)`, `useTabStore` (hook Zustand con estado `{tabs, activeTabId, recency, entityLabels}` y acciones `openTab({path,label})`, `closeTab(id)`, `activateTab(id)`, `updateTabLabel(id,label)`, `registerEntityLabel(id,label)`).

- [ ] **Step 1: Escribir los tests (fallan primero)**

Crear `frontend/src/stores/tab-store.test.ts`:

```ts
import { describe, expect, it, beforeEach } from "vitest";
import { useTabStore, MAX_TABS, partialize } from "./tab-store";

function reset() {
  useTabStore.setState({ tabs: [], activeTabId: null, recency: [], entityLabels: {} });
}

describe("tab-store", () => {
  beforeEach(reset);

  it("openTab agrega una tab nueva y la activa", () => {
    useTabStore.getState().openTab({ path: "/monitors/1", label: "Monitor 1" });
    const state = useTabStore.getState();
    expect(state.tabs).toEqual([{ id: "/monitors/1", path: "/monitors/1", label: "Monitor 1" }]);
    expect(state.activeTabId).toBe("/monitors/1");
    expect(state.recency).toEqual(["/monitors/1"]);
  });

  it("openTab con un path ya abierto reactiva en vez de duplicar", () => {
    const { openTab } = useTabStore.getState();
    openTab({ path: "/monitors/1", label: "Monitor 1" });
    openTab({ path: "/monitors/2", label: "Monitor 2" });
    openTab({ path: "/monitors/1", label: "Otro label" });
    const state = useTabStore.getState();
    expect(state.tabs).toHaveLength(2);
    expect(state.tabs.find((t) => t.id === "/monitors/1")?.label).toBe("Monitor 1");
    expect(state.activeTabId).toBe("/monitors/1");
    expect(state.recency).toEqual(["/monitors/2", "/monitors/1"]);
  });

  it("openTab evict la tab LRU al superar MAX_TABS, nunca la nueva", () => {
    const { openTab } = useTabStore.getState();
    for (let i = 0; i < MAX_TABS; i++) {
      openTab({ path: `/monitors/${i}`, label: `M${i}` });
    }
    expect(useTabStore.getState().tabs).toHaveLength(MAX_TABS);
    openTab({ path: "/monitors/new", label: "Nuevo" });
    const state = useTabStore.getState();
    expect(state.tabs).toHaveLength(MAX_TABS);
    expect(state.tabs.some((t) => t.id === "/monitors/0")).toBe(false); // LRU evictada
    expect(state.tabs.some((t) => t.id === "/monitors/new")).toBe(true); // la nueva sobrevive
  });

  it("closeTab de la tab activa activa la más reciente entre las que quedan", () => {
    const { openTab, closeTab } = useTabStore.getState();
    openTab({ path: "/monitors/1", label: "M1" });
    openTab({ path: "/monitors/2", label: "M2" });
    openTab({ path: "/monitors/3", label: "M3" });
    useTabStore.getState().activateTab("/monitors/1"); // recency: 2,3,1 — activa=1
    closeTab("/monitors/1");
    expect(useTabStore.getState().activeTabId).toBe("/monitors/3");
  });

  it("closeTab de una tab inactiva no cambia activeTabId", () => {
    const { openTab, closeTab } = useTabStore.getState();
    openTab({ path: "/monitors/1", label: "M1" });
    openTab({ path: "/monitors/2", label: "M2" });
    closeTab("/monitors/1");
    expect(useTabStore.getState().activeTabId).toBe("/monitors/2");
  });

  it("closeTab de la última tab deja activeTabId en null", () => {
    const { openTab, closeTab } = useTabStore.getState();
    openTab({ path: "/monitors/1", label: "M1" });
    closeTab("/monitors/1");
    expect(useTabStore.getState().activeTabId).toBeNull();
  });

  it("activateTab sube el id al tope de recency", () => {
    const { openTab, activateTab } = useTabStore.getState();
    openTab({ path: "/monitors/1", label: "M1" });
    openTab({ path: "/monitors/2", label: "M2" });
    activateTab("/monitors/1");
    expect(useTabStore.getState().recency).toEqual(["/monitors/2", "/monitors/1"]);
  });

  it("updateTabLabel reemplaza solo el label de esa tab", () => {
    const { openTab, updateTabLabel } = useTabStore.getState();
    openTab({ path: "/monitors/1", label: "M1" });
    openTab({ path: "/monitors/2", label: "M2" });
    updateTabLabel("/monitors/1", "CBCG Tarjetas");
    const state = useTabStore.getState();
    expect(state.tabs.find((t) => t.id === "/monitors/1")?.label).toBe("CBCG Tarjetas");
    expect(state.tabs.find((t) => t.id === "/monitors/2")?.label).toBe("M2");
  });

  it("registerEntityLabel agrega/sobreescribe entityLabels", () => {
    useTabStore.getState().registerEntityLabel("abc123", "CBCG Tarjetas");
    expect(useTabStore.getState().entityLabels).toEqual({ abc123: "CBCG Tarjetas" });
    useTabStore.getState().registerEntityLabel("abc123", "Renombrado");
    expect(useTabStore.getState().entityLabels).toEqual({ abc123: "Renombrado" });
  });

  it("partialize recorta entityLabels a los ids que siguen en algún path abierto", () => {
    const state = {
      tabs: [{ id: "/monitors/abc123", path: "/monitors/abc123", label: "M" }],
      activeTabId: "/monitors/abc123",
      recency: ["/monitors/abc123"],
      entityLabels: { abc123: "CBCG Tarjetas", huerfano: "Ya no está abierto" },
    };
    const result = partialize(state as ReturnType<typeof useTabStore.getState>);
    expect(result.entityLabels).toEqual({ abc123: "CBCG Tarjetas" });
  });
});
```

- [ ] **Step 2: Correr los tests, verificar que fallan**

Run: `cd frontend && npx vitest run src/stores/tab-store.test.ts`
Expected: FAIL — `./tab-store` no existe todavía.

- [ ] **Step 3: Crear `frontend/src/stores/tab-store.ts`**

```ts
import { create } from "zustand";
import { persist, createJSONStorage } from "zustand/middleware";

export interface Tab {
  id: string;
  path: string;
  label: string;
}

export const MAX_TABS = 10;

interface TabState {
  tabs: Tab[];
  activeTabId: string | null;
  recency: string[];
  entityLabels: Record<string, string>;
  openTab: (args: { path: string; label: string }) => void;
  closeTab: (id: string) => void;
  activateTab: (id: string) => void;
  updateTabLabel: (id: string, label: string) => void;
  registerEntityLabel: (id: string, label: string) => void;
}

/** Recorta entityLabels a solo los ids que siguen apareciendo en el path
 * de alguna tab abierta — evita que nombres de entidades (potencialmente
 * sensibles en un producto AML) queden indefinidamente en localStorage
 * después de cerrar la tab que los mostraba. */
export function partialize(
  state: TabState,
): Pick<TabState, "tabs" | "activeTabId" | "recency" | "entityLabels"> {
  const idsInOpenTabs = new Set<string>();
  for (const tab of state.tabs) {
    for (const segment of tab.path.split("/").filter(Boolean)) {
      idsInOpenTabs.add(segment);
    }
  }
  const entityLabels: Record<string, string> = {};
  for (const [id, label] of Object.entries(state.entityLabels)) {
    if (idsInOpenTabs.has(id)) entityLabels[id] = label;
  }
  return {
    tabs: state.tabs,
    activeTabId: state.activeTabId,
    recency: state.recency,
    entityLabels,
  };
}

function bumpRecency(recency: string[], id: string): string[] {
  return [...recency.filter((r) => r !== id), id];
}

export const useTabStore = create<TabState>()(
  persist(
    (set, get) => ({
      tabs: [],
      activeTabId: null,
      recency: [],
      entityLabels: {},

      openTab: ({ path, label }) => {
        const existing = get().tabs.find((t) => t.path === path);
        if (existing) {
          set({
            activeTabId: existing.id,
            recency: bumpRecency(get().recency, existing.id),
          });
          return;
        }
        const tab: Tab = { id: path, path, label };
        let tabs = [...get().tabs, tab];
        let recency = bumpRecency(get().recency, tab.id);
        if (tabs.length > MAX_TABS) {
          const lruId = recency.find((id) => id !== tab.id);
          if (lruId) {
            tabs = tabs.filter((t) => t.id !== lruId);
            recency = recency.filter((id) => id !== lruId);
          }
        }
        set({ tabs, activeTabId: tab.id, recency });
      },

      closeTab: (id) => {
        const wasActive = get().activeTabId === id;
        const tabs = get().tabs.filter((t) => t.id !== id);
        const recency = get().recency.filter((r) => r !== id);
        let activeTabId = get().activeTabId;
        if (wasActive) {
          activeTabId = null;
          for (let i = recency.length - 1; i >= 0; i--) {
            if (tabs.some((t) => t.id === recency[i])) {
              activeTabId = recency[i];
              break;
            }
          }
        }
        set({ tabs, recency, activeTabId });
      },

      activateTab: (id) => {
        set({ activeTabId: id, recency: bumpRecency(get().recency, id) });
      },

      updateTabLabel: (id, label) => {
        set({
          tabs: get().tabs.map((t) => (t.id === id ? { ...t, label } : t)),
        });
      },

      registerEntityLabel: (id, label) => {
        set({ entityLabels: { ...get().entityLabels, [id]: label } });
      },
    }),
    {
      name: "thureos-tabs",
      partialize,
      storage: createJSONStorage(() => ({
        getItem: (name) => localStorage.getItem(name),
        setItem: (name, value) => {
          try {
            localStorage.setItem(name, value);
          } catch {
            // localStorage lleno o deshabilitado: se degrada a memoria
            // sin romper la app.
          }
        },
        removeItem: (name) => localStorage.removeItem(name),
      })),
    },
  ),
);
```

- [ ] **Step 4: Correr los tests, verificar que pasan**

Run: `cd frontend && npx vitest run src/stores/tab-store.test.ts`
Expected: PASS (10/10).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/stores/tab-store.ts frontend/src/stores/tab-store.test.ts
git commit -m "feat(tabs): store de tabs persistido (Zustand + localStorage)"
```

---

## Task 2: Registro de rutas y matcher

**Files:**
- Create: `frontend/src/lib/tab-registry.ts`
- Test: `frontend/src/lib/tab-registry.test.ts`

**Interfaces:**
- Produces: `RegistryEntry{pattern, Component}` (`Component: React.ComponentType<{params: Record<string,string>; tabId: string}>`), `RESERVED_DYNAMIC_SEGMENTS`, `matchRegistryEntry(path, registry) => {entry, params} | null`.

`matchRegistryEntry` toma el registro como parámetro explícito (no importa `TAB_CONTENT_REGISTRY` directo) para poder testearlo sin depender de los componentes de contenido reales, que todavía no existen en este punto del plan — la Tarea 8 arma `TAB_CONTENT_REGISTRY` una vez que las Tareas 4-6 ya crearon esos componentes.

- [ ] **Step 1: Escribir los tests (fallan primero)**

Crear `frontend/src/lib/tab-registry.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { matchRegistryEntry, type RegistryEntry } from "./tab-registry";

const FakeComponent = () => null;

const registry: RegistryEntry[] = [
  { pattern: "/monitors/[id]", Component: FakeComponent },
  { pattern: "/red-flags/[id]", Component: FakeComponent },
];

describe("matchRegistryEntry", () => {
  it("matchea una ruta registrada y extrae el param", () => {
    const result = matchRegistryEntry("/monitors/abc123", registry);
    expect(result).not.toBeNull();
    expect(result?.params).toEqual({ id: "abc123" });
    expect(result?.entry.pattern).toBe("/monitors/[id]");
  });

  it("no matchea una ruta con distinta cantidad de segmentos (subruta)", () => {
    expect(matchRegistryEntry("/monitors/abc123/edit", registry)).toBeNull();
  });

  it("no matchea una ruta no registrada", () => {
    expect(matchRegistryEntry("/rules", registry)).toBeNull();
  });

  it("no matchea un segmento reservado como si fuera un id", () => {
    expect(matchRegistryEntry("/monitors/new", registry)).toBeNull();
  });

  it("devuelve null para un registro vacío", () => {
    expect(matchRegistryEntry("/monitors/abc123", [])).toBeNull();
  });
});
```

- [ ] **Step 2: Correr los tests, verificar que fallan**

Run: `cd frontend && npx vitest run src/lib/tab-registry.test.ts`
Expected: FAIL — `./tab-registry` no existe todavía.

- [ ] **Step 3: Crear `frontend/src/lib/tab-registry.ts`**

```ts
export interface RegistryEntry {
  pattern: string;
  Component: React.ComponentType<{
    params: Record<string, string>;
    tabId: string;
  }>;
}

/** Segmentos que nunca se tratan como id dinámico, aunque el patrón de la
 * ruta sea `[algo]` — evita que una ruta estática (ej. "/monitors/new")
 * se confunda con un id real. */
export const RESERVED_DYNAMIC_SEGMENTS = new Set(["new"]);

export function matchRegistryEntry(
  path: string,
  registry: RegistryEntry[],
): { entry: RegistryEntry; params: Record<string, string> } | null {
  const pathSegments = path.split("/").filter(Boolean);

  for (const entry of registry) {
    const patternSegments = entry.pattern.split("/").filter(Boolean);
    if (patternSegments.length !== pathSegments.length) continue;

    const params: Record<string, string> = {};
    let matched = true;

    for (let i = 0; i < patternSegments.length; i++) {
      const patternSegment = patternSegments[i];
      const pathSegment = pathSegments[i];

      if (patternSegment.startsWith("[") && patternSegment.endsWith("]")) {
        if (RESERVED_DYNAMIC_SEGMENTS.has(pathSegment)) {
          matched = false;
          break;
        }
        params[patternSegment.slice(1, -1)] = pathSegment;
      } else if (patternSegment !== pathSegment) {
        matched = false;
        break;
      }
    }

    if (matched) return { entry, params };
  }

  return null;
}
```

- [ ] **Step 4: Correr los tests, verificar que pasan**

Run: `cd frontend && npx vitest run src/lib/tab-registry.test.ts`
Expected: PASS (5/5).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/tab-registry.ts frontend/src/lib/tab-registry.test.ts
git commit -m "feat(tabs): registro de rutas cero-recarga y matcher"
```

---

## Task 3: Labels de tabs

**Files:**
- Create: `frontend/src/lib/tab-labels.ts`
- Test: `frontend/src/lib/tab-labels.test.ts`

**Interfaces:**
- Produces: `SEGMENT_LABELS`, `labelForPath(path) => string`, `resolveTabLabel(tab, entityLabels) => string`.

- [ ] **Step 1: Escribir los tests (fallan primero)**

Crear `frontend/src/lib/tab-labels.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { labelForPath, resolveTabLabel } from "./tab-labels";

describe("labelForPath", () => {
  it("usa SEGMENT_LABELS para un segmento conocido", () => {
    expect(labelForPath("/monitors")).toBe("Monitores");
  });

  it("aplica title-case a un segmento desconocido", () => {
    expect(labelForPath("/algo-nuevo")).toBe("Algo Nuevo");
  });

  it("usa el último segmento de una ruta dinámica", () => {
    expect(labelForPath("/monitors/abc123")).toBe("Abc123");
  });

  it("devuelve 'Panel' para la raíz", () => {
    expect(labelForPath("/")).toBe("Panel");
  });
});

describe("resolveTabLabel", () => {
  it("devuelve el label guardado si ningún segmento está en entityLabels", () => {
    const tab = { path: "/monitors/abc123", label: "Abc123" };
    expect(resolveTabLabel(tab, {})).toBe("Abc123");
  });

  it("prioriza entityLabels sobre el label guardado", () => {
    const tab = { path: "/monitors/abc123", label: "Abc123" };
    expect(resolveTabLabel(tab, { abc123: "CBCG Tarjetas" })).toBe(
      "CBCG Tarjetas",
    );
  });

  it("con dos ids en el path, prioriza el más específico (derecha a izquierda)", () => {
    const tab = {
      path: "/monitors/proj1/rules/rule1",
      label: "Rule1",
    };
    const entityLabels = { proj1: "Proyecto Uno", rule1: "Regla Específica" };
    expect(resolveTabLabel(tab, entityLabels)).toBe("Regla Específica");
  });
});
```

- [ ] **Step 2: Correr los tests, verificar que fallan**

Run: `cd frontend && npx vitest run src/lib/tab-labels.test.ts`
Expected: FAIL — `./tab-labels` no existe todavía.

- [ ] **Step 3: Crear `frontend/src/lib/tab-labels.ts`**

```ts
export const SEGMENT_LABELS: Record<string, string> = {
  monitors: "Monitores",
  "red-flags": "Banderas Rojas",
  dashboards: "Dashboards",
  rules: "Reglas",
  screening: "Screening",
  chatbot: "Analista IA",
  settings: "Configuración",
  users: "Usuarios",
  countries: "Países",
  mcc: "MCC",
  uploads: "Cargas",
  "activity-logs": "Bitácora de Acceso",
};

function titleCase(segment: string): string {
  return segment
    .split("-")
    .map((word) => (word ? word.charAt(0).toUpperCase() + word.slice(1) : word))
    .join(" ");
}

export function labelForPath(path: string): string {
  const last = path.split("/").filter(Boolean).pop();
  if (!last) return "Panel";
  return SEGMENT_LABELS[last] || titleCase(last);
}

export function resolveTabLabel(
  tab: { path: string; label: string },
  entityLabels: Record<string, string>,
): string {
  const segments = tab.path.split("/").filter(Boolean);
  for (let i = segments.length - 1; i >= 0; i--) {
    const segment = segments[i];
    if (entityLabels[segment]) return entityLabels[segment];
  }
  return tab.label;
}
```

- [ ] **Step 4: Correr los tests, verificar que pasan**

Run: `cd frontend && npx vitest run src/lib/tab-labels.test.ts`
Expected: PASS (7/7).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/tab-labels.ts frontend/src/lib/tab-labels.test.ts
git commit -m "feat(tabs): resolución de labels de tabs"
```

---

## Task 4: Extraer el contenido de detalle de monitor

**Files:**
- Create: `frontend/src/components/tabs/content/monitor-tab-content.tsx` (contenido íntegro movido desde la página actual)
- Modify: `frontend/src/app/(dashboard)/monitors/[id]/page.tsx` (queda como wrapper fino)

**Interfaces:**
- Consumes: `useTabStore` (Tarea 1, acciones `updateTabLabel`/`registerEntityLabel`).
- Produces: `MonitorTabContent({params: {id}, tabId}) => JSX.Element` (export nombrado, no default).

Esta tarea es una extracción **mecánica** — el archivo actual tiene 750 líneas y su lógica de negocio no cambia en absoluto. Los únicos cambios reales son: de dónde sale `id` (de `useParams()` a un prop `params`), el nombre/tipo de export, y el agregado de la sincronización del label de la tab. No hace falta entender el resto del archivo para hacer esta tarea correctamente.

- [ ] **Step 1: Mover el archivo**

```bash
git mv "frontend/src/app/(dashboard)/monitors/[id]/page.tsx" frontend/src/components/tabs/content/monitor-tab-content.tsx
```

- [ ] **Step 2: Editar el archivo movido — 3 cambios puntuales**

Abrir `frontend/src/components/tabs/content/monitor-tab-content.tsx`.

**Cambio A** — quitar el import de `useParams` (línea 4 original):

Buscar:
```tsx
import { useParams } from "next/navigation";
```
Borrar esa línea.

**Cambio B** — cambiar la firma de la función y de dónde sale `id` (líneas 63-64 originales):

Buscar:
```tsx
export default function MonitorDetailPage() {
  const { id } = useParams<{ id: string }>();
```

Reemplazar por:
```tsx
export function MonitorTabContent({
  params,
  tabId,
}: {
  params: Record<string, string>;
  tabId: string;
}) {
  const { id } = params;
```

**Cambio C** — agregar la sincronización del label de la tab. Buscar la línea con `const { toastError } = useToast();` cerca del inicio del componente (línea 102 original, justo antes del primer `useEffect`) y agregar inmediatamente después:

```tsx
  const updateTabLabel = useTabStore((s) => s.updateTabLabel);
  const registerEntityLabel = useTabStore((s) => s.registerEntityLabel);

  useEffect(() => {
    if (monitor?.name) {
      updateTabLabel(tabId, monitor.name);
      registerEntityLabel(id, monitor.name);
    }
  }, [monitor?.name, tabId, id, updateTabLabel, registerEntityLabel]);
```

Y agregar el import correspondiente junto a los demás imports del archivo:
```tsx
import { useTabStore } from "@/stores/tab-store";
```

- [ ] **Step 3: Crear la página wrapper en la ubicación original**

Crear `frontend/src/app/(dashboard)/monitors/[id]/page.tsx` (archivo nuevo, la ruta real de Next sigue existiendo acá):

```tsx
"use client";

import { useParams } from "next/navigation";
import { MonitorTabContent } from "@/components/tabs/content/monitor-tab-content";

export default function MonitorDetailPage() {
  const { id } = useParams<{ id: string }>();
  return <MonitorTabContent params={{ id }} tabId={`/monitors/${id}`} />;
}
```

- [ ] **Step 4: Verificar que compila**

Run: `cd frontend && npx tsc --noEmit`
Expected: sin errores.

- [ ] **Step 5: Verificación manual**

Run: `cd frontend && npm run dev`, navegar a `/monitors/<id de un monitor real>`.
Expected: la página se ve y funciona exactamente igual que antes de esta tarea (todavía no hay tabs — eso llega en la Tarea 9; esta tarea solo debe preservar el comportamiento actual de la ruta).

- [ ] **Step 6: Commit**

```bash
git add "frontend/src/app/(dashboard)/monitors/[id]/page.tsx" frontend/src/components/tabs/content/monitor-tab-content.tsx
git commit -m "refactor(tabs): extraer detalle de monitor a MonitorTabContent"
```

---

## Task 5: Extraer el contenido de detalle de bandera roja

**Files:**
- Create: `frontend/src/components/tabs/content/red-flag-tab-content.tsx`
- Modify: `frontend/src/app/(dashboard)/red-flags/[id]/page.tsx` (queda como wrapper fino)

**Interfaces:**
- Consumes: `useTabStore` (Tarea 1).
- Produces: `RedFlagTabContent({params: {id}, tabId}) => JSX.Element` (export nombrado).

Este archivo ya tiene una función interna `CasePage()` separada del `export default` — la extracción es aún más directa que en la Tarea 4: solo hay que renombrar y mover esa función interna, no reestructurar nada.

- [ ] **Step 1: Mover el archivo**

```bash
git mv "frontend/src/app/(dashboard)/red-flags/[id]/page.tsx" frontend/src/components/tabs/content/red-flag-tab-content.tsx
```

- [ ] **Step 2: Editar el archivo movido — 4 cambios puntuales**

Abrir `frontend/src/components/tabs/content/red-flag-tab-content.tsx`.

**Cambio A** — quitar el import de `useParams`:

Buscar:
```tsx
import { useParams } from "next/navigation";
```
Borrar esa línea.

**Cambio B** — cambiar la firma de `CasePage` y renombrarla. Buscar:
```tsx
function CasePage() {
  const { id } = useParams<{ id: string }>();
```

Reemplazar por:
```tsx
export function RedFlagTabContent({
  params,
  tabId,
}: {
  params: Record<string, string>;
  tabId: string;
}) {
  const { id } = params;
```

**Cambio C** — agregar la sincronización del label de la tab. Buscar la línea `const [rf, setRf] = useState<RedFlag | null>(null);` y agregar, en algún punto después de que `rf` ya esté declarado (junto a los demás `useEffect` del componente):

```tsx
  const updateTabLabel = useTabStore((s) => s.updateTabLabel);
  const registerEntityLabel = useTabStore((s) => s.registerEntityLabel);

  useEffect(() => {
    if (rf?.ruleName) {
      updateTabLabel(tabId, rf.ruleName);
      registerEntityLabel(id, rf.ruleName);
    }
  }, [rf?.ruleName, tabId, id, updateTabLabel, registerEntityLabel]);
```

Agregar el import:
```tsx
import { useTabStore } from "@/stores/tab-store";
```

**Cambio D** — quitar el wrapper `export default` viejo, que ya no aplica (al final del archivo):

Buscar:
```tsx
export default function RedFlagCasePage() {
  return (
    <CasePage />
  );
}
```
Borrar ese bloque completo (la página wrapper real se crea en el Step 3, en la ubicación original).

- [ ] **Step 3: Crear la página wrapper en la ubicación original**

Crear `frontend/src/app/(dashboard)/red-flags/[id]/page.tsx` (archivo nuevo):

```tsx
"use client";

import { useParams } from "next/navigation";
import { RedFlagTabContent } from "@/components/tabs/content/red-flag-tab-content";

export default function RedFlagCasePage() {
  const { id } = useParams<{ id: string }>();
  return <RedFlagTabContent params={{ id }} tabId={`/red-flags/${id}`} />;
}
```

- [ ] **Step 4: Verificar que compila**

Run: `cd frontend && npx tsc --noEmit`
Expected: sin errores.

- [ ] **Step 5: Verificación manual**

Run: `cd frontend && npm run dev`, navegar a `/red-flags/<id de una bandera roja real>`.
Expected: idéntico al comportamiento previo a esta tarea.

- [ ] **Step 6: Commit**

```bash
git add "frontend/src/app/(dashboard)/red-flags/[id]/page.tsx" frontend/src/components/tabs/content/red-flag-tab-content.tsx
git commit -m "refactor(tabs): extraer detalle de bandera roja a RedFlagTabContent"
```

---

## Task 6: Extraer el contenido de detalle de dashboard

**Files:**
- Create: `frontend/src/components/tabs/content/dashboard-tab-content.tsx`
- Modify: `frontend/src/app/(dashboard)/dashboards/[id]/page.tsx` (queda como wrapper fino)

**Interfaces:**
- Consumes: `useTabStore` (Tarea 1).
- Produces: `DashboardTabContent({params: {id}, tabId}) => JSX.Element` (export nombrado).

Mismo patrón mecánico que la Tarea 4 (esta página tampoco tiene una función interna separada).

- [ ] **Step 1: Mover el archivo**

```bash
git mv "frontend/src/app/(dashboard)/dashboards/[id]/page.tsx" frontend/src/components/tabs/content/dashboard-tab-content.tsx
```

- [ ] **Step 2: Editar el archivo movido — 3 cambios puntuales**

Abrir `frontend/src/components/tabs/content/dashboard-tab-content.tsx`.

**Cambio A** — quitar el import de `useParams`:

Buscar:
```tsx
import { useParams } from "next/navigation";
```
Borrar esa línea.

**Cambio B** — cambiar la firma de la función. Buscar:
```tsx
export default function DashboardDetailPage() {
  const { id } = useParams<{ id: string }>();
```

Reemplazar por:
```tsx
export function DashboardTabContent({
  params,
  tabId,
}: {
  params: Record<string, string>;
  tabId: string;
}) {
  const { id } = params;
```

**Cambio C** — agregar la sincronización del label de la tab. Buscar la línea `const { toastError } = useToast();` cerca del inicio del componente y agregar inmediatamente después:

```tsx
  const updateTabLabel = useTabStore((s) => s.updateTabLabel);
  const registerEntityLabel = useTabStore((s) => s.registerEntityLabel);

  useEffect(() => {
    if (dashboard?.name) {
      updateTabLabel(tabId, dashboard.name);
      registerEntityLabel(id, dashboard.name);
    }
  }, [dashboard?.name, tabId, id, updateTabLabel, registerEntityLabel]);
```

Agregar el import:
```tsx
import { useTabStore } from "@/stores/tab-store";
```

- [ ] **Step 3: Crear la página wrapper en la ubicación original**

Crear `frontend/src/app/(dashboard)/dashboards/[id]/page.tsx` (archivo nuevo):

```tsx
"use client";

import { useParams } from "next/navigation";
import { DashboardTabContent } from "@/components/tabs/content/dashboard-tab-content";

export default function DashboardDetailPage() {
  const { id } = useParams<{ id: string }>();
  return <DashboardTabContent params={{ id }} tabId={`/dashboards/${id}`} />;
}
```

- [ ] **Step 4: Verificar que compila**

Run: `cd frontend && npx tsc --noEmit`
Expected: sin errores.

- [ ] **Step 5: Verificación manual**

Run: `cd frontend && npm run dev`, navegar a `/dashboards/<id de un dashboard real>`.
Expected: idéntico al comportamiento previo a esta tarea.

- [ ] **Step 6: Commit**

```bash
git add "frontend/src/app/(dashboard)/dashboards/[id]/page.tsx" frontend/src/components/tabs/content/dashboard-tab-content.tsx
git commit -m "refactor(tabs): extraer detalle de dashboard a DashboardTabContent"
```

---

## Task 7: Barra visual de tabs

**Files:**
- Create: `frontend/src/components/tabs/tab-bar.tsx`

**Interfaces:**
- Consumes: `useTabStore` (Tarea 1), `resolveTabLabel` (Tarea 3).
- Produces: `TabBar` (componente, sin props — lee todo del store).

Sin tests (componente React, sin convención de testing de componentes en este proyecto).

- [ ] **Step 1: Crear `frontend/src/components/tabs/tab-bar.tsx`**

```tsx
"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { X } from "lucide-react";
import { useTabStore } from "@/stores/tab-store";
import { resolveTabLabel } from "@/lib/tab-labels";

export function TabBar() {
  const pathname = usePathname();
  const router = useRouter();
  const tabs = useTabStore((s) => s.tabs);
  const activeTabId = useTabStore((s) => s.activeTabId);
  const entityLabels = useTabStore((s) => s.entityLabels);
  const activateTab = useTabStore((s) => s.activateTab);
  const closeTab = useTabStore((s) => s.closeTab);

  if (tabs.length === 0) return null;

  return (
    <div className="flex shrink-0 items-stretch gap-0.5 overflow-x-auto border-b bg-muted/40 px-2 pt-2">
      {tabs.map((tab) => {
        const isActive = tab.id === activeTabId;
        const label = resolveTabLabel(tab, entityLabels);
        return (
          <div
            key={tab.id}
            className={`group flex shrink-0 items-center gap-2 rounded-t-md border border-b-0 max-w-[180px] ${
              isActive
                ? "border-border bg-accent text-accent-foreground"
                : "border-transparent bg-muted/60 text-muted-foreground shadow-sm hover:bg-accent/50 hover:shadow"
            }`}
          >
            <Link
              href={tab.path}
              onClick={() => activateTab(tab.id)}
              aria-current={isActive ? "page" : undefined}
              className="min-w-0 flex-1 truncate px-3 py-1.5 text-xs font-medium"
            >
              {label}
            </Link>
            <button
              onClick={() => {
                closeTab(tab.id);
                const nextActiveId = useTabStore.getState().activeTabId;
                if (nextActiveId && nextActiveId !== pathname) {
                  router.push(nextActiveId);
                }
              }}
              aria-label={`Cerrar pestaña ${label}`}
              className={`mr-1.5 shrink-0 rounded-sm p-0.5 opacity-0 group-hover:opacity-100 ${
                isActive ? "hover:bg-accent-foreground/10" : "hover:bg-muted"
              }`}
            >
              <X className="h-3 w-3" />
            </button>
          </div>
        );
      })}
    </div>
  );
}
```

- [ ] **Step 2: Verificar que compila**

Run: `cd frontend && npx tsc --noEmit`
Expected: sin errores.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/tabs/tab-bar.tsx
git commit -m "feat(tabs): barra visual de tabs"
```

---

## Task 8: Interceptor de navegación y host de contenido

**Files:**
- Create: `frontend/src/components/tabs/use-tab-navigation.ts`
- Create: `frontend/src/components/tabs/tab-content-host.tsx`
- Modify: `frontend/src/lib/tab-registry.ts` (agrega `TAB_CONTENT_REGISTRY`, ahora que los 3 componentes de contenido ya existen)

**Interfaces:**
- Consumes: `useTabStore` (Tarea 1), `matchRegistryEntry`/`RegistryEntry` (Tarea 2), `labelForPath` (Tarea 3), `MonitorTabContent`/`RedFlagTabContent`/`DashboardTabContent` (Tareas 4-6).
- Produces: `useTabNavigation(currentPath, currentLabel, registry) => {visiblePath}`, `TabContentHost`, `TAB_CONTENT_REGISTRY`.

Sin tests (depende de `document`/DOM real — se verifica a mano en la Tarea 9, cuando ya está integrado en la app).

- [ ] **Step 1: Agregar `TAB_CONTENT_REGISTRY` a `frontend/src/lib/tab-registry.ts`**

Al final del archivo (después de `matchRegistryEntry`), agregar:

```ts
import { MonitorTabContent } from "@/components/tabs/content/monitor-tab-content";
import { RedFlagTabContent } from "@/components/tabs/content/red-flag-tab-content";
import { DashboardTabContent } from "@/components/tabs/content/dashboard-tab-content";

export const TAB_CONTENT_REGISTRY: RegistryEntry[] = [
  { pattern: "/monitors/[id]", Component: MonitorTabContent },
  { pattern: "/red-flags/[id]", Component: RedFlagTabContent },
  { pattern: "/dashboards/[id]", Component: DashboardTabContent },
];
```

- [ ] **Step 2: Crear `frontend/src/components/tabs/use-tab-navigation.ts`**

```ts
"use client";

import { useEffect, useRef, useState } from "react";
import { useTabStore } from "@/stores/tab-store";
import { labelForPath } from "@/lib/tab-labels";
import { matchRegistryEntry, type RegistryEntry } from "@/lib/tab-registry";

function isModifiedClick(e: MouseEvent): boolean {
  return e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey;
}

function findAnchor(target: EventTarget | null): HTMLAnchorElement | null {
  if (!(target instanceof Element)) return null;
  return target.closest("a");
}

/** Registra cualquier navegación real de Next como tab, e intercepta
 * clicks en links a rutas registradas para abrirlas "cero recarga" (sin
 * ir al servidor, vía history.pushState). */
export function useTabNavigation(
  currentPath: string,
  currentLabel: string,
  registry: RegistryEntry[],
) {
  const openTab = useTabStore((s) => s.openTab);
  const lastOpenedPath = useRef<string | null>(null);
  const [visiblePath, setVisiblePath] = useState<string | null>(null);

  useEffect(() => {
    if (lastOpenedPath.current === currentPath) return;
    lastOpenedPath.current = currentPath;
    openTab({ path: currentPath, label: currentLabel });
    setVisiblePath(null);
  }, [currentPath, currentLabel, openTab]);

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (isModifiedClick(e)) return;
      const anchor = findAnchor(e.target);
      if (!anchor) return;

      const url = new URL(anchor.href, window.location.origin);
      if (url.origin !== window.location.origin) return;

      const match = matchRegistryEntry(url.pathname, registry);
      if (!match) return;

      // NO stopPropagation: no debe romper onClick propios de otros <a>
      // (ej. un botón que cierra un popover en su propio onClick).
      e.preventDefault();
      openTab({ path: url.pathname, label: labelForPath(url.pathname) });
      window.history.pushState(null, "", url.pathname + url.search + url.hash);
      lastOpenedPath.current = url.pathname;
      setVisiblePath(url.pathname);
    }

    document.addEventListener("click", handleClick, { capture: true });
    return () => document.removeEventListener("click", handleClick, { capture: true });
  }, [openTab, registry]);

  return { visiblePath };
}
```

- [ ] **Step 3: Crear `frontend/src/components/tabs/tab-content-host.tsx`**

```tsx
"use client";

import { useTabStore } from "@/stores/tab-store";
import { matchRegistryEntry, type RegistryEntry } from "@/lib/tab-registry";
import { useTabNavigation } from "./use-tab-navigation";

export function TabContentHost({
  currentPath,
  currentLabel,
  registry,
  children,
}: {
  currentPath: string;
  currentLabel: string;
  registry: RegistryEntry[];
  children: React.ReactNode;
}) {
  const { visiblePath } = useTabNavigation(currentPath, currentLabel, registry);
  const tabs = useTabStore((s) => s.tabs);

  const registeredOpenTabs = tabs
    .map((tab) => {
      const match = matchRegistryEntry(tab.path, registry);
      return match ? { tab, ...match } : null;
    })
    .filter((x): x is NonNullable<typeof x> => x !== null);

  return (
    <>
      <div hidden={visiblePath !== null}>{children}</div>
      {registeredOpenTabs.map(({ tab, entry, params }) => (
        <div key={tab.path} hidden={visiblePath !== tab.path}>
          <entry.Component params={params} tabId={tab.id} />
        </div>
      ))}
    </>
  );
}
```

- [ ] **Step 4: Verificar que compila**

Run: `cd frontend && npx tsc --noEmit`
Expected: sin errores.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/tab-registry.ts frontend/src/components/tabs/use-tab-navigation.ts frontend/src/components/tabs/tab-content-host.tsx
git commit -m "feat(tabs): interceptor de navegación y host de contenido"
```

---

## Task 9: Integración en el layout del dashboard

**Files:**
- Create: `frontend/src/components/tabs/tab-system.tsx`
- Modify: `frontend/src/app/(dashboard)/layout.tsx`

**Interfaces:**
- Consumes: `TabBar` (Tarea 7), `TabContentHost`/`TAB_CONTENT_REGISTRY` (Tarea 8), `labelForPath` (Tarea 3).
- Produces: `TabSystem({children}) => JSX.Element`, montado en el layout real del dashboard.

Última tarea del plan — cierra el sistema completo.

- [ ] **Step 1: Crear `frontend/src/components/tabs/tab-system.tsx`**

```tsx
"use client";

import { usePathname } from "next/navigation";
import { TabBar } from "./tab-bar";
import { TabContentHost } from "./tab-content-host";
import { TAB_CONTENT_REGISTRY } from "@/lib/tab-registry";
import { labelForPath } from "@/lib/tab-labels";

export function TabSystem({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const label = labelForPath(pathname);

  return (
    <>
      <TabBar />
      <main className="flex-1 overflow-y-auto">
        <TabContentHost
          currentPath={pathname}
          currentLabel={label}
          registry={TAB_CONTENT_REGISTRY}
        >
          {children}
        </TabContentHost>
      </main>
    </>
  );
}
```

- [ ] **Step 2: Integrar en `frontend/src/app/(dashboard)/layout.tsx`**

Agregar el import:
```tsx
import { TabSystem } from "@/components/tabs/tab-system";
```

Reemplazar:
```tsx
  return (
    <div className="flex h-screen flex-col overflow-hidden">
      <TopNav />
      <main className="flex-1 overflow-y-auto">{children}</main>
    </div>
  );
```

por:
```tsx
  return (
    <div className="flex h-screen flex-col overflow-hidden">
      <TopNav />
      <TabSystem>{children}</TabSystem>
    </div>
  );
```

(`TabSystem` ya trae su propio `<main className="flex-1 overflow-y-auto">` — no queda duplicado.)

- [ ] **Step 3: Verificar que compila**

Run: `cd frontend && npx tsc --noEmit`
Expected: sin errores.

- [ ] **Step 4: Lint y build**

Run: `cd frontend && npm run lint`
Expected: 0 errores (warnings preexistentes sin cambios).

Run: `cd frontend && npm run build`
Expected: compila.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/tabs/tab-system.tsx "frontend/src/app/(dashboard)/layout.tsx"
git commit -m "feat(tabs): integrar el sistema de tabs en el layout del dashboard"
```

---

## Criterio de cierre del lote

- Gate (`npx tsc --noEmit`, `npm run lint`, `npm run build`, `npx vitest run`) sin bloqueantes en las 9 tareas.
- Recorrido manual:
  - Abrir 3 monitores distintos desde `/monitors` → aparecen 3 tabs en la barra.
  - Cambiar entre esas 3 tabs → el estado de cada una (scroll, formularios abiertos, datos ya cargados) se conserva, sin refetch visible.
  - El título de cada tab pasa de un id/CUID a el nombre real del monitor apenas carga.
  - Cerrar la tab activa → navega a la tab más reciente entre las que quedan.
  - Cerrar una tab inactiva → no navega, la URL actual no cambia.
  - Abrir 11 tabs (superando `MAX_TABS`) → se evict la menos usada recientemente, nunca la nueva.
  - Recargar la página (F5) → las tabs siguen ahí, en el mismo orden, con los mismos labels.
  - Abrir un monitor, cerrarlo, inspeccionar `localStorage["thureos-tabs"]` → el nombre del monitor ya no aparece en `entityLabels`.
  - Mezclar tabs registradas (monitor/bandera roja/dashboard) con tabs no registradas (ej. `/rules`, `/settings`) → las no registradas navegan normal y también aparecen como tab, sin el mecanismo de cero-recarga.
