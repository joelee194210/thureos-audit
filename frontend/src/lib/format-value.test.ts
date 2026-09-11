import { describe, it, expect } from "vitest";
import { classifyValue, formatValue } from "./format-value";

describe("formatValue", () => {
  it("formatea números con separador de miles es-CO", () => {
    expect(formatValue(1234567)).toBe("1.234.567");
    expect(formatValue(1234.5)).toBe("1.234,5");
  });

  it("muestra un guion largo para vacíos", () => {
    expect(formatValue(null)).toBe("—");
    expect(formatValue(undefined)).toBe("—");
    expect(formatValue("")).toBe("—");
  });

  it("traduce booleanos", () => {
    expect(formatValue(true)).toBe("Sí");
    expect(formatValue(false)).toBe("No");
  });

  it("formatea fechas ISO que llegan como texto", () => {
    // Las colecciones dinámicas guardan fechas como string ISO.
    expect(formatValue("2026-03-15T10:30:00Z")).not.toContain("T");
    expect(formatValue("2026-03-15T10:30:00Z")).not.toBe("2026-03-15T10:30:00Z");
  });

  it("deja el texto común sin tocar", () => {
    expect(formatValue("Bancolombia")).toBe("Bancolombia");
  });

  it("serializa objetos", () => {
    expect(formatValue({ a: 1 })).toBe('{"a":1}');
  });

  it("no confunde un texto que empieza con dígitos con una fecha", () => {
    expect(formatValue("2026 fue un buen año")).toBe("2026 fue un buen año");
  });
});

describe("classifyValue", () => {
  it("distingue las clases que la tabla necesita para alinear", () => {
    expect(classifyValue(42)).toBe("number");
    expect(classifyValue("2026-03-15T10:30:00Z")).toBe("date");
    expect(classifyValue(true)).toBe("boolean");
    expect(classifyValue(null)).toBe("empty");
    expect(classifyValue("hola")).toBe("text");
  });

  it("un número en forma de string sigue siendo texto", () => {
    // No se adivina: si Mongo lo guardó como string, se muestra como
    // string. Convertir acá desalinearía la columna respecto del dato.
    expect(classifyValue("42")).toBe("text");
  });
});
