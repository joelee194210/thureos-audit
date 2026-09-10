import type { LucideIcon } from "lucide-react";

/** Los grupos del índice, en el orden en que se muestran — los cuatro del
 *  menú superior más uno para los conceptos que atraviesan varias pantallas. */
export const HELP_GROUPS = [
  "Operación",
  "Monitoreo",
  "Catálogos",
  "Administración",
  "Conceptos",
] as const;

export type HelpGroup = (typeof HELP_GROUPS)[number];

/**
 * Una sección tiene párrafo O lista, nunca las dos. La unión lo vuelve
 * imposible por tipos: es el defecto que aparece cuando el contenido lo
 * escriben varias manos y una sección termina con un párrafo suelto arriba
 * de una lista que dice lo mismo.
 */
export type HelpSection =
  | { heading: string; body: string }
  | { heading: string; items: string[] };

export interface HelpArticle {
  /** Segmento de la URL: /ayuda/<slug>. Único. */
  slug: string;
  title: string;
  /** El mismo ícono con el que esa pantalla aparece en el menú superior. */
  icon: LucideIcon;
  group: HelpGroup;
  /** Ruta de la pantalla que documenta, o null para las fichas de concepto.
   *  El test la usa para comprobar que ninguna pantalla del menú quedó sin
   *  documentar. */
  route: string | null;
  /** En español llano, no un código de permiso: "Cualquier usuario con
   *  sesión iniciada", "Solo administradores". */
  quienPuede: string;
  /** Una o dos líneas. Se muestra en la tarjeta del índice y bajo el título
   *  de la ficha, así que se escribe corto en vez de truncarse. */
  summary: string;
  /** Archivos dentro de public/ayuda/. Vacío es válido. */
  images: string[];
  sections: HelpSection[];
}
