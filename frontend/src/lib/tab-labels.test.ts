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
