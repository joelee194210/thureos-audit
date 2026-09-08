export interface RegistryEntry {
  pattern: string;
  Component: React.ComponentType<{
    params: Record<string, string>;
    tabId: string;
  }>;
}

/** Segmentos que nunca se tratan como id dinámico, aunque el patrón de la
 * ruta sea `[algo]` — evita que una ruta estática (ej. "/monitors/new")
 * se confunda con un id real. */
export const RESERVED_DYNAMIC_SEGMENTS = new Set(["new"]);

export function matchRegistryEntry(
  path: string,
  registry: RegistryEntry[],
): { entry: RegistryEntry; params: Record<string, string> } | null {
  const pathSegments = path.split("/").filter(Boolean);

  for (const entry of registry) {
    const patternSegments = entry.pattern.split("/").filter(Boolean);
    if (patternSegments.length !== pathSegments.length) continue;

    const params: Record<string, string> = {};
    let matched = true;

    for (let i = 0; i < patternSegments.length; i++) {
      const patternSegment = patternSegments[i];
      const pathSegment = pathSegments[i];

      if (patternSegment.startsWith("[") && patternSegment.endsWith("]")) {
        if (RESERVED_DYNAMIC_SEGMENTS.has(pathSegment)) {
          matched = false;
          break;
        }
        params[patternSegment.slice(1, -1)] = pathSegment;
      } else if (patternSegment !== pathSegment) {
        matched = false;
        break;
      }
    }

    if (matched) return { entry, params };
  }

  return null;
}

import { MonitorTabContent } from "@/components/tabs/content/monitor-tab-content";
import { RedFlagTabContent } from "@/components/tabs/content/red-flag-tab-content";
import { DashboardTabContent } from "@/components/tabs/content/dashboard-tab-content";

export const TAB_CONTENT_REGISTRY: RegistryEntry[] = [
  { pattern: "/monitors/[id]", Component: MonitorTabContent },
  { pattern: "/red-flags/[id]", Component: RedFlagTabContent },
  { pattern: "/dashboards/[id]", Component: DashboardTabContent },
];
