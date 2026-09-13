import { describe, it, expect } from "vitest";
import {
  isRerunnable,
  deriveSeriesFromRows,
  resolveInitialRows,
  resolveInitialSeries,
  resolveRunSeries,
  type ArtifactRun,
} from "./artifacts";
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

describe("resolveInitialRows", () => {
  it("prioriza cachedData sobre chartSpec.data", () => {
    const artifact: ChatArtifact = {
      type: "chart",
      title: "x",
      cachedData: [{ a: 1 }],
      chartSpec: { data: [{ a: 2 }] },
    };
    expect(resolveInitialRows(artifact)).toEqual([{ a: 1 }]);
  });

  it("sin cachedData cae a chartSpec.data (instantánea legacy)", () => {
    const artifact: ChatArtifact = {
      type: "chart",
      title: "x",
      chartSpec: { data: [{ a: 2 }] },
    };
    expect(resolveInitialRows(artifact)).toEqual([{ a: 2 }]);
  });

  it("sin ninguno de los dos devuelve un arreglo vacío", () => {
    expect(resolveInitialRows({ type: "chart", title: "x" })).toEqual([]);
  });
});

describe("resolveInitialSeries", () => {
  it("con cachedData deriva la serie de esas filas, NO de chartSpec.yKeys viejas", () => {
    // yKeys ("aggValue") es la ejecución original de una sola fuente; el
    // pivote posterior a varias fuentes renombró las columnas cacheadas.
    // Invertir la rama de este if es el fallo silencioso que la función
    // existe para evitar — de ahí el test explícito de la rama contraria.
    const artifact: ChatArtifact = {
      type: "chart",
      title: "x",
      chartSpec: { xKey: "fecha", yKeys: ["aggValue"] },
      cachedData: [{ fecha: "2026-01-01", montoBanco: 10, montoFintech: 5 }],
    };
    expect(resolveInitialSeries(artifact)).toEqual(["montoBanco", "montoFintech"]);
  });

  it("sin cachedData usa chartSpec.yKeys (instantánea legacy, sí describe esos datos)", () => {
    const artifact: ChatArtifact = {
      type: "chart",
      title: "x",
      chartSpec: { xKey: "fecha", yKeys: ["monto"], data: [{ fecha: "x", monto: 1 }] },
    };
    expect(resolveInitialSeries(artifact)).toEqual(["monto"]);
  });

  it("sin cachedData ni yKeys devuelve un arreglo vacío", () => {
    expect(resolveInitialSeries({ type: "chart", title: "x" })).toEqual([]);
  });
});

describe("resolveRunSeries", () => {
  it("usa run.series cuando viene poblada", () => {
    const run: ArtifactRun = {
      data: [{ fecha: "x", monto: 1 }],
      series: ["monto"],
      ranAt: "2026-01-01T00:00:00Z",
    };
    expect(resolveRunSeries(run, "fecha")).toEqual(["monto"]);
  });

  it("con una sola fuente, ParseArtifact permite yKeys vacías: run.series llega [] con filas reales — se deriva de esas filas en vez de graficar ejes sin barras", () => {
    const run: ArtifactRun = {
      data: [
        { fecha: "2026-01-01", monto: 100 },
        { fecha: "2026-01-02", monto: 200 },
      ],
      series: [],
      ranAt: "2026-01-01T00:00:00Z",
    };
    expect(resolveRunSeries(run, "fecha")).toEqual(["monto"]);
  });

  it("series vacía y sin filas tampoco inventa una columna", () => {
    const run: ArtifactRun = { data: [], series: [], ranAt: "2026-01-01T00:00:00Z" };
    expect(resolveRunSeries(run, "fecha")).toEqual([]);
  });
});
