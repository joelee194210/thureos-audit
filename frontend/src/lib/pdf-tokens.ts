/**
 * Resuelve tokens de color de la línea gráfica a tripletes RGB, que es lo
 * único que entiende jsPDF.
 *
 * Dos razones para leerlos del DOM en vez de escribir los hex en este archivo:
 * los tokens son la fuente de verdad (un literal aquí quedaría desfasado en
 * silencio la próxima vez que cambien) y ningún módulo de la aplicación
 * escribe un color literal.
 *
 * Siempre se lee la paleta clara, sin importar el tema activo: el PDF se ve
 * sobre papel blanco, y en oscuro `--fg-default` es casi blanco — el informe
 * saldría ilegible. Los propios tokens describen el tema claro como el de
 * «reportes exportables e impresión».
 */

export type RGB = [number, number, number];

/** Gris carbón: solo se usa si el token no resuelve (SSR, jsdom, tema roto). */
const FALLBACK: RGB = [26, 26, 26];

/** Exportado solo para tests: es pura y no toca el DOM, a diferencia de `readReportColors`. */
export function parseColor(value: string): RGB | null {
  const v = value.trim();

  if (v.startsWith("#")) {
    const hex = v.slice(1);
    const full = hex.length === 3 ? hex.replace(/./g, (c) => c + c) : hex;
    if (full.length < 6) return null;
    const n = parseInt(full.slice(0, 6), 16);
    return Number.isNaN(n) ? null : [(n >> 16) & 255, (n >> 8) & 255, n & 255];
  }

  // rgb()/rgba(), en cualquiera de las dos sintaxis (coma o espacio). El alfa
  // se descarta: en el PDF todo se dibuja opaco sobre blanco.
  const m = v.match(/rgba?\(\s*([\d.]+)[\s,]+([\d.]+)[\s,]+([\d.]+)/i);
  if (!m) return null;
  return [Math.round(+m[1]), Math.round(+m[2]), Math.round(+m[3])];
}

/**
 * Recibe un mapa `nombre lógico -> variable CSS` y devuelve el mismo mapa con
 * los colores ya resueltos a RGB.
 */
export function readReportColors<K extends string>(
  tokens: Record<K, string>,
): Record<K, RGB> {
  const keys = Object.keys(tokens) as K[];
  const resolved = {} as Record<K, RGB>;

  if (typeof document === "undefined") {
    for (const k of keys) resolved[k] = FALLBACK;
    return resolved;
  }

  // Un contenedor con data-theme="light" redefine las variables para su
  // subárbol; la marca se hereda de <html> para no cambiar de producto.
  const probe = document.createElement("div");
  probe.setAttribute("data-theme", "light");
  const brand = document.documentElement.getAttribute("data-brand");
  if (brand) probe.setAttribute("data-brand", brand);
  probe.style.cssText =
    "position:absolute;left:-9999px;top:0;width:0;height:0;pointer-events:none";
  document.body.appendChild(probe);

  try {
    const styles = getComputedStyle(probe);
    for (const k of keys) {
      resolved[k] = parseColor(styles.getPropertyValue(tokens[k])) ?? FALLBACK;
    }
  } finally {
    probe.remove();
  }

  return resolved;
}
