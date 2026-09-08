# Especificación de referencia: Sistema de Tabs tipo navegador (Thureos Admin)

> Este documento es la especificación que el usuario proveyó, generada leyendo el
> código real de OTRO proyecto suyo ("Thureos Admin", rama `main`, commit `ca33f9a`).
> Se guarda acá tal cual, sin editar, como referencia técnica exacta para la
> implementación de `docs/superpowers/specs/2026-09-07-tabs-navegador-design.md`
> (el diseño adaptado a ESTE proyecto). Cuando el diseño adaptado difiere de este
> documento, el diseño adaptado es la autoridad — ver ahí la sección "Diferencia
> estructural clave respecto a la referencia".

> Documento generado leyendo el código real del repo (rama `main`, commit `ca33f9a`). Cada afirmación está respaldada por un archivo citado. Cuando el código no confirma un detalle, se marca explícitamente como **"no confirmado por código, verificar manualmente"**.

## 1. Qué es

Un sistema de pestañas persistentes tipo navegador/VS Code para el panel admin (`(admin)/*`) de una app Next.js (App Router). Permite tener abiertas varias pantallas (cliente, persona, solicitud, etc.) a la vez, cambiar entre ellas sin perder su estado, y cerrarlas individualmente. Convive con una barra de breadcrumbs que se actualiza según la pestaña activa.

No existe en las rutas públicas (`(public)/f/*`) ni en el portal de cliente (`(client)/dashboard/*`) — solo en `(admin)/*`.

## 2. Arquitectura general — piezas y responsabilidad

| Archivo | Responsabilidad |
|---|---|
| `src/lib/tab-store.ts` | Store global (Zustand + persist a `localStorage`). Única fuente de verdad: lista de tabs abiertas, tab activa, orden de uso (recency, para LRU), y caché `entityLabels` (id → nombre legible). |
| `src/lib/tab-registry.tsx` | Registro estático de qué **patrones de ruta dinámica** ("`/admin/clients/[clientId]`") tienen un componente de contenido "sin recarga" asociado, y función `matchRegistryEntry(path)` que hace el matching de rutas contra esos patrones. |
| `src/lib/tab-labels.ts` | Deriva el label textual de una tab a partir de su ruta (`labelForPath`) y resuelve qué label mostrar realmente combinando el label guardado con `entityLabels` (`resolveTabLabel`). |
| `src/lib/tab-query-errors.ts` | Helper `isPermissionError(error)` — detecta errores de autorización (regex sobre el mensaje: `/autorizado|permiso/i`) para mostrar `<AccessDenied />` en vez de un error genérico dentro del contenido de una tab. |
| `src/components/tabs/tab-bar.tsx` | La UI de la barra de pestañas (fila horizontal con título + botón cerrar por tab). Se suscribe al store. |
| `src/components/tabs/tab-system.tsx` | Orquestador de layout: monta `TabBar` + `AppBreadcrumbs` + `TabContentHost`, en ese orden vertical. Es lo que se monta en el layout admin. |
| `src/components/tabs/tab-content-host.tsx` | El "host" que decide qué contenido mostrar: para rutas registradas en `tab-registry`, monta TODAS las tabs abiertas que matchean el registro simultáneamente (ocultas con `hidden`) para que no pierdan estado de React/React Query al cambiar de tab; para rutas no registradas, muestra el `children` que vino del Server Component de Next normal. |
| `src/components/tabs/use-tab-navigation.ts` | Hook que (a) abre/activa una tab cuando cambia la ruta real de Next.js (navegación SSR normal), y (b) intercepta clicks en links a rutas registradas para hacer una navegación "cero recarga" (sin ir al servidor), actualizando la URL vía `history.pushState`. |
| `src/components/tabs/use-has-been-visible.ts` | Hook "latch": una vez que una tab estuvo activa una vez, sigue reportando `true` para siempre (aunque se cambie de tab), usado para gatear (`enabled`) las queries de React Query de cada tab-content y no perder su caché al ocultarla. |
| `src/components/register-entity-label.tsx` | Componente sin render (`RegisterEntityLabel`) usado en Server Components para registrar el nombre legible de una entidad en el store apenas se conoce (ej. página de detalle SSR que ya hizo el fetch). |
| `src/components/app-breadcrumbs.tsx` | Barra de breadcrumbs. Comparte el mismo `entityLabels` del tab-store para resolver nombres de segmentos dinámicos de la URL. Define también `SEGMENT_LABELS`, el mapa estático segmento→label en español usado también por `tab-labels.ts`. |
| `src/components/tabs/content/*.tsx` | Componentes "tab content" reales: `PersonTabContent`, `ClientTabContent`, `SubmissionTabContent`. Hacen fetch vía React Query + Server Actions, y al obtener el nombre real de la entidad llaman `updateTabLabel` + `registerEntityLabel`. |
| `src/components/tabs/query-provider.tsx` | `QueryClientProvider` de TanStack Query para toda el área admin (requerido porque el contenido de tabs usa `useQuery`). |

