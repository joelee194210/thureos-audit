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

/**
 * Filas a pintar ANTES de que llegue una corrida fresca: la caché de una
 * corrida real anterior, o los datos embebidos de una instantánea legacy
 * (sin `sources`, nunca se re-ejecuta). Vive acá y no en el componente
 * porque decide cuál de las dos ramas de `deriveSeriesFromRows`/`yKeys`
 * corresponde — invertir esa rama en silencio es exactamente el fallo que
 * esta función existe para no reproducir, así que se prueba en `lib/`
 * como el resto de las funciones puras del repo.
 */
export function resolveInitialRows(
  artifact: ChatArtifact,
): Record<string, unknown>[] {
  return artifact.cachedData ?? artifact.chartSpec?.data ?? [];
}

/**
 * Serie a graficar ANTES de que llegue una corrida fresca (ver
 * `resolveInitialRows`). Con `cachedData` presente, el artefacto tiene
 * `sources` y ya corrió antes: `chartSpec.yKeys` es de la ejecución
 * ORIGINAL y puede no describir esas columnas (pivote multi-fuente), así
 * que se deriva de `cachedData` mismo. Sin `cachedData`, es una
 * instantánea legacy sin `sources` — ahí `chartSpec.data` es la única
 * ejecución que existió y `yKeys` sí la describe correctamente.
 */
export function resolveInitialSeries(artifact: ChatArtifact): string[] {
  if (artifact.cachedData) {
    return deriveSeriesFromRows(artifact.cachedData, artifact.chartSpec?.xKey);
  }
  return artifact.chartSpec?.yKeys ?? [];
}

/**
 * Serie a graficar a partir de una corrida fresca de `/run`. `run.series`
 * es la fuente de verdad casi siempre — pero `ParseArtifact` en el backend
 * solo exige `yKeys` no vacías cuando el artefacto declara MÁS DE UNA
 * fuente; con una sola, el LLM puede omitirlas legítimamente, y entonces
 * `run.series` llega vacío con filas reales. Sin este respaldo, el
 * gráfico dibujaría los ejes sin ninguna barra/línea — el mismo fallo
 * silencioso que la restricción "usar siempre `run.series`" existe para
 * evitar, alcanzado por el camino hermano. Nunca cae a `chartSpec.yKeys`:
 * ese es justamente el dato que puede no aplicar más.
 */
export function resolveRunSeries(
  run: ArtifactRun,
  xKey: string | undefined,
): string[] {
  return run.series.length > 0 ? run.series : deriveSeriesFromRows(run.data, xKey);
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
