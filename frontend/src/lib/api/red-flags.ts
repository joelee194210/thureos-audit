import { api } from "./client";
import type { RedFlag, RedFlagStatus, RedFlagLog, ActionCategory } from "@/lib/types";

export interface RedFlagRecordsResponse {
  records: Record<string, unknown>[];
  total: number;
  page: number;
  limit: number;
  totalPages: number;
}

export interface CalendarDay {
  _id: string; // YYYY-MM-DD
  total: number;
  critical: number;
  high: number;
  new: number;
}

export const redFlagsApi = {
  list: (params?: { monitorId?: string; limit?: number }) => {
    const searchParams = new URLSearchParams();
    if (params?.monitorId) searchParams.set("monitorId", params.monitorId);
    if (params?.limit) searchParams.set("limit", String(params.limit));
    const qs = searchParams.toString();
    return api.get<RedFlag[]>(`/red-flags${qs ? `?${qs}` : ""}`);
  },

  get: (id: string) => api.get<RedFlag>(`/red-flags/${id}`),

  updateStatus: (id: string, status: RedFlagStatus, category: ActionCategory, notes: string) =>
    api.patch<{ message: string }>(`/red-flags/${id}/status`, { status, category, notes }),

  getLogs: (id: string) =>
    api.get<RedFlagLog[]>(`/red-flags/${id}/logs`),

  stats: () =>
    api.get<{ new: number; acknowledged: number }>("/red-flags/stats"),

  records: (id: string, page = 1, limit = 50) =>
    api.get<RedFlagRecordsResponse>(`/red-flags/${id}/records?page=${page}&limit=${limit}`),

  calendar: (year: number, month: number) =>
    api.get<CalendarDay[]>(`/red-flags/calendar?year=${year}&month=${month}`),

  byDate: (date: string) =>
    api.get<RedFlag[]>(`/red-flags/date/${date}`),
};