## 3. Cómo se monta en la app

`src/app/(admin)/layout.tsx`:

```tsx
<SessionProvider>
  <QueryProvider>
    <div className="flex h-screen flex-col overflow-hidden bg-background">
      <TopNav ... />
      {/* Barra de pestañas + breadcrumb de la pestaña activa + área de
          trabajo (TabSystem posee su propio <main>). */}
      <TabSystem>{children}</TabSystem>
      <footer>...</footer>
    </div>
  </QueryProvider>
</SessionProvider>
```

Y `TabSystem` (`src/components/tabs/tab-system.tsx`):

```tsx
export function TabSystem({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const label = labelForPath(pathname);

  return (
    <>
      <TabBar />
      <div className="shrink-0 border-b bg-card/50 px-4 py-2 md:px-6 lg:px-8">
        <AppBreadcrumbs />
      </div>
      <main className="flex-1 overflow-y-auto min-h-0">
        <TabContentHost currentPath={pathname} currentLabel={label}>
          <div className="w-full px-4 py-7 md:px-6 lg:px-8">
            {children}
          </div>
        </TabContentHost>
      </main>
    </>
  );
}
```

Orden vertical fijo: **barra de tabs → breadcrumb de la tab activa → contenido**. El comentario en el código dice explícitamente que replica el orden de Chrome/VS Code.

## 4. Dos modos de apertura de tab: "SSR normal" vs "cero recarga"

Este es el punto más importante y menos obvio del sistema. Hay dos maneras de que exista una tab:

### 4.1. Navegación SSR normal (siempre pasa, para cualquier ruta admin)

Cada vez que `usePathname()` cambia (cualquier navegación real de Next.js, incluida la primera carga), `useTabNavigation` la registra como tab:

```ts
// src/components/tabs/use-tab-navigation.ts
useEffect(() => {
  if (lastOpenedPath.current === currentPath) return;
  lastOpenedPath.current = currentPath;
  openTab({ path: currentPath, label: currentLabel });
  setVisiblePath(null);
}, [currentPath, currentLabel, openTab]);
```

`currentLabel` viene de `labelForPath(pathname)` calculado en `TabSystem`. En este modo, el contenido que se muestra es simplemente el `children` (la página Server Component real de Next), sin ningún truco — Next hace su render/fetch normal en el servidor. No hay "cero recarga" aquí: si el usuario clickea un link normal en la sidebar/topnav a una ruta NO registrada en `tab-registry`, ocurre una navegación Next.js normal (con su propio round-trip), y esa ruta se agrega/activa como tab.

### 4.2. Navegación interceptada "cero recarga" (solo para rutas en `TAB_CONTENT_REGISTRY`)

Para las 3 rutas registradas hoy (`/admin/persons/[personId]`, `/admin/clients/[clientId]`, `/admin/submissions/[submissionId]`), un listener de click en fase de **captura**, montado sobre `document`, intercepta cualquier click en un `<a>` cuyo `href` matchea uno de esos patrones:

