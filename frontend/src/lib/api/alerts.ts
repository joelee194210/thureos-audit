import { api } from "./client";
import type { Alert, AlertStatus, AlertLog, ActionCategory } from "@/lib/types";

export interface AlertRecordsResponse {
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

export const alertsApi = {
  list: (params?: { monitorId?: string; limit?: number }) => {
    const searchParams = new URLSearchParams();
    if (params?.monitorId) searchParams.set("monitorId", params.monitorId);
    if (params?.limit) searchParams.set("limit", String(params.limit));
    const qs = searchParams.toString();
    return api.get<Alert[]>(`/alerts${qs ? `?${qs}` : ""}`);
  },

  get: (id: string) => api.get<Alert>(`/alerts/${id}`),

  updateStatus: (id: string, status: AlertStatus, category: ActionCategory, notes: string) =>
    api.patch<{ message: string }>(`/alerts/${id}/status`, { status, category, notes }),

  getLogs: (id: string) =>
    api.get<AlertLog[]>(`/alerts/${id}/logs`),

  stats: () =>
    api.get<{ new: number; acknowledged: number }>("/alerts/stats"),

  records: (id: string, page = 1, limit = 50) =>
    api.get<AlertRecordsResponse>(`/alerts/${id}/records?page=${page}&limit=${limit}`),

  calendar: (year: number, month: number) =>
    api.get<CalendarDay[]>(`/alerts/calendar?year=${year}&month=${month}`),

  byDate: (date: string) =>
    api.get<Alert[]>(`/alerts/date/${date}`),
};
