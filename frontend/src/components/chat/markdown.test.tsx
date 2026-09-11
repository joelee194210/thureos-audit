/**
 * Prueba empírica de la invariante de seguridad: renderiza el componente
 * de verdad (no lee el código, lo ejecuta) y verifica que el marcado
 * generado nunca contiene el tag hostil que un string del LLM podría
 * intentar colar.
 *
 * `renderToStaticMarkup` alcanza porque `Markdown` no usa hooks ni nada
 * que dependa del navegador: es una función pura de texto a JSX. No hace
 * falta jsdom ni ninguna dependencia nueva.
 */
import { describe, it, expect } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { Markdown } from "./markdown";

describe("Markdown", () => {
  it("no interpreta <script> como HTML: el tag no aparece en el marcado", () => {
    const html = renderToStaticMarkup(
      <Markdown>{"<script>alert(1)</script>"}</Markdown>,
    );
    expect(html).not.toContain("<script>");
    expect(html).not.toContain("<script ");
    // El texto sigue presente, solo que como texto escapado.
    expect(html).toContain("script");
  });

  it("no genera un <a> para un href javascript:", () => {
    const html = renderToStaticMarkup(
      <Markdown>{"[click](javascript:alert(1))"}</Markdown>,
    );
    expect(html).not.toContain("<a");
    // El contenido del link no se pierde, solo la navegación.
    expect(html).toContain(">click<");
  });

  it("no genera un <a> para un href data:", () => {
    const html = renderToStaticMarkup(
      <Markdown>
        {"[click](data:text/html,<script>alert(1)</script>)"}
      </Markdown>,
    );
    expect(html).not.toContain("<a");
  });

  it("HTML crudo dentro de una celda de tabla no se interpreta", () => {
    const html = renderToStaticMarkup(
      <Markdown>
        {"| a |\n|---|\n| <img src=x onerror=alert(1)> |"}
      </Markdown>,
    );
    expect(html).not.toContain("<img");
    // El texto crudo de la celda sigue presente, escapado como texto.
    expect(html).toContain("&lt;img");
  });

  it("no dispara un <img> para una imagen de markdown (decisión, no bug)", () => {
    const html = renderToStaticMarkup(
      <Markdown>{"![alt](http://evil.com/track.png)"}</Markdown>,
    );
    expect(html).not.toContain("<img");
  });

  it("desescapa un caracter escapado sin dejar la barra invertida visible", () => {
    const html = renderToStaticMarkup(<Markdown>{"5 \\* 3"}</Markdown>);
    expect(html).toContain("5 * 3");
    expect(html).not.toContain("\\*");
  });

  it("distingue una tarea pendiente de una hecha", () => {
    const pendingHtml = renderToStaticMarkup(
      <Markdown>{"- [ ] pendiente"}</Markdown>,
    );
    const doneHtml = renderToStaticMarkup(<Markdown>{"- [x] hecho"}</Markdown>);

    // La casilla marcada trae el atributo checked; la pendiente, no.
    expect(doneHtml).toMatch(/<input[^>]*checked[^>]*>/);
    expect(pendingHtml).not.toMatch(/<input[^>]*checked[^>]*>/);
  });
});