```ts
// src/components/tabs/use-tab-navigation.ts
document.addEventListener("click", handleClick, { capture: true });

function handleClick(e: MouseEvent) {
  if (isModifiedClick(e)) return;              // click derecho/medio, cmd/ctrl/shift/alt → no interceptar
  const anchor = findAnchor(e.target);
  if (!anchor) return;
  const url = new URL(anchor.href, window.location.origin);
  if (url.origin !== window.location.origin) return;   // solo mismo origen
  const match = matchRegistryEntry(url.pathname);
  if (!match) return;                            // ruta no registrada → dejar pasar la navegación normal

  e.preventDefault();                            // NO stopPropagation (a propósito, ver comentario abajo)
  openTab({ path: url.pathname, label: labelForPath(url.pathname) });
  window.history.pushState(null, "", url.pathname + url.search + url.hash);
  lastOpenedPath.current = url.pathname;
  setVisiblePath(url.pathname);
}
```

Efecto: la URL del navegador cambia (vía `history.pushState`, no vía fetch), la tab se abre/activa en el store, y el componente de contenido registrado para esa ruta (`PersonTabContent`, etc.) se muestra sin que Next haga ningún round-trip al servidor — el componente ya estaba montado (oculto) o se monta ahora y hace su propio fetch client-side con React Query.

Detalles de esta intercepción, confirmados por tests (`use-tab-navigation.test.tsx`):
- Solo click primario sin modificadoras (`button === 0`, sin `metaKey/ctrlKey/shiftKey/altKey`) — así "abrir en pestaña nueva del navegador" (cmd/ctrl+click) sigue funcionando normal.
- Preserva `?query` y `#hash` en la URL pusheada, pero `visiblePath`/la tab solo usan el `pathname`.
- Usa `preventDefault()` pero deliberadamente **no** `stopPropagation()`, para no romper `onClick` propios que algunos `<a>` puedan tener (ejemplo citado en el código: `notification-bell.tsx` cierra un popover en su propio `onClick`).
- Un segmento reservado como `"new"` (ej. `/admin/persons/new`) nunca matchea el patrón `[personId]` — hay una lista `RESERVED_DYNAMIC_SEGMENTS` en `tab-registry.tsx` para evitarlo, porque si no, la ruta estática de "crear" se confundiría con un id real.

### 4.3. Cómo conviven ambos modos: `TabContentHost`

```tsx
// src/components/tabs/tab-content-host.tsx
export function TabContentHost({ currentPath, currentLabel, children }) {
  const { visiblePath } = useTabNavigation(currentPath, currentLabel);
  const tabs = useTabStore((s) => s.tabs);

  const registeredOpenTabs = tabs
    .map((tab) => {
      const match = matchRegistryEntry(tab.path);
      return match ? { tab, ...match } : null;
    })
    .filter(Boolean);

  return (
    <>
      <div hidden={visiblePath !== null}>{children}</div>
      {registeredOpenTabs.map(({ tab, entry, params }) => (
        <div key={tab.path} hidden={visiblePath !== tab.path}>
          <entry.Component params={params} isActive={visiblePath === tab.path} tabId={tab.id} />
        </div>
      ))}
    </>
  );
}
```

Regla clave: **TODAS** las tabs abiertas que matchean el registro se montan simultáneamente en el DOM (ocultas con el atributo HTML `hidden`, no `display:none` vía clase), de modo que su estado de React/React Query sobrevive al cambiar entre ellas. `visiblePath` (estado interno del hook, no persistido) decide cuál de esos divs está visible: `null` = se muestra el `children` normal de Next (SSR); un pathname concreto = se muestra ese tab-content client-side.

Cada `entry.Component` recibe:
- `params`: los parámetros extraídos de la URL (ej. `{ clientId: "..." }`).
- `isActive`: `true` solo si es la tab visible en este momento.
- `tabId`: el id de la tab en el store (igual al `path`), usado para poder renombrar su label después.

## 5. `tab-registry.tsx` — matching de rutas

```ts
export const TAB_CONTENT_REGISTRY: RegistryEntry[] = [
  { pattern: "/admin/persons/[personId]", Component: PersonTabContent },
  { pattern: "/admin/clients/[clientId]", Component: ClientTabContent },
  { pattern: "/admin/submissions/[submissionId]", Component: SubmissionTabContent },
];
```

