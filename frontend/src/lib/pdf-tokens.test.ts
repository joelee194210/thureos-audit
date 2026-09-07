import { describe, expect, it } from "vitest";
import { parseColor, readReportColors } from "@/lib/pdf-tokens";

describe("parseColor", () => {
  it("resuelve hex de 6 dígitos", () => {
    expect(parseColor("#04182c")).toEqual([4, 24, 44]);
  });

  it("resuelve hex de 3 dígitos duplicando cada dígito", () => {
    expect(parseColor("#abc")).toEqual([170, 187, 204]);
  });

  it("resuelve rgb() con comas", () => {
    expect(parseColor("rgb(52, 211, 153)")).toEqual([52, 211, 153]);
  });

  it("resuelve rgba() con espacios y descarta el alfa", () => {
    expect(parseColor("rgba(255 190 69 / 0.14)".replace("/", ","))).toEqual([
      255, 190, 69,
    ]);
    expect(parseColor("rgba(255, 190, 69, .14)")).toEqual([255, 190, 69]);
  });

  it("devuelve null ante un valor irreconocible", () => {
    // Es lo que llega si el nombre del token está mal escrito y
    // getComputedStyle devuelve la cadena vacía: sin este null, el informe
    // saldría con [NaN, NaN, NaN] en vez de caer al color de respaldo.
    expect(parseColor("")).toBeNull();
    expect(parseColor("not-a-color")).toBeNull();
    expect(parseColor("#ab")).toBeNull();
  });
});

describe("readReportColors", () => {
  it("cae al gris de respaldo fuera del navegador (SSR)", () => {
    // Este test corre en el entorno 'node' de vitest, sin DOM — el mismo
    // camino que toma el módulo en un render de servidor.
    const resolved = readReportColors({
      ink: "--fg-default",
      muted: "--fg-muted",
    });
    expect(resolved.ink).toEqual([26, 26, 26]);
    expect(resolved.muted).toEqual([26, 26, 26]);
  });
});
