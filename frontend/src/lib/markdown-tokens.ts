/**
 * Capa entre marked y el renderizador de React.
 *
 * Se usa ÚNICAMENTE el lexer de marked: nunca marked.parse() ni nada que
 * produzca HTML. El árbol de tokens se mapea a elementos de React en
 * components/chat/markdown.tsx, así que no existe un punto del camino en
 * el que un string del LLM se convierta en marcado.
 *
 * Motivo: el texto del asistente está influido por archivos que suben los
 * usuarios, y una inyección de prompt puede moldear lo que escribe.
 * Renderizar eso como HTML en el origen de la app —donde vive el JWT— es
 * lo que el iframe sandboxeado de los artefactos custom ya evitaba.
 */
import { Lexer, type Token } from "marked";

export type { Token };

/** Esquemas de URL que se permiten en un enlace. */
const SAFE_SCHEMES = ["http:", "https:", "mailto:"];

/**
 * safeHref devuelve la URL si es segura de poner en un href, o null si
 * no. Un null significa "renderizá el texto sin enlace", nunca "omití el
 * contenido".
 *
 * Se quitan los espacios en blanco INTERNOS antes de mirar el esquema:
 * los navegadores ignoran tabs y saltos de línea dentro del esquema, así
 * que "java\tscript:" es ejecutable y tiene que caer acá.
 */
export function safeHref(href: string): string | null {
  // Se descarta todo lo que sea <= 0x20 (espacios y caracteres de
  // control) filtrando por code point, y no con una clase de regex, para
  // no tener que incrustar caracteres de control en el archivo fuente.
  const collapsed = Array.from(href)
    .filter((ch) => ch.charCodeAt(0) > 0x20)
    .join("")
    .toLowerCase();

  // Una ruta relativa o un ancla no llevan esquema y son seguros.
  if (collapsed.startsWith("/") || collapsed.startsWith("#")) {
    return href.trim();
  }
  const colon = collapsed.indexOf(":");
  if (colon === -1) {
    // Sin esquema: relativo.
    return href.trim();
  }
  const scheme = collapsed.slice(0, colon + 1);
  return SAFE_SCHEMES.includes(scheme) ? href.trim() : null;
}

/** Tokeniza markdown. Nunca produce HTML. */
export function lexMarkdown(source: string): Token[] {
  if (!source) return [];
  return new Lexer({ gfm: true, breaks: true }).lex(source);
}