`matchRegistryEntry(path)`:
- Compara segmento a segmento; requiere **igual cantidad de segmentos** que el patrón (no hay match parcial ni de subrutas: `/admin/persons/p1/edit` NO matchea `/admin/persons/[personId]`, confirmado por test).
- Un segmento de patrón `[algo]` matchea cualquier segmento salvo los reservados (`"new"`).
- Devuelve `{ entry, params }` o `null`.

Para reimplementar en otro proyecto: agregar una entrada por cada ruta dinámica que se quiera "cero recarga"; cualquier otra ruta simplemente usa el modo SSR normal (sección 4.1) sin cambios adicionales.

## 6. El store (`src/lib/tab-store.ts`) — estado y acciones

```ts
export interface Tab {
  id: string;      // = path (ver abajo)
  path: string;
  label: string;
  icon?: string;    // definido en el tipo pero NUNCA usado/rendereado en ningún lado del código actual — campo muerto
}

export const MAX_TABS = 10;

interface TabState {
  tabs: Tab[];
  activeTabId: string | null;
  recency: string[];             // orden de uso más reciente al final, para LRU
  entityLabels: Record<string, string>;  // id → nombre legible
  openTab, closeTab, activateTab, updateTabLabel, registerEntityLabel
}
```

Notas:
- **El `id` de una tab es literalmente su `path`** (`{ id: path, path, label, icon }` en `openTab`). No hay UUIDs generados — dos tabs nunca pueden compartir path porque `openTab` reutiliza la existente.
- `MAX_TABS = 10`. Al abrir una tab nueva que superaría el límite, se **evicta la tab usada menos recientemente (LRU)**, nunca la recién abierta ni la reactivada. Implementado buscando en `recency` el primer id que no sea el nuevo path.
- `entityLabels` es un caché plano compartido entre el tab-bar y los breadcrumbs (sección 8).

### 6.1. `openTab({ path, label, icon })`

- Si ya existe una tab con ese `path`: solo la activa y la sube al tope de `recency` (no crea duplicado, no cambia su label).
- Si no existe: la agrega al final de `tabs`, la activa, la sube en `recency`, y si se pasó de `MAX_TABS` evicta la LRU.

### 6.2. `closeTab(id)`

- Quita la tab de `tabs` y de `recency`.
- Si la tab cerrada **era la activa**, la nueva activa pasa a ser la más reciente entre las que quedan (recorriendo `recency` en reversa buscando la primera que siga en `tabs`). Si no queda ninguna, `activeTabId = null`.
- Si la tab cerrada **no era la activa**, `activeTabId` no cambia.

### 6.3. `activateTab(id)` / `updateTabLabel(id, label)` / `registerEntityLabel(id, label)`

- `activateTab`: cambia `activeTabId` y sube el id al tope de `recency`.
- `updateTabLabel`: reemplaza el `label` de una tab puntual (usado por los tab-content components cuando obtienen el nombre real, sección 7).
- `registerEntityLabel`: agrega/sobreescribe `entityLabels[id] = label`. Es un caché global por id de entidad, no por tab — lo puede llamar cualquier página, tenga o no una tab de tipo "cero recarga" asociada.

## 7. Cómo se determina el título de cada tab

Dos fuentes, resueltas en cascada por `resolveTabLabel` (`src/lib/tab-labels.ts`), que es lo que efectivamente pinta `tab-bar.tsx`:

```ts
export function resolveTabLabel(
  tab: { path: string; label: string },
  entityLabels: Record<string, string>,
): string {
  const segments = tab.path.split("/").filter(Boolean);
  for (let i = segments.length - 1; i >= 0; i--) {   // de derecha a izquierda
    const segment = segments[i];
    if (entityLabels[segment]) return entityLabels[segment];
  }
  return tab.label;
}
```

