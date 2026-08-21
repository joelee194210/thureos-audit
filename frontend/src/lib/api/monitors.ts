import { api } from "./client";
import type { Monitor, SchemaField, SourceType, CreateSourceConfig } from "@/lib/types";

export const monitorsApi = {
  list: () => api.get<Monitor[]>("/monitors"),

  get: (id: string) => api.get<Monitor>(`/monitors/${id}`),

  create: (data: { name: string; description: string; sourceType: SourceType; sourceConfig?: CreateSourceConfig }) =>
    api.post<Monitor | { monitor: Monitor; pushToken: string }>("/monitors", data),

  update: (id: string, data: Partial<Pick<Monitor, "name" | "description">>) =>
    api.put<{ message: string }>(`/monitors/${id}`, data),

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
