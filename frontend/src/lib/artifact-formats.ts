/**
 * Qué formatos de exportación ofrece cada tipo de artefacto. Único lugar
 * donde vive esta tabla — `export-menu.tsx` solo la consulta, nunca la
 * repite en el JSX.
 *
 * `custom` es código generado por el LLM, no datos tabulares: no hay PDF
 * (el módulo de informes lanza para este tipo, ver `artifact-report.ts`)
 * ni Excel (el backend no tiene datos que volcar). La limitación se dice
 * en el menú (`unavailableFormatsNotice`), no se esconde.
 */
import type { ChatArtifact } from "@/lib/api/chat";

export type ExportFormat = "png" | "csv" | "xlsx" | "pdf" | "html" | "print";

export interface FormatOption {
  format: ExportFormat;
  label: string;
}

const CHART_FORMATS: FormatOption[] = [
  { format: "png", label: "PNG" },
  { format: "csv", label: "CSV" },
  { format: "xlsx", label: "Excel" },
  { format: "pdf", label: "PDF" },
  { format: "html", label: "HTML" },
];

const TABLE_FORMATS: FormatOption[] = [
  { format: "csv", label: "CSV" },
  { format: "xlsx", label: "Excel" },
  { format: "pdf", label: "PDF" },
  { format: "html", label: "HTML" },
];

const CUSTOM_FORMATS: FormatOption[] = [
  { format: "html", label: "HTML" },
  { format: "print", label: "Imprimir" },
];

/** Formatos que el menú de exportación ofrece para este artefacto. */
export function availableFormats(artifact: ChatArtifact): FormatOption[] {
  switch (artifact.type) {
    case "chart":
      return CHART_FORMATS;
    case "table":
      return TABLE_FORMATS;
    case "custom":
      return CUSTOM_FORMATS;
    default:
      return [];
  }
}

/**
 * Por qué un artefacto `custom` no ofrece PDF ni Excel — se dice, no se
 * esconde. `null` para tipos sin limitación que explicar.
 */
export function unavailableFormatsNotice(artifact: ChatArtifact): string | null {
  if (artifact.type === "custom") {
    return "Un artefacto interactivo no se puede convertir a PDF ni a Excel: es código, no datos tabulares.";
  }
  return null;
}