1. **Label estático inicial** (`tab.label`): calculado al abrir la tab por `labelForPath(path)`:
   ```ts
   export function labelForPath(path: string): string {
     const last = path.split("/").filter(Boolean).pop();
     if (!last) return "Panel";
     return SEGMENT_LABELS[last] || titleCase(last);
   }
   ```
   Usa el **último segmento** de la ruta contra el mapa `SEGMENT_LABELS` (`src/components/app-breadcrumbs.tsx`, ej. `clients: "Clientes"`, `persons: "Clientes Aprobados"`, `sar: "SAR / ROS"`...). Si no está en el mapa, aplica `titleCase` reemplazando guiones por espacios y capitalizando cada palabra (`"algo-nuevo"` → `"Algo Nuevo"`). Si el segmento es un CUID (sin guiones), queda tal cual capitalizado (ej. `"Cmtqhcdq70000bp2voyiof5jw"`) — esto es intencionalmente temporal, hasta que se resuelva el nombre real.

2. **Label dinámico vía `entityLabels`**: cuando una tab-content obtiene el nombre real de la entidad (ej. el nombre del cliente tras el fetch), llama tanto `updateTabLabel(tabId, nombre)` (cambia el label guardado de ESA tab puntual) como `registerEntityLabel(id, nombre)` (cachea el nombre por **id de entidad**, reusable por cualquier otra tab/breadcrumb que referencie ese mismo id). Ejemplo real, `client-tab-content.tsx`:
   ```tsx
   const clientName = clientQuery.data?.name;
   useEffect(() => {
     if (clientName) {
       updateTabLabel(tabId, clientName);
       registerEntityLabel(params.clientId, clientName);
     }
   }, [clientName, tabId, params.clientId, updateTabLabel, registerEntityLabel]);
   ```
   `resolveTabLabel` prioriza `entityLabels` sobre el `label` guardado, escaneando los segmentos del path **de derecha a izquierda** — así, en una ruta con dos ids registrados (ej. `/admin/monitoring/projects/{projectId}/rules/{ruleId}`), se muestra el nombre del segmento **más específico** (la regla), no el del contenedor (el proyecto). Confirmado por test en `tab-labels.test.ts`.

3. Para páginas Server-rendered normales (fuera de `TAB_CONTENT_REGISTRY`) que igual quieren mostrar un nombre real en vez de un CUID, existe `<RegisterEntityLabel id={id} label={nombreYaConocido} />` (`src/components/register-entity-label.tsx`) — un componente cliente sin render que solo llama `registerEntityLabel` en un `useEffect`. Se usa en ~13 páginas de detalle (investigaciones, formularios, workflows, beneficiarios finales, matriz de riesgo, expedientes, SAR, monitoreo, ayuda, screening, dashboard de cliente), siempre pasando un label que la página SSR ya obtuvo en su propio fetch (no dispara fetch propio). Ejemplo real:
   ```tsx
   // src/app/(admin)/admin/investigations/[investigationId]/page.tsx
   <RegisterEntityLabel id={params.investigationId} label={result.data.query} />
   <InvestigationDetail investigation={result.data} />
   ```

## 8. Integración con breadcrumbs

`AppBreadcrumbs` (`src/components/app-breadcrumbs.tsx`) construye el breadcrumb de la ruta actual segmento por segmento:
- El primer segmento (`admin`/`dashboard`) se colapsa en un ícono de casa ("Inicio"), no se muestra como texto.
- Segmentos estáticos usan `SEGMENT_LABELS[segment] || segment`.
- Segmentos que matchean un patrón de ID (`isDynamicSegment`: UUID v4, CUID `^c[a-z0-9]{20,}$`, ObjectId Mongo de 24 hex, o numérico puro) buscan su nombre en el **mismo** `entityLabels` del tab-store:
  ```ts
  label = entityLabels[segment] || truncateId(segment);
  ```
  `truncateId` recorta a 8 caracteres + "…" si el id es más largo de 12.
- Esto significa que **`tab-bar.tsx` y `app-breadcrumbs.tsx` leen exactamente el mismo diccionario** (`useTabStore((s) => s.entityLabels)`) — cualquier `registerEntityLabel`/`updateTabLabel` que dispare un tab-content también actualiza el breadcrumb, y viceversa (cualquier `RegisterEntityLabel` en una página SSR normal también renombra la tab si esa tab existe). No hay una segunda fuente de verdad para nombres de entidad.
- Un id dinámico puede aparecer en cualquier posición del path (no solo el último segmento) — soporta rutas como `/admin/forms/{id}/designer`.

