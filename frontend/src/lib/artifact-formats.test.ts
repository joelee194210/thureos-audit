import { describe, it, expect } from "vitest";
import { availableFormats, unavailableFormatsNotice } from "./artifact-formats";
import type { ChatArtifact } from "./api/chat";

function artifact(type: ChatArtifact["type"]): ChatArtifact {
  return { type, title: "Artefacto de prueba" };
}

describe("availableFormats", () => {
  it("un gráfico ofrece PNG, CSV, Excel, PDF y HTML", () => {
    expect(availableFormats(artifact("chart")).map((f) => f.format)).toEqual([
      "png",
      "csv",
      "xlsx",
      "pdf",
      "html",
    ]);
  });

  it("una tabla ofrece CSV, Excel, PDF y HTML, sin PNG", () => {
    const formats = availableFormats(artifact("table")).map((f) => f.format);
    expect(formats).toEqual(["csv", "xlsx", "pdf", "html"]);
    expect(formats).not.toContain("png");
  });

  it("un artefacto custom solo ofrece HTML e Imprimir: nada de PDF ni Excel", () => {
    const formats = availableFormats(artifact("custom")).map((f) => f.format);
    expect(formats).toEqual(["html", "print"]);
    expect(formats).not.toContain("pdf");
    expect(formats).not.toContain("xlsx");
  });
});

describe("unavailableFormatsNotice", () => {
  it("explica por qué custom no tiene PDF ni Excel, en vez de omitirlo en silencio", () => {
    const notice = unavailableFormatsNotice(artifact("custom"));
    expect(notice).toMatch(/PDF/);
    expect(notice).toMatch(/Excel/);
  });

  it("no hay nada que explicar para chart o table", () => {
    expect(unavailableFormatsNotice(artifact("chart"))).toBeNull();
    expect(unavailableFormatsNotice(artifact("table"))).toBeNull();
  });
});
