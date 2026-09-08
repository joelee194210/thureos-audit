# Sistema de Tabs tipo Navegador — Diseño

**Fecha:** 2026-09-07
**Estado:** Aprobado para pasar a plan de implementación (diseño validado contra una especificación de referencia del propio usuario, con las dos decisiones de adaptación confirmadas explícitamente en el chat)
**Referencia técnica exacta:** `docs/superpowers/specs/2026-09-07-tabs-navegador-referencia-externa.md` (código real citado del proyecto "Thureos Admin" — el plan de implementación debe leer este archivo para el código exacto del store, el matcher de rutas y `resolveTabLabel`, no solo este documento de diseño)

## Objetivo

Un sistema de pestañas persistentes tipo navegador/VS Code para el dashboard de Thureos Compliance (`(dashboard)/*`). Permite tener abiertas varias pantallas de detalle (un monitor, una bandera roja, un dashboard) a la vez, cambiar entre ellas sin perder su estado ni volver a hacer fetch, y cerrarlas individualmente. Basado en una especificación de referencia que documenta un sistema equivalente ya implementado en otro proyecto del usuario ("Thureos Admin") — este diseño reutiliza su arquitectura donde aplica y la adapta donde la base técnica de este proyecto difiere.

## Diferencia estructural clave respecto a la referencia

La referencia depende de que las páginas normales sean **Server Components** (Next.js hace fetch en el servidor en cada navegación) y de que el modo "cero recarga" use **TanStack Query** del lado del cliente como excepción. En este proyecto **las tres páginas candidatas ya son Client Components** (`"use client"`, fetch propio vía `useEffect`/`useState` contra `lib/api/*.ts`) — no hay Server Components que evitar. `@tanstack/react-query` está instalado y su `QueryClientProvider` ya existe en `src/components/providers.tsx`, pero ningún componente lo usa hoy (`useState`/`useEffect` es el patrón real en todo el dashboard).

Consecuencia: **un solo modo de tab, no dos.** Cualquier navegación a una ruta registrada se intercepta y se muestra vía el host de tabs (montada, oculta con `hidden`, nunca se desmonta mientras la tab siga abierta — así conserva su `useState` local y no repite el fetch). Las rutas no registradas (listados como `/monitors`, `/rules`, `/red-flags`) navegan normal con Next, sin ningún cambio — se abren como tab igual, pero su contenido es simplemente lo que Next ya renderiza.

## Fuera de alcance (explícito)

- **Breadcrumbs.** La referencia los incluye porque ya existían en ese proyecto; acá no existen y no se pidieron — agregarlos es un cambio visual aparte, no una dependencia funcional del sistema de tabs. Se puede sumar después si hace falta.
- **Atajos de teclado, drag-and-drop de tabs, menú de overflow.** La referencia tampoco los tiene (confirmado por su propia auditoría de código) — no se inventan acá.
- **Server Actions / SSR real para las 3 rutas registradas.** No aplica a este proyecto — ver la diferencia estructural arriba.

## Arquitectura

```
Usuario hace click en un link
        │
        ▼
¿La ruta matchea un patrón en TAB_CONTENT_REGISTRY?
        │
   No ──┴── Sí
   │         │
   ▼         ▼
Next.js    preventDefault(); openTab({path, label})
navega     history.pushState (sin ir al servidor)
normal     visiblePath = la nueva ruta
   │         │
   ▼         ▼
usePathname cambia    TabContentHost muestra/mantiene montado
   │                  el <XTabContent params isActive tabId />
   ▼                  registrado para esa ruta — nunca se
openTab({path, label: labelForPath(path)})   desmonta mientras la tab exista
   │
   ▼
tab-store: agrega o reactiva la tab, sube en `recency`,
evict LRU si > MAX_TABS
   │
   ▼
TabBar re-renderiza, persist middleware de Zustand
escribe a localStorage
```

## Modelo de datos (store)

`frontend/src/stores/tab-store.ts` — Zustand + `persist` a `localStorage` (mismo esquema que la referencia, mismos nombres):

```ts
interface Tab {
  id: string;   // = path
  path: string;
  label: string;
}

const MAX_TABS = 10;

interface TabState {
  tabs: Tab[];
  activeTabId: string | null;
  recency: string[];                     // LRU, más reciente al final
  entityLabels: Record<string, string>;  // id de entidad → nombre legible
  openTab, closeTab, activateTab, updateTabLabel, registerEntityLabel: (...) => void;
}
```

Comportamiento (idéntico a la referencia — ver sección 6 de su documento para el detalle exacto de cada acción, no se repite acá porque el usuario ya lo escribió completo):
- `openTab` reactiva si el `path` ya existe (nunca duplica); si no, agrega, activa, sube `recency`, evict LRU si supera `MAX_TABS`.
- `closeTab` quita la tab; si era la activa, la nueva activa es la más reciente entre las que quedan (o `null` si no queda ninguna).
- `entityLabels` es un caché plano por id de entidad (no por tab), poblado por los tab-content components apenas conocen el nombre real (ej. el nombre de un monitor tras su fetch).
- `partialize` recorta `entityLabels` a solo los ids que siguen en el path de alguna tab abierta antes de persistir a `localStorage` — mismo motivo que la referencia: evitar que nombres de entidades (potencialmente sensibles en un producto AML — nombre de cliente, descripción de una bandera roja) queden indefinidamente en el navegador tras cerrar la tab que los mostraba.

## Componentes