`TabSystem` monta `AppBreadcrumbs` inmediatamente debajo de `TabBar`, y el comentario en el código aclara la intención: el breadcrumb describe la pestaña activa, no toda la app.

## 9. Persistencia del estado

`tab-store.ts` usa el middleware `persist` de Zustand con `createJSONStorage` sobre `localStorage`, bajo la key `"thureos-admin-tabs"`:

```ts
export const useTabStore = create<TabState>()(
  persist(
    (set, get) => ({ /* ... */ }),
    {
      name: "thureos-admin-tabs",
      partialize,
      storage: createJSONStorage(() => ({
        getItem: (name) => localStorage.getItem(name),
        setItem: (name, value) => {
          try { localStorage.setItem(name, value); }
          catch { /* localStorage lleno o deshabilitado: se degrada a memoria sin romper la app */ }
        },
        removeItem: (name) => localStorage.removeItem(name),
      })),
    }
  )
);
```

Implicaciones:
- **Es `localStorage`, no `sessionStorage` ni cookie**: sobrevive a un refresh Y a cerrar/reabrir el navegador. Es por **navegador/perfil**, no por sesión de servidor — dos usuarios que comparten el mismo perfil de navegador comparten las tabs abiertas (ver la nota de seguridad abajo). **No confirmado por código**: si hay lógica en otro lado que limpie el store al hacer logout — no se encontró ninguna en los archivos revisados; verificar manualmente si esto es un problema en la práctica.
- `partialize` decide **qué se escribe a disco** (recorta `entityLabels` a solo los ids que siguen apareciendo en el path de alguna tab abierta):
  ```ts
  export function partialize(state: TabState): Pick<TabState, "tabs" | "activeTabId" | "recency" | "entityLabels"> {
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
    return { tabs: state.tabs, activeTabId: state.activeTabId, recency: state.recency, entityLabels };
  }
  ```
  Motivo explícito en el comentario del código: evitar que nombres de entidades (a veces datos sensibles, ej. el texto de búsqueda de una investigación OSINT) queden indefinidamente en el navegador después de cerrar la tab que los mostraba, y evitar fugas entre sesiones/usuarios que comparten perfil de navegador. **En memoria** (mientras la pestaña del navegador sigue abierta) `entityLabels` NO se poda al cerrar una tab — solo se recorta lo que se persiste a disco.
- Al recargar la página, Zustand rehidrata `tabs`, `activeTabId`, `recency` y el `entityLabels` recortado — las tabs vuelven a aparecer en la barra en el mismo orden y con el mismo label guardado.
- Manejo de error: si `localStorage.setItem` falla (cuota llena o deshabilitado), se atrapa el error y el store sigue funcionando solo en memoria para esa sesión de página, sin romper la app.

## 10. Estilos — clases literales por estado

Todo vive en `src/components/tabs/tab-bar.tsx`. El proyecto usa Tailwind v4 + shadcn/ui (estilo "new-york"), con dark mode controlado por atributo (`[data-theme="dark"]`, NO la clase `.dark` de Tailwind por defecto — ver `@custom-variant dark (&:is([data-theme="dark"] *));` en `src/app/globals.css`).

### 10.1. Contenedor de la barra

```
flex shrink-0 items-stretch gap-0.5 overflow-x-auto border-b bg-slate-200 px-2 pt-2 dark:bg-slate-900
```
- `overflow-x-auto`: si hay muchas tabs abiertas, la barra scrollea horizontalmente (no hay wrap ni menú de overflow "▾"). No hay indicador visual de que hay más tabs fuera de vista más allá del scroll nativo del navegador.
- Si `tabs.length === 0`, el componente retorna `null` (la barra ni se renderiza).

### 10.2. Cada pestaña (contenedor)

```
group flex shrink-0 items-center gap-2 rounded-t-md border border-b-0 max-w-[180px]
```
más, condicional:

- **Activa**:
  ```
  border-border bg-slate-500 text-white dark:bg-slate-600
  ```
