import { describe, it, expect } from "vitest";
import { escapeHTML, buildArtifactHTML, exportArtifactPDF } from "./artifact-report";
import type { ChatArtifact } from "./api/chat";

describe("escapeHTML", () => {
  it("reemplaza los cinco caracteres que importan en HTML", () => {
    expect(escapeHTML(`&<>"'`)).toBe("&amp;&lt;&gt;&quot;&#39;");
  });

  it("no escapa dos veces el & que ella misma genera", () => {
    // Si el orden de los replace fuera al revés, "<" produciría "&lt;" y
    // el "&" de esa entidad volvería a escaparse a "&amp;lt;".
    expect(escapeHTML("<")).toBe("&lt;");
  });

  it("trata null/undefined como cadena vacía", () => {
    expect(escapeHTML(null)).toBe("");
    expect(escapeHTML(undefined)).toBe("");
  });

  it("convierte valores no-string antes de escapar", () => {
    expect(escapeHTML(42)).toBe("42");
  });
});

const baseArtifact: ChatArtifact = {
  type: "table",
  title: "Reporte de prueba",
};

describe("buildArtifactHTML", () => {
  it("escapa un título hostil de punta a punta: no debe sobrevivir ninguna etiqueta", () => {
    const html = buildArtifactHTML(
      { ...baseArtifact, title: `"><script>alert(1)</script>` },
      [{ a: 1 }],
      "2026-01-01T00:00:00Z",
      [],
    );
    expect(html).not.toContain("<script");
    expect(html).toContain("&lt;script&gt;");
  });

  it("escapa marcado embebido en una celda de dato", () => {
    const html = buildArtifactHTML(
      baseArtifact,
      [{ nota: "</td></tr><script>alert(1)</script>" }],
      "2026-01-01T00:00:00Z",
      [],
    );
    expect(html).not.toContain("<script");
    expect(html).toContain("&lt;script&gt;");
  });

  it("escapa nombres de columna y de monitores", () => {
    const html = buildArtifactHTML(
      baseArtifact,
      [{ '"><img src=x>': 1 }],
      "2026-01-01T00:00:00Z",
      ['"><b>hola'],
    );
    expect(html).not.toContain("<img");
    expect(html).not.toContain("<b>hola");
  });

  it("imprime la fecha de corrida y la de generación por separado", () => {
    const html = buildArtifactHTML(
      baseArtifact,
      [{ a: 1 }],
      "2026-01-01T00:00:00Z",
      [],
    );
    expect(html).toContain("Datos al:");
    expect(html).toContain("Generado:");
  });

  it("muestra un mensaje en vez de una tabla vacía cuando no hay filas", () => {
    const html = buildArtifactHTML(baseArtifact, [], "2026-01-01T00:00:00Z", []);
    expect(html).toContain("La consulta no devolvió resultados.");
    expect(html).not.toContain("<table>");
  });

  it("solo embebe la imagen del gráfico si es un data URI de imagen", () => {
    const chartArtifact: ChatArtifact = { ...baseArtifact, type: "chart" };
    const withImage = buildArtifactHTML(
      chartArtifact,
      [{ a: 1 }],
      "2026-01-01T00:00:00Z",
      [],
      "data:image/png;base64,AAAA",
    );
    expect(withImage).toContain("data:image/png;base64,AAAA");

    const withoutImage = buildArtifactHTML(
      chartArtifact,
      [{ a: 1 }],
      "2026-01-01T00:00:00Z",
      [],
      null,
    );
    expect(withoutImage).not.toContain("<img");

    // Un valor que no sea data URI de imagen (ej. algo inesperado) tampoco
    // se inserta como marcado.
    const withGarbage = buildArtifactHTML(
      chartArtifact,
      [{ a: 1 }],
      "2026-01-01T00:00:00Z",
      [],
      "not-a-data-uri",
    );
    expect(withGarbage).not.toContain("<img");
  });

  it("respeta la proyección y las etiquetas de columnas declaradas", () => {
    const html = buildArtifactHTML(
      {
        ...baseArtifact,
        chartSpec: {
          columns: ["monto"],
          labels: { monto: "Monto (COP)" },
        },
      },
      [{ monto: 1000, otra: "no debería salir" }],
      "2026-01-01T00:00:00Z",
      [],
    );
    expect(html).toContain("Monto (COP)");
    expect(html).not.toContain("no debería salir");
  });

  it("no toca document ni window: corre igual en el entorno node de vitest", () => {
    expect(() =>
      buildArtifactHTML(baseArtifact, [{ a: 1 }], "2026-01-01T00:00:00Z", []),
    ).not.toThrow();
  });

  it("no revienta si falta ranAt: es opcional en ChatArtifact (instantánea sin sources)", () => {
    expect(() =>
      buildArtifactHTML(baseArtifact, [{ a: 1 }], undefined, []),
    ).not.toThrow();
    const html = buildArtifactHTML(baseArtifact, [{ a: 1 }], undefined, []);
    expect(html).toContain("Datos al: sin fecha de corrida");
  });

  it("no revienta con una fecha de corrida inválida", () => {
    expect(() =>
      buildArtifactHTML(baseArtifact, [{ a: 1 }], "no-es-una-fecha", []),
    ).not.toThrow();
    const html = buildArtifactHTML(
      baseArtifact,
      [{ a: 1 }],
      "no-es-una-fecha",
      [],
    );
    expect(html).toContain("Datos al: sin fecha de corrida");
  });

  it("un artefacto custom es su propio código, no una tabla envuelta en marca", () => {
    const code = "<!doctype html><html><body><h1>hola</h1></body></html>";
    const html = buildArtifactHTML(
      { type: "custom", title: "Panel a medida", code },
      [{ a: 1 }],
      "2026-01-01T00:00:00Z",
      [],
    );
    expect(html).toBe(code);
    // No lleva la cabecera de marca ni el mensaje de tabla vacía: es el
    // documento del LLM tal cual, no la plantilla de Thureos Compliance.
    expect(html).not.toContain("Thureos Compliance");
    expect(html).not.toContain("La consulta no devolvió resultados.");
  });

  it("un custom sin código no revienta: sale un documento vacío válido", () => {
    const html = buildArtifactHTML(
      { type: "custom", title: "Sin código" },
      [],
      "2026-01-01T00:00:00Z",
      [],
    );
    expect(html).toContain("<html>");
  });

  it("no incluye recursos externos: sin http(s)://, @import ni url()", () => {
    const html = buildArtifactHTML(
      {
        ...baseArtifact,
        chartSpec: { columns: ["a"], labels: { a: "Columna A" } },
      },
      [{ a: 1 }],
      "2026-01-01T00:00:00Z",
      ["Monitor de prueba"],
    );
    expect(html).not.toMatch(/https?:\/\//);
    expect(html).not.toContain("@import");
    expect(html).not.toContain("url(");
  });

  it("un gráfico embebido tampoco introduce recursos externos: el data URI no es una URL de red", () => {
    const html = buildArtifactHTML(
      { ...baseArtifact, type: "chart" },
      [{ a: 1 }],
      "2026-01-01T00:00:00Z",
      [],
      "data:image/png;base64,AAAA",
    );
    expect(html).not.toMatch(/https?:\/\//);
    expect(html).not.toContain("@import");
    expect(html).not.toContain("url(");
  });
});

describe("exportArtifactPDF", () => {
  it("rechaza un artefacto custom en vez de generar un PDF sin sentido", async () => {
    await expect(
      exportArtifactPDF(
        { type: "custom", title: "Panel a medida", code: "<h1>hola</h1>" },
        [],
        "2026-01-01T00:00:00Z",
        [],
      ),
    ).rejects.toThrow(/informe en PDF/);
  });
});
