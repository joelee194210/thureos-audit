import { describe, it, expect } from "vitest";
import { safeHref, lexMarkdown } from "./markdown-tokens";

describe("safeHref", () => {
  it("acepta los esquemas seguros", () => {
    expect(safeHref("https://thureos.com")).toBe("https://thureos.com");
    expect(safeHref("http://thureos.com")).toBe("http://thureos.com");
    expect(safeHref("mailto:a@b.com")).toBe("mailto:a@b.com");
  });

  it("rechaza javascript:", () => {
    expect(safeHref("javascript:alert(1)")).toBeNull();
  });

  it("rechaza javascript: disfrazado con espacios y mayúsculas", () => {
    expect(safeHref("  JaVaScRiPt:alert(1)")).toBeNull();
    expect(safeHref("java\tscript:alert(1)")).toBeNull();
    expect(safeHref("java\nscript:alert(1)")).toBeNull();
  });

  it("rechaza data: y otros esquemas", () => {
    expect(safeHref("data:text/html,<script>alert(1)</script>")).toBeNull();
    expect(safeHref("vbscript:msgbox(1)")).toBeNull();
    expect(safeHref("file:///etc/passwd")).toBeNull();
  });

  it("acepta rutas relativas", () => {
    expect(safeHref("/monitores/123")).toBe("/monitores/123");
  });
});

describe("lexMarkdown", () => {
  it("tokeniza los elementos que el asistente usa", () => {
    const tokens = lexMarkdown("# Título\n\n- uno\n- dos\n\n**negrita**");
    const types = tokens.map((t) => t.type);
    expect(types).toContain("heading");
    expect(types).toContain("list");
    expect(types).toContain("paragraph");
  });

  it("no interpreta HTML crudo: lo deja como token html para tratar como texto", () => {
    const tokens = lexMarkdown("<script>alert(1)</script>");
    // Sea cual sea el tipo que marked le asigne, el texto crudo se
    // conserva y NUNCA se convierte en marcado. El componente lo
    // renderiza como texto; acá solo verificamos que no desaparece.
    const raw = JSON.stringify(tokens);
    expect(raw).toContain("script");
  });

  it("tokeniza una tabla", () => {
    const tokens = lexMarkdown("| a | b |\n|---|---|\n| 1 | 2 |");
    expect(tokens.map((t) => t.type)).toContain("table");
  });

  it("devuelve lista vacía para entrada vacía", () => {
    expect(lexMarkdown("")).toEqual([]);
  });
});
