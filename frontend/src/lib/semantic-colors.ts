/**
 * Mapas de clase por eje semántico. Ningún componente escribe un color
 * literal: importa de aquí.
 *
 * Tres ejes distintos, que no se mezclan:
 *  - RISK_CLASSES     severidad y riesgo. Rojo es crítico en toda la
 *                     plataforma, independientemente de la marca activa.
 *  - STATUS_CLASSES   resultado de una operación: éxito, error, aviso.
 *  - CATEGORY_CLASSES distinción sin orden: tipo de fuente, tipo de acción.
 *                     Usa las series de gráfico, verificadas para daltonismo.
 */

export type RiskLevel = "none" | "low" | "medium" | "high" | "critical";

export const RISK_CLASSES: Record<RiskLevel, string> = {
  none: "bg-risk-none-bg text-risk-none-fg",
  low: "bg-risk-low-bg text-risk-low-fg",
  medium: "bg-risk-medium-bg text-risk-medium-fg",
  high: "bg-risk-high-bg text-risk-high-fg",
  critical: "bg-risk-critical-bg text-risk-critical-fg",
};

/** Solo el color de texto, para iconos sobre fondo transparente. */
export const RISK_FG: Record<RiskLevel, string> = {
  none: "text-risk-none-fg",
  low: "text-risk-low-fg",
  medium: "text-risk-medium-fg",
  high: "text-risk-high-fg",
  critical: "text-risk-critical-fg",
};

/** Solo el fondo suave, para cuadros de icono en tarjetas de estadísticas. */
export const RISK_BG: Record<RiskLevel, string> = {
  none: "bg-risk-none-bg",
  low: "bg-risk-low-bg",
  medium: "bg-risk-medium-bg",
  high: "bg-risk-high-bg",
  critical: "bg-risk-critical-bg",
};

/** Borde izquierdo para tarjetas de bandera roja. */
export const RISK_BORDER_L: Record<RiskLevel, string> = {
  none: "border-l-risk-none-fg",
  low: "border-l-risk-low-fg",
  medium: "border-l-risk-medium-fg",
  high: "border-l-risk-high-fg",
  critical: "border-l-risk-critical-fg",
};

export type StatusKind = "success" | "warning" | "danger" | "info" | "neutral";

export const STATUS_CLASSES: Record<StatusKind, string> = {
  success: "bg-success-bg text-success-fg border-success-border",
  warning: "bg-warning-bg text-warning-fg border-warning-border",
  danger: "bg-danger-bg text-danger-fg border-danger-border",
  info: "bg-info-bg text-info-fg border-info-border",
  neutral: "bg-surface-2 text-ink-muted border-line",
};

export const STATUS_FG: Record<StatusKind, string> = {
  success: "text-success-fg",
  warning: "text-warning-fg",
  danger: "text-danger-fg",
  info: "text-info-fg",
  neutral: "text-ink-muted",
};

/**
 * Ejes categóricos: tipo de fuente, tipo de acción. Sin orden ni severidad.
 * Pintarlos con la escala de riesgo sugeriría que una carga por API es más
 * peligrosa que una por CSV, que es falso.
 */
export const CATEGORY_CLASSES: string[] = [
  "bg-chart-1/15 text-chart-1",
  "bg-chart-2/15 text-chart-2",
  "bg-chart-3/15 text-chart-3",
  "bg-chart-4/15 text-chart-4",
  "bg-chart-5/15 text-chart-5",
  "bg-chart-6/15 text-chart-6",
  "bg-chart-7/15 text-chart-7",
  "bg-chart-8/15 text-chart-8",
];
