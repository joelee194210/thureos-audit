import { describe, expect, it } from "vitest";
import { esAgrupada } from "./types";

describe("esAgrupada", () => {
  it("es verdadero para los tipos que traen grupo", () => {
    expect(esAgrupada("aggregate")).toBe(true);
    expect(esAgrupada("velocity")).toBe(true);
  });

  it("es falso para las alertas de fila", () => {
    expect(esAgrupada("row")).toBe(false);
  });
});
