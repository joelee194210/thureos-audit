/**
 * Tests del canvas de artefactos, en el mismo estilo que markdown.test.tsx:
 * se renderiza el componente DE VERDAD con `renderToStaticMarkup` y se
 * mira el marcado. No hace falta jsdom ni mocks de la API — los efectos no
 * corren en el render del servidor, así que el canvas se queda exactamente
 * en el estado inicial, que es justo el que estos tres hallazgos describen.
 */
import { describe, it, expect } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { ArtifactCanvas } from "./artifact-canvas";
import type { ChatArtifact } from "@/lib/api/chat";

const VACIO = "La consulta no devolvió resultados.";
const SIN_ID = "Este artefacto no quedó vinculado a datos: no se puede guardar.";

describe("ArtifactCanvas — gráfico sin filas (hallazgo 5)", () => {
  it("muestra el estado vacío en vez de dibujar ejes sin marcas", () => {
    // Sin `sources` y sin id no se dispara ninguna corrida: el canvas
    // arranca y se queda con cero filas, que es el estado que el spec
    // manda mostrar explícito ("no un gráfico en blanco").
    const artifact: ChatArtifact = {
      type: "chart",
      title: "Montos por región",
      chartSpec: { chartType: "bar", xKey: "_id", yKeys: ["aggValue"], data: [] },
    };
    expect(renderToStaticMarkup(<ArtifactCanvas artifact={artifact} />)).toContain(VACIO);
  });

  it("con filas sí dibuja el gráfico y no el estado vacío", () => {
    const artifact: ChatArtifact = {
      type: "chart",
      title: "Montos por región",
      chartSpec: {
        chartType: "bar",
        xKey: "_id",
        yKeys: ["aggValue"],
        data: [{ _id: "Caribe", aggValue: 100 }],
      },
    };
    expect(renderToStaticMarkup(<ArtifactCanvas artifact={artifact} />)).not.toContain(VACIO);
  });
});

describe("ArtifactCanvas — orden de columnas declarado (hallazgo 6)", () => {
  it("la tabla usa el orden de chartSpec.columns, no el de las claves de la fila", () => {
    // Las claves de la fila llegan en orden alfabético (así las serializa
    // el map de Go); las declaradas dicen otra cosa, y es esa la que el
    // XLSX, el HTML y el PDF ya respetaban.
    const artifact: ChatArtifact = {
      type: "table",
      title: "Operaciones",
      chartSpec: {
        columns: ["fecha", "monto", "beneficiario"],
        labels: { fecha: "Fecha", monto: "Monto", beneficiario: "Beneficiario" },
        data: [{ beneficiario: "ACME", fecha: "2026-03-15T10:00:00Z", monto: 10 }],
      },
    };
    const html = renderToStaticMarkup(<ArtifactCanvas artifact={artifact} />);

    const cabeceras = [...html.matchAll(/<th[^>]*>([^<]*)<\/th>/g)].map((m) => m[1]);
    expect(cabeceras).toEqual(["Fecha", "Monto", "Beneficiario"]);
  });
});

describe("ArtifactCanvas — artefacto sin id (hallazgo 2)", () => {
  it("deshabilita Guardar y deja ver el motivo", () => {
    // El backend ya no serializa el id de ceros de un artefacto legacy, así
    // que `artifact.id` llega undefined y el guarda `!artifact.id` —que con
    // la cadena "000...0", truthy en JS, no se disparaba nunca— sí actúa:
    // el botón queda deshabilitado y su tooltip explicativo, alcanzable.
    const legacy: ChatArtifact = {
      type: "table",
      title: "Ventas",
      chartSpec: { data: [{ region: "Caribe" }] },
    };
    expect(renderToStaticMarkup(<ArtifactCanvas artifact={legacy} />)).toContain(SIN_ID);
  });

  it("con id, Guardar está disponible y sin motivo que explicar", () => {
    const guardable: ChatArtifact = {
      id: "68c1f0a2b3c4d5e6f7a8b9c0",
      type: "table",
      title: "Ventas",
      chartSpec: { data: [{ region: "Caribe" }] },
    };
    expect(renderToStaticMarkup(<ArtifactCanvas artifact={guardable} />)).not.toContain(SIN_ID);
  });

  it("sin fecha de corrida lo dice en palabras, no como una fecha", () => {
    const legacy: ChatArtifact = { type: "table", title: "Ventas" };
    const html = renderToStaticMarkup(<ArtifactCanvas artifact={legacy} />);
    expect(html).toContain("Sin corrida registrada");
    expect(html).not.toContain("1/1/0001");
  });
});
