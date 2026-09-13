/**
 * Cliente de artefactos del chat: biblioteca guardada, re-ejecución contra
 * datos actuales y export a Excel.
 */
import { api } from "./client";
import type { ChatArtifact } from "./chat";
import { unionColumns } from "@/lib/format-value";

export interface ArtifactRun {
  data: Record<string, unknown>[];
  /**
   * Claves a graficar/tabular en `data`. Con varias fuentes el backend
   * pivotea y renombra las columnas con el label de cada fuente, así que
   * `chartSpec.yKeys` deja de apuntar a nada: hay que usar SIEMPRE esto.
   */
  series: string[];
  ranAt: string;
}

/**
 * Un artefacto es re-ejecutable si declara las consultas que lo alimentan.
 * Sin `sources` es una instantánea: legacy o custom.
 */
export function isRerunnable(artifact: ChatArtifact): boolean {
  return (artifact.sources?.length ?? 0) > 0;
}

/**
 * Serie a graficar mientras no hay una corrida fresca todavía. El backend
 * NO persiste `series` junto con `cachedData` — solo lo devuelve al vuelo
 * en `/run` — así que al abrir un artefacto que ya corrió antes no hay
 * forma de recuperar la lista real que usó esa corrida. `chartSpec.yKeys`
 * tampoco sirve: son las claves de la ejecución ORIGINAL, y dejan de
 * apuntar a nada en cuanto el backend pivotea con más de una fuente (ver
 * Global Constraints). El mejor dato disponible es derivarla de las
 * columnas que sí trae `cachedData`, descartando la del eje X.
 */
export function deriveSeriesFromRows(
  rows: Record<string, unknown>[],
  xKey: string | undefined,
): string[] {
  return unionColumns(rows).filter((key) => key !== xKey);
}

export const artifactsApi = {
  list: () => api.get<ChatArtifact[]>("/chat/artifacts"),

  save: (id: string, name: string) =>
    api.post<ChatArtifact>(`/chat/artifacts/${id}/save`, { name }),

  unsave: (id: string) =>
    api.delete<{ message: string }>(`/chat/artifacts/${id}/save`),

  /**
   * Vuelve a correr las `sources` del artefacto contra la data actual.
   * 409 = el monitor de origen ya no existe: quien llama debe caer al modo
   * degradado (mostrar `cachedData` con un aviso), no tratarlo como fallo.
   */
  run: (id: string) => api.post<ArtifactRun>(`/chat/artifacts/${id}/run`, {}),

  remove: (id: string) =>
    api.delete<{ message: string }>(`/chat/artifacts/${id}`),

  /**
   * Blob del Excel. `client.ts` manda el JWT por header `Authorization`,
   * así que una URL abierta con `window.open`/`<a href>` nunca lo llevaría
   * (el request saldría sin sesión) — por eso esto es un `fetch` autenticado
   * en vez de exponer la URL cruda. Igual que `run`, un 409 significa
   * "monitor de origen ausente"; `api.getBlob` lo propaga como `ApiError`
   * con `status`, `null` es reservado para 404 (no existe o no es tuyo).
   */
  exportXlsx: (id: string) =>
    api.getBlob(`/chat/artifacts/${id}/export.xlsx`),
};
