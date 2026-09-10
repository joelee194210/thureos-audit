import { api } from "./client";
import type { MCC, MCCRiskLevel, MCCNetwork } from "../types";

export const mccApi = {
  list: () => api.getList<MCC>("/mccs"),
  search: (q: string) =>
    api.getList<MCC>(`/mccs/search?q=${encodeURIComponent(q)}`),
  categories: () => api.getList<string>("/mccs/categories"),
  byCategory: (cat: string) =>
    api.getList<MCC>(`/mccs/category/${encodeURIComponent(cat)}`),
  byRiskLevel: (level: MCCRiskLevel) => api.getList<MCC>(`/mccs/risk/${level}`),
  update: (
    id: string,
    data: {
      riskLevel?: MCCRiskLevel;
      networks?: MCCNetwork[];
      description?: string;
    },
  ) => api.put<{ message: string }>(`/mccs/${id}`, data),
};
