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