- **Inactiva**:
  ```
  border-transparent bg-slate-100/70 text-muted-foreground shadow-sm hover:bg-slate-300/60 hover:shadow dark:bg-slate-800/60 dark:hover:bg-slate-700/60
  ```

Notas: la tab activa usa color de fondo sólido `slate-500`/`slate-600` (dark) con texto blanco — no usa los tokens semánticos del theme (`--primary`, etc.) para el fondo activo; el borde sí usa el token `border-border`. La tab inactiva usa `text-muted-foreground` (token semántico) sobre fondos `slate-*` con opacidad. `max-w-[180px]` trunca tabs con labels largos.

### 10.3. El link del título (dentro de la tab)

```
min-w-0 flex-1 truncate px-3 py-1.5 text-xs font-medium
```
Es un `<Link href={tab.path}>` de `next/link`, con `aria-current="page"` cuando está activa. `truncate` corta el texto con ellipsis si excede el ancho.

### 10.4. Botón de cerrar

```
mr-1.5 shrink-0 rounded-sm p-0.5 opacity-0 group-hover:opacity-100
```
más, condicional:
- Sobre tab activa: `hover:bg-slate-400/60`
- Sobre tab inactiva: `hover:bg-muted`

Ícono: `<X className="h-3 w-3" />` de `lucide-react`. El botón está **invisible por defecto** (`opacity-0`) y solo aparece al hacer hover sobre la tab entera (`group-hover:opacity-100` — el contenedor de la tab tiene la clase `group`). No hay estado "focus-visible" explícito documentado en el código para el botón de cerrar — **no confirmado por código si es accesible por teclado sin hover** (revisar manualmente con Tab+Enter).

`aria-label` dinámico: `` `Cerrar pestaña ${resolveTabLabel(tab, entityLabels)}` `` — usa el nombre resuelto (real), no el CUID.

### 10.5. Focus / accesibilidad

No se encontraron clases `focus:` / `focus-visible:` custom en `tab-bar.tsx` — el foco de teclado usa el estilo por defecto del navegador o el heredado de `outline-ring/50` global (`src/app/globals.css`: `* { @apply border-border outline-ring/50; }`).

Paleta de tokens semánticos referenciados (definidos en `src/app/globals.css`, con variante clara/oscura vía `[data-theme="light"]` / `[data-theme="dark"]`):
- `--border`: claro `hsl(209 30% 84%)`, oscuro `hsl(209 65% 20%)`.
- `--muted-foreground`: claro `hsl(211 25% 35%)`, oscuro `hsl(207 26% 71%)`.
- `--card`: claro `hsl(0 0% 100%)`, oscuro `hsl(210 72% 9%)`.

Los colores literales `slate-*` de la barra de tabs son de la paleta base de Tailwind (no tokens custom del proyecto) y no cambian según el theme salvo por los pares explícitos `bg-X dark:bg-Y` escritos a mano en `tab-bar.tsx`.

## 11. Cierre de una tab — flujo completo

`tab-bar.tsx`, `onClick` del botón cerrar:

```tsx
onClick={() => {
  closeTab(tab.id);
  const nextActiveId = useTabStore.getState().activeTabId;
  if (nextActiveId && nextActiveId !== pathname) {
    router.push(nextActiveId);
  }
}}
```

1. `closeTab(id)` actualiza el store (sección 6.2): quita la tab, y si era la activa, activa la más recientemente usada entre las que quedan (o `null` si no queda ninguna).
2. Si tras cerrar hay una nueva tab activa y esa ruta es distinta de la URL actual del navegador (`pathname`), se hace `router.push(nextActiveId)` — esto SÍ dispara una navegación real de Next.js hacia esa ruta (confirmado por test: cerrar la tab activa navega a la nueva tab activa; cerrar una tab inactiva NO navega, porque `pathname` no cambia).
3. Si no queda ninguna tab activa (se cerró la última tab), no hay `router.push` — el usuario se queda en la URL actual aunque la barra de tabs quede vacía (y de hecho `TabBar` retorna `null` en ese caso).

## 12. Límites, atajos y edge cases confirmados por código/tests

