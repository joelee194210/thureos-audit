import { api } from "./client";

export interface NormalizedMatch {
  name: string;
  entityType: string;
  sourceList: string;
  score: number;
  programs?: string[];
}

export type ScreeningStatus =
  | "clear"
  | "match"
  | "review"
  | "dismissed"
  | "false_positive";

export interface ScreeningResult {
  id: string;
  query: string;
  redFlagId?: string;
  field?: string;
  status: ScreeningStatus;
  strongestMatch?: string;
  matches?: NormalizedMatch[];
  reviewedBy?: string;
  reviewedAt?: string;
  reviewNotes?: string;
  whitelistExpiresAt?: string;
  createdAt: string;
}

export const screeningApi = {
  search: (name: string, dateOfBirth?: string) =>
    api.post<ScreeningResult>("/screening/search", { name, dateOfBirth }),

  getByRedFlag: (redFlagId: string) =>
    api.get<ScreeningResult[]>(`/red-flags/${redFlagId}/screening`),

  dismiss: (
    id: string,
    data: { status: "dismissed" | "false_positive"; notes: string },
  ) => api.post<{ message: string }>(`/screening/${id}/dismiss`, data),

  getConfig: () => api.get<{ watchmanUrl: string }>("/settings/screening"),

  updateConfig: (watchmanUrl: string) =>
    api.put<{ message: string }>("/settings/screening", { watchmanUrl }),
};
