import { describe, expect, it } from "vitest";
import { ApiError, esNoEncontrado } from "./client";

// Antes el cliente lanzaba un Error pelado con el mensaje, así que nadie
// podía distinguir "esto no existe" de "el servidor falló". Una pestaña
// guardada apuntando a una entidad borrada repetía dos toasts de error en
// cada carga, para siempre y sin dar salida.
describe("ApiError", () => {
  it("conserva el código de estado y el mensaje del servidor", () => {
    const err = new ApiError("monitor not found", 404);
    expect(err.status).toBe(404);
    expect(err.message).toBe("monitor not found");
  });

  // Los call sites que ya existen hacen `err instanceof Error ? err.message`.
  // Si ApiError dejara de ser un Error, todos mostrarían el texto genérico.
  it("sigue siendo un Error, para no romper a quien ya lo atrapa", () => {
    const err: unknown = new ApiError("boom", 500);
    expect(err instanceof Error).toBe(true);
    expect(err instanceof Error ? err.message : "genérico").toBe("boom");
  });
});

describe("esNoEncontrado", () => {
  it("es verdadero solo para un 404 del backend", () => {
    expect(esNoEncontrado(new ApiError("no existe", 404))).toBe(true);
  });

  it("es falso para otros estados", () => {
    for (const status of [400, 409, 500, 503]) {
      expect(esNoEncontrado(new ApiError("otro", status))).toBe(false);
    }
  });

  // Un fallo de red lanza un TypeError, no un ApiError: no debe confundirse
  // con "borrado" ni cerrar la puerta a reintentar.
  it("es falso para un error que no vino del backend", () => {
    expect(esNoEncontrado(new TypeError("Failed to fetch"))).toBe(false);
    expect(esNoEncontrado(new Error("cualquier cosa"))).toBe(false);
    expect(esNoEncontrado(undefined)).toBe(false);
  });
});
