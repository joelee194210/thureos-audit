import { api } from "./client";
import type {
  Monitor,
  SchemaField,
  SourceType,
  CreateSourceConfig,
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

  delete: (id: string) => api.delete<{ message: string }>(`/monitors/${id}`),

  upload: (id: string, file: File) =>
    api.upload<{
      recordsIngested: number;
      schema: SchemaField[];
      evaluationQueued: boolean;
    }>(`/monitors/${id}/upload`, file),

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
