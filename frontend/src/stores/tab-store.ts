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
