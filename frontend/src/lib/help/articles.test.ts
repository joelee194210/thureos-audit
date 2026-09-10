import { existsSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { HELP_ARTICLES, articuloPorSlug } from "./articles";
import { HELP_GROUPS } from "./types";
import { NAV_AREAS } from "@/lib/nav";

describe("contenido de la ayuda", () => {
  it("no repite slugs", () => {
    const slugs = HELP_ARTICLES.map((a) => a.slug);
    expect(new Set(slugs).size).toBe(slugs.length);
  });

  it("usa solo grupos declarados", () => {
    for (const a of HELP_ARTICLES) {
      expect(HELP_GROUPS).toContain(a.group);
    }
  });

  it("no deja secciones vacías", () => {
    for (const a of HELP_ARTICLES) {
      expect(a.sections.length).toBeGreaterThan(0);
      for (const s of a.sections) {
        expect(s.heading.trim()).not.toBe("");
        if ("body" in s) expect(s.body.trim()).not.toBe("");
        else expect(s.items.length).toBeGreaterThan(0);
      }
    }
  });

  // Toda ficha abre igual: es la estructura que hace la ayuda predecible.
  it("abre con para qué sirve y quién puede usarlo", () => {
    for (const a of HELP_ARTICLES) {
      expect(a.sections[0].heading).toBe("¿Para qué sirve?");
      expect(a.sections[1].heading).toMatch(/^¿Quién puede usar/);
    }
  });

  it("encuentra por slug y devuelve undefined si no existe", () => {
    expect(articuloPorSlug("monitores")?.title).toBe("Monitores");
    expect(articuloPorSlug("no-existe")).toBeUndefined();
  });

  // El que atrapa el error real: renombrar una captura y olvidar la referencia.
  it("referencia solo imágenes que existen", () => {
    for (const a of HELP_ARTICLES) {
      for (const img of a.images) {
        const ruta = path.join(process.cwd(), "public", "ayuda", img);
        expect(existsSync(ruta), `falta ${img} (ficha ${a.slug})`).toBe(true);
      }
    }
  });

  // La única defensa contra que la ayuda se quede atrás del producto: si
  // alguien agrega una pantalla al menú y no la documenta, esto lo dice.
  it("tiene una ficha por cada pantalla del menú", () => {
    const rutasDocumentadas = new Set(
      HELP_ARTICLES.map((a) => a.route).filter((r): r is string => r !== null),
    );
    const sinFicha = NAV_AREAS.flatMap((area) => area.items)
      .filter((item) => item.href !== "/ayuda")
      .filter((item) => !rutasDocumentadas.has(item.href))
      .map((item) => `${item.name} (${item.href})`);

    expect(sinFicha, `pantallas sin ficha de ayuda: ${sinFicha.join(", ")}`).toEqual([]);
  });
});
