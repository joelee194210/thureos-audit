import { api } from "./client";
import type { ActivityLogEntry } from "@/lib/types";

export const activityLogsApi = {
  list: (params?: { userId?: string; action?: string; limit?: number }) => {
    const searchParams = new URLSearchParams();
    if (params?.userId) searchParams.set("userId", params.userId);
    if (params?.action) searchParams.set("action", params.action);
    if (params?.limit) searchParams.set("limit", String(params.limit));
    const qs = searchParams.toString();
    return api.get<ActivityLogEntry[]>(`/activity-logs${qs ? `?${qs}` : ""}`);
  },
};
