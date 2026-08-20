import { api } from "./client";
import type { MCC, MCCRiskLevel, MCCNetwork } from "../types";

export const mccApi = {
  list: () => api.get<MCC[]>("/mccs"),
  search: (q: string) => api.get<MCC[]>(`/mccs/search?q=${encodeURIComponent(q)}`),
  categories: () => api.get<string[]>("/mccs/categories"),
  byCategory: (cat: string) => api.get<MCC[]>(`/mccs/category/${encodeURIComponent(cat)}`),
  byRiskLevel: (level: MCCRiskLevel) => api.get<MCC[]>(`/mccs/risk/${level}`),
  update: (id: string, data: { riskLevel?: MCCRiskLevel; networks?: MCCNetwork[]; description?: string }) =>
    api.put<{ message: string }>(`/mccs/${id}`, data),
};
