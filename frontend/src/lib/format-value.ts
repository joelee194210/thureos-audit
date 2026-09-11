/**
 * Formateo de un valor suelto proveniente de una colección dinámica.
 *
 * Vive acá y no en cada consumidor porque el informe PDF de banderas
 * rojas y la tabla del chat tienen que mostrar el mismo dato igual: un
 * monto que en el PDF sale "1.234.567" y en pantalla "1234567" es un
 * reporte de bug esperando a pasar.
 */
import { formatDate } from "@/lib/utils";

/** Fechas ISO que llegan como texto desde colecciones dinámicas. */
const ISO_DATE_RE = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}/;

export type ValueKind = "number" | "date" | "boolean" | "empty" | "text";

/**
 * classifyValue dice de qué clase es un valor, para que quien lo muestre
 * pueda decidir alineación y tipografía. Deliberadamente NO convierte:
 * un "42" guardado como string se clasifica como texto, porque
 * mostrarlo alineado a la derecha mentiría sobre el tipo del dato.
 */
export function classifyValue(val: unknown): ValueKind {
  if (val === null || val === undefined || val === "") return "empty";
  if (typeof val === "number") return "number";
  if (typeof val === "boolean") return "boolean";
  if (typeof val === "string" && ISO_DATE_RE.test(val)) {
    return Number.isNaN(new Date(val).getTime()) ? "text" : "date";
  }
  return "text";
}

export function formatValue(val: unknown): string {
  switch (classifyValue(val)) {
    case "empty":
      return "—";
    case "number":
      return new Intl.NumberFormat("es-CO").format(val as number);
    case "boolean":
      return val ? "Sí" : "No";
    case "date":
      return formatDate(val as string);
    default:
      if (typeof val === "object") return JSON.stringify(val);
      return String(val);
  }
}
