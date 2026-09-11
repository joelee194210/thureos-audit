import { api } from "./client";
import type {
  Monitor,
  SchemaField,
  SourceType,
  CreateSourceConfig,
  DerivedTimestampConfig,
  UploadCheckResult,
} from "@/lib/types";

export const monitorsApi = {
  list: () => api.get<Monitor[]>("/monitors"),

  get: (id: string) => api.get<Monitor>(`/monitors/${id}`),

  create: (data: {
    name: string;
    description: string;
    sourceType: SourceType;
    sourceConfig?: CreateSourceConfig;
    schema?: SchemaField[];
  }) =>
    api.post<Monitor | { monitor: Monitor; pushToken: string }>(
      "/monitors",
      data,
    ),

  update: (
    id: string,
    data: {
      name?: string;
      description?: string;
      sourceConfig?: CreateSourceConfig;
    },
  ) => api.put<{ message: string }>(`/monitors/${id}`, data),

  updateSchema: (id: string, schema: SchemaField[], rescaleExisting = false) =>
    api.put<{
      schema: SchemaField[];
      rescaled?: Record<string, number>;
      omitidos?: Record<string, number>;
    }>(`/monitors/${id}/schema`, { schema, rescaleExisting }),

  /**
   * Configura el timestamp derivado, o lo limpia mandando null. La clave
   * viaja siempre: el backend distingue "no la mandaste" (error) de "la
   * mandaste en null" (limpiar), para no borrar la configuración por una
   * request incompleta.
   */
  updateDerivedTimestamp: (id: string, cfg: DerivedTimestampConfig | null) =>
    api.put<Monitor>(`/monitors/${id}/derived-timestamp`, {
      derivedTimestamp: cfg,
    }),

  /** Calcula el timestamp derivado para los datos ya ingeridos. Idempotente. */
  backfillTimestamp: (id: string) =>
    api.post<{ updated: number; skipped: number }>(
      `/monitors/${id}/backfill-timestamp`,
      {},
    ),

  detectSchema: (
    sourceType: string,
    sourceConfig: Partial<CreateSourceConfig>,
  ) =>
    api.post<{ schema: SchemaField[]; sampleCount: number }>(
      "/monitors/detect-schema",
      {
        sourceType,
        sourceConfig,
      },
    ),

  rotatePushToken: (id: string) =>
    api.post<{ pushToken: string }>(`/monitors/${id}/rotate-push-token`, {}),

  /** Borrado lógico: el monitor se oculta, pero sus datos, reglas y
   *  banderas rojas quedan intactos y se puede restaurar. */
  delete: (id: string) => api.delete<{ message: string }>(`/monitors/${id}`),

  /** Los monitores borrados, para poder restaurarlos. */
  listDeleted: () => api.get<Monitor[]>("/monitors/deleted"),

  restore: (id: string) =>
    api.post<{ message: string }>(`/monitors/${id}/restore`, {}),

  upload: (id: string, file: File) =>
    api.upload<{
      recordsIngested: number;
      schema: SchemaField[];
      evaluationQueued: boolean;
    }>(`/monitors/${id}/upload`, file),

  uploadCheck: (id: string, file: File) =>
    api.upload<UploadCheckResult>(`/monitors/${id}/upload/check`, file),

  getData: (id: string) =>
    api.get<{
      data: Record<string, unknown>[];
      total: number;
      schema: SchemaField[];
    }>(`/monitors/${id}/data`),

  evaluate: (id: string) =>
    api.post<{
      redFlagsGenerated: number;
      redFlags: unknown[];
    }>(`/monitors/${id}/evaluate`, {}),
};
