export const SEGMENT_LABELS: Record<string, string> = {
  monitors: "Monitores",
  "red-flags": "Banderas Rojas",
  dashboards: "Dashboards",
  rules: "Reglas",
  screening: "Screening",
  chatbot: "Analista IA",
  settings: "Configuración",
  users: "Usuarios",
  countries: "Países",
  mcc: "MCC",
  uploads: "Cargas",
  "activity-logs": "Bitácora de Acceso",
  ayuda: "Ayuda",
};

function titleCase(segment: string): string {
  return segment
    .split("-")
    .map((word) => (word ? word.charAt(0).toUpperCase() + word.slice(1) : word))
    .join(" ");
}

export function labelForPath(path: string): string {
  const last = path.split("/").filter(Boolean).pop();
  if (!last) return "Panel";
  return SEGMENT_LABELS[last] || titleCase(last);
}

export function resolveTabLabel(
  tab: { path: string; label: string },
  entityLabels: Record<string, string>,
): string {
  const segments = tab.path.split("/").filter(Boolean);
  for (let i = segments.length - 1; i >= 0; i--) {
    const segment = segments[i];
    if (entityLabels[segment]) return entityLabels[segment];
  }
  return tab.label;
}