- **Límite duro**: `MAX_TABS = 10`. Al superarlo se evicta la tab LRU (nunca la nueva ni la reactivada). No hay aviso visual de "límite alcanzado" en el código revisado.
- **No hay atajos de teclado** (ej. Ctrl+W, Ctrl+Tab) — no se encontró ningún listener de `keydown` en `src/components/tabs/`.
- **No hay drag-and-drop / reordenamiento de tabs** — no se encontró ningún uso de `@dnd-kit` (que sí existe en el proyecto para otras features) dentro de `src/components/tabs/`. El orden de las tabs en la barra es simplemente el orden de `tabs` en el store (orden de apertura), no el de `recency`.
- **Overflow de tabs**: scroll horizontal nativo (`overflow-x-auto`), sin menú desplegable de tabs ocultas.
- **Prevención de duplicados**: abrir una ruta ya abierta reactiva la tab existente, nunca crea una segunda.
- **Segmentos reservados** (`"new"`) nunca se tratan como id dinámico en el registro de "cero recarga", para no romper la página estática de creación.
- **Auditoría/costos de fetch**: `useHasBeenVisible` es un "latch" monótono (`false→true`, nunca `true→false`) usado como `enabled` de cada `useQuery` en los tab-content — evita re-fetchear (y en este dominio AML/KYC, re-auditar en `AuditLog` un acceso a datos sensibles) cada vez que el usuario cambia de tab y vuelve, pero también evita fetchear tabs que el usuario jamás llegó a mirar aunque estén "abiertas" en el store.
- **Errores de permiso dentro de una tab**: si el fetch de una tab-content falla con un mensaje que matchea `/autorizado|permiso/i`, se muestra `<AccessDenied />` en vez de un mensaje de error genérico (`isPermissionError`, `tab-query-errors.ts`).
- **Sensibilidad de datos en persistencia**: ver sección 9 — `entityLabels` se recorta antes de persistir a `localStorage` específicamente por motivos de confidencialidad (dato citado en comentario del código: el texto de búsqueda de una investigación OSINT).

## 13. Resumen accionable para reimplementar en otro proyecto

1. Crear un store (Zustand u otro) con: `tabs: {id, path, label}[]`, `activeTabId`, `recency: string[]`, `entityLabels: Record<string,string>`, y las 5 acciones descritas en la sección 6. Persistir a `localStorage` con una función `partialize` que recorte `entityLabels` a los ids que sigan en algún path abierto (si hay datos sensibles en los labels).
2. Definir `MAX_TABS` y la lógica de evicción LRU basada en `recency`.
3. Un mapa estático `SEGMENT_LABELS` (segmento de URL → texto humano) + una función `labelForPath` con fallback a title-case.
4. Una función `resolveTabLabel(tab, entityLabels)` que escanea los segmentos del path de derecha a izquierda buscando un id conocido, con fallback al label guardado de la tab.
5. Un hook que en un `useEffect` sobre `pathname` (o el router del framework) llame `openTab` en cada navegación real — esto por sí solo ya da tabs "con recarga completa" (más simple, suficiente si no se necesita el modo cero-recarga).
6. (Opcional, más complejo) Si se quiere evitar recargas para ciertas rutas dinámicas: un registro `{pattern, Component}[]` + matcher segmento-a-segmento, un listener de click en fase de captura sobre `document` que intercepte esos `<a>`, haga `preventDefault` + `history.pushState`, y un host que monte TODAS las tabs registradas abiertas ocultas con el atributo `hidden` (no CSS `display:none` vía JS condicional, para no desmontar y perder estado/caché de React Query), mostrando solo la que corresponda a `visiblePath`.
7. Componente `TabBar`: contenedor `overflow-x-auto` con una fila de "chips" (link truncado + botón X que aparece en hover); estados visual activa/inactiva/hover como en la sección 10 (ajustar paleta al proyecto destino).
8. Enlazar `entityLabels` también al sistema de breadcrumbs, si existe, para que ambos compartan el mismo caché de "id → nombre real" y un `registerEntityLabel`/componente sin-render se pueda llamar desde cualquier página SSR que ya conozca el nombre.
