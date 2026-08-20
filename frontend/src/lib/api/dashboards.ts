import { api } from "./client";
import type { Dashboard, Widget, WidgetType, AggregationType } from "@/lib/types";

export const dashboardsApi = {
  list: () => api.get<Dashboard[]>("/dashboards"),

  get: (id: string) => api.get<Dashboard>(`/dashboards/${id}`),

  create: (data: { name: string; description: string; monitorIds: string[] }) =>
    api.post<Dashboard>("/dashboards", data),

  addWidget: (
    dashboardId: string,
    widget: {
      title: string;
      type: WidgetType;
      monitorId?: string;
      field?: string;
      aggregation?: AggregationType;
      groupBy?: string;
      ruleId?: string;
      position: { x: number; y: number; w: number; h: number };
    }
  ) => api.post<Widget>(`/dashboards/${dashboardId}/widgets`, widget),

  getData: (id: string) =>
    api.get<{
      widgets: {
        widgetId: string;
        title: string;
        type: string;
        data: Record<string, unknown>[];
      }[];
    }>(`/dashboards/${id}/data`),

  drilldown: (
    dashboardId: string,
    widgetId: string,
    body: { groupValue: string; page: number; pageSize: number; search: string; export?: boolean }
  ) =>
    api.post<{ records: Record<string, unknown>[]; total: number; page: number }>(
      `/dashboards/${dashboardId}/widgets/${widgetId}/drilldown`,
      body
    ),

  deleteWidget: (dashboardId: string, widgetId: string) =>
    api.delete<{ message: string }>(`/dashboards/${dashboardId}/widgets/${widgetId}`),

  delete: (id: string) => api.delete<{ message: string }>(`/dashboards/${id}`),
};
