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

  it("rechaza vacío (o solo espacios/control) como null, no como ''", () => {
    // href="" resuelve a la página actual: no es "sin enlace", es un
    // enlace vivo a otro lado. El contrato es null = "sin enlace".
    expect(safeHref("")).toBeNull();
    expect(safeHref("   ")).toBeNull();
  });

  it("acepta protocolo-relativo como enlace externo (documentado, no un agujero)", () => {
    // "//evil.com" no tiene esquema propio y entra por la rama de rutas
    // relativas, así que termina en un enlace externo vivo. No es una
    // superficie nueva: "https://evil.com" ya está permitido igual.
    expect(safeHref("//evil.com")).toBe("//evil.com");
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

  it("no interpreta HTML crudo: lo deja como token 'html' para tratar como texto", () => {
    const tokens = lexMarkdown("<script>alert(1)</script>");
    // Aserción real, no "la subcadena 'script' aparece en algún lado del
    // JSON": eso pasaría igual si marked lo tokenizara como párrafo,
    // codespan, o cualquier otra cosa -incluido un futuro cambio de marked
    // que devolviera marcado ya resuelto-. Lo que importa es que el tipo
    // sea "html" (nunca interpretado) y que el texto crudo se conserve
    // intacto, sin escapar ni transformar, listo para que el componente lo
    // trate como texto plano.
    expect(tokens).toHaveLength(1);
    expect(tokens[0].type).toBe("html");
    expect((tokens[0] as { raw: string }).raw).toBe(
      "<script>alert(1)</script>",
    );
  });

  it("tokeniza una tabla", () => {
    const tokens = lexMarkdown("| a | b |\n|---|---|\n| 1 | 2 |");
    expect(tokens.map((t) => t.type)).toContain("table");
  });

  it("devuelve lista vacía para entrada vacía", () => {
    expect(lexMarkdown("")).toEqual([]);
  });
});
