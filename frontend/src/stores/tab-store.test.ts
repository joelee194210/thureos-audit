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
    const result = partialize(state);
    expect(result.entityLabels).toEqual({ abc123: "CBCG Tarjetas" });
  });
});
