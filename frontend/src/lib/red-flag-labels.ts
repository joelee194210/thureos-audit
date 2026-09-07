/**
 * Etiquetas en español de los enumerados de bandera roja. Viven fuera de la
 * página porque el informe PDF (`red-flag-report.ts`) muestra los mismos
 * valores: si las etiquetas se duplicaran, el informe y la pantalla podrían
 * decir cosas distintas del mismo registro.
 */
import type { RedFlagStatus, ActionCategory, Severity } from "@/lib/types";

export const RED_FLAG_STATUS_LABELS: Record<RedFlagStatus, string> = {
  new: "Nueva",
  acknowledged: "En investigación",
  escalated: "Escalada",
  resolved: "Resuelta",
  dismissed: "Descartada",
};

export const ACTION_CATEGORY_LABELS: Record<ActionCategory | string, string> = {
  investigation: "Investigación",
  false_positive: "Falso positivo",
  escalation: "Escalado",
  corrective_action: "Acción correctiva",
  other: "Otro",
};

export const SEVERITY_LABELS: Record<Severity, string> = {
  critical: "Crítica",
  high: "Alta",
  medium: "Media",
  low: "Baja",
};
