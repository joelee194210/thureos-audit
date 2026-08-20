import { api } from "./client";
import type { Country } from "../types";

export const countriesApi = {
  list: () => api.get<Country[]>("/countries"),
  active: () => api.get<Country[]>("/countries/active"),
  search: (q: string) => api.get<Country[]>(`/countries/search?q=${encodeURIComponent(q)}`),
  regions: () => api.get<string[]>("/countries/regions"),
  byRiskLevel: (level: string) => api.get<Country[]>(`/countries/risk/${level}`),
  byRegion: (region: string) => api.get<Country[]>(`/countries/region/${encodeURIComponent(region)}`),
  get: (id: string) => api.get<Country>(`/countries/${id}`),
  create: (data: Partial<Country>) => api.post<Country>("/countries", data),
  update: (id: string, data: Partial<Country>) => api.put<Country>(`/countries/${id}`, data),
  delete: (id: string) => api.delete<{ message: string }>(`/countries/${id}`),
};
