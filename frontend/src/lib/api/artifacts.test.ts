import { describe, it, expect } from "vitest";
import { isRerunnable, deriveSeriesFromRows } from "./artifacts";
import type { ChatArtifact } from "./chat";

describe("isRerunnable", () => {
  it("es re-ejecutable si declara al menos una fuente", () => {
    const artifact: ChatArtifact = {
      type: "chart",
      title: "x",
      sources: [{ monitor: "m1" }],
    };
    expect(isRerunnable(artifact)).toBe(true);
  });

  it("una instantánea (sin sources) no es re-ejecutable", () => {
    const artifact: ChatArtifact = { type: "chart", title: "x" };
    expect(isRerunnable(artifact)).toBe(false);
  });

  it("un arreglo de sources vacío tampoco es re-ejecutable", () => {
    const artifact: ChatArtifact = { type: "chart", title: "x", sources: [] };
    expect(isRerunnable(artifact)).toBe(false);
  });
});

describe("deriveSeriesFromRows", () => {
  it("junta las columnas de todas las filas, sin la del eje X", () => {
    const rows = [
      { fecha: "2026-01-01", montoA: 10, montoB: 20 },
      { fecha: "2026-01-02", montoA: 5, montoC: 1 },
    ];
    expect(deriveSeriesFromRows(rows, "fecha")).toEqual([
      "montoA",
      "montoB",
      "montoC",
    ]);
  });

  it("sin xKey no descarta ninguna columna", () => {
    const rows = [{ a: 1, b: 2 }];
    expect(deriveSeriesFromRows(rows, undefined)).toEqual(["a", "b"]);
  });

  it("sin filas devuelve un arreglo vacío", () => {
    expect(deriveSeriesFromRows([], "fecha")).toEqual([]);
  });
});