- **`frontend/src/lib/tab-registry.ts`** — `TAB_CONTENT_REGISTRY: {pattern, Component}[]` con las 3 rutas registradas (`/monitors/[id]`, `/red-flags/[id]`, `/dashboards/[id]`) + `matchRegistryEntry(path)` (matching segmento a segmento, misma cantidad de segmentos, sin match parcial de subrutas — idéntico a la referencia sección 5).
- **`frontend/src/lib/tab-labels.ts`** — `labelForPath(path)` (mapa `SEGMENT_LABELS` propio de este proyecto + fallback a title-case) y `resolveTabLabel(tab, entityLabels)` (escanea segmentos de derecha a izquierda buscando un id conocido en `entityLabels`, fallback al label guardado — idéntico a la referencia sección 7).
- **`frontend/src/components/tabs/tab-bar.tsx`** — la barra visual. Misma estructura que la referencia (fila `overflow-x-auto`, chip truncado + botón cerrar que aparece en hover), **pero con los tokens semánticos de este proyecto** en vez de los `slate-*` literales de la referencia: tab activa `bg-accent text-accent-foreground`, tab inactiva `text-muted-foreground hover:bg-accent/50` (mismo patrón que ya usa la selección de conversación del chatbot en `chatbot/page.tsx`), botón cerrar con `hover:bg-destructive/10 hover:text-destructive` al aparecer en hover del grupo.
- **`frontend/src/components/tabs/tab-content-host.tsx`** — monta TODAS las tabs abiertas que matchean el registro, ocultas con el atributo HTML `hidden` (no `display:none` vía clase — para no desmontar React y perder estado), mostrando solo la que corresponde a `visiblePath`.
- **`frontend/src/components/tabs/use-tab-navigation.ts`** — hook con el listener de click en fase de captura sobre `document` (intercepta `<a>` cuyo `href` matchea el registro, `preventDefault` + `history.pushState`, sin `stopPropagation`) + el `useEffect` que registra cualquier navegación real como tab.
- **`frontend/src/components/tabs/tab-system.tsx`** — orquestador: monta `TabBar` + `TabContentHost` (sin breadcrumbs, ver Fuera de alcance), se monta en `frontend/src/app/(dashboard)/layout.tsx` envolviendo `{children}`, junto al `TopNav` ya existente.
- **`frontend/src/components/tabs/content/monitor-tab-content.tsx`, `red-flag-tab-content.tsx`, `dashboard-tab-content.tsx`** (nuevo, no existe en la referencia porque ahí no hacía falta): extraen el JSX + lógica de fetch de las 3 páginas de detalle actuales (`monitors/[id]/page.tsx`, `red-flags/[id]/page.tsx`, `dashboards/[id]/page.tsx`) a componentes que reciben `params: {id: string}` como prop (no `useParams()` de `next/navigation`, que no aplica cuando el componente no es literalmente la página enrutada) y `isActive`/`tabId` como en la referencia. Las páginas originales quedan como wrappers finos: siguen siendo la ruta real de Next (para que la navegación SSR/primera carga funcione), pero delegan su contenido a estos componentes vía el mismo mecanismo del host.

## Integración con el layout existente

`frontend/src/app/(dashboard)/layout.tsx` (ya existe, cliente, envuelve `TopNav` + `<main>{children}</main>`): se inserta `TabSystem` entre `TopNav` y el área de contenido, exactamente como en la referencia sección 3 pero sin la fila de breadcrumbs:

```tsx
<div className="flex h-screen flex-col overflow-hidden">
  <TopNav />
  <TabBar />
  <main className="flex-1 overflow-y-auto">
    <TabContentHost currentPath={pathname} currentLabel={label}>
      {children}
    </TabContentHost>
  </main>
</div>
```

## Persistencia

Igual que la referencia (sección 9): `persist` de Zustand sobre `localStorage`, key `"thureos-tabs"` (namespace propio de este proyecto, no `"thureos-admin-tabs"`). Mismo manejo de error si `localStorage` falla (degrada a memoria sin romper la app). Mismo `partialize` recortando `entityLabels`.

## Manejo de errores

- `localStorage.setItem` fallando (cuota llena/deshabilitado) → capturado, el store sigue funcionando en memoria para esa sesión de página, sin romper la app (idéntico a la referencia).
- Un tab-content cuyo fetch falla usa el mismo patrón de error que su página original ya tiene hoy (`toastError` vía `useToast`) — no se introduce un mecanismo de error nuevo.
- Segmentos reservados (ninguno identificado hoy en las 3 rutas registradas, pero se deja el mecanismo — `RESERVED_DYNAMIC_SEGMENTS` en `tab-registry.ts`, igual que la referencia) para rutas estáticas que no deben tratarse como id dinámico si alguna llegara a coexistir (ej. `/monitors/new` si existiera).

## Testing

- **TDD completo** para las piezas puras: `matchRegistryEntry` (matching de rutas, casos con/sin match, subrutas que no matchean), `resolveTabLabel`/`labelForPath` (fallback a título, prioridad de `entityLabels`, escaneo derecha-a-izquierda), y las acciones puras del store (`openTab` no duplica, `closeTab` recalcula la activa correctamente, eviction LRU al superar `MAX_TABS`) — mismo criterio que la referencia, que testea exactamente estas piezas.
- **Sin tests** para los componentes de React en sí (`tab-bar.tsx`, `tab-content-host.tsx`, los 3 `*-tab-content.tsx`) — no hay convención de testing de componentes en este proyecto (el único test frontend existente hoy es sobre `src/lib/`, ver `vitest.config.ts`).
- **Verificación manual** (checklist de cierre del plan): abrir 3 monitores distintos como tabs, cambiar entre ellos sin perder el estado de scroll/formularios abiertos, cerrar la tab activa (navega a la más reciente entre las que quedan), cerrar una tab inactiva (no navega), superar `MAX_TABS` y confirmar que se evict la LRU, recargar la página y confirmar que las tabs persisten, abrir una tab con datos sensibles (ej. nombre de cliente) y confirmar que tras cerrarla el nombre no queda en `localStorage`.
