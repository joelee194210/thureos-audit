"use client";

import React, { useEffect, useState, useCallback } from "react";
import { Header } from "@/components/layout/header";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import {
  Plus,
  Trash2,
  TrendingUp,
  Search,
  ChevronLeft,
  ChevronRight,
  MousePointerClick,
  BarChart3,
  LineChart as LineChartIcon,
  PieChart as PieChartIcon,
  AreaChart as AreaChartIcon,
  Hash,
  Download,
  ChevronDown,
  ChevronUp,
  Loader2,
  AlertTriangle,
} from "lucide-react";
import {
  BarChart,
  Bar,
  LineChart,
  Line,
  PieChart,
  Pie,
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip as RechartsTooltip,
  ResponsiveContainer,
  Cell,
  Legend,
} from "recharts";
import { dashboardsApi } from "@/lib/api/dashboards";
import { monitorsApi } from "@/lib/api/monitors";
import { rulesApi } from "@/lib/api/rules";
import { useToast } from "@/lib/use-toast";
import { useTabStore } from "@/stores/tab-store";
import type {
  Dashboard,
  Monitor,
  Rule,
  WidgetType,
  AggregationType,
} from "@/lib/types";

// Professional color palette
// Series de los tokens Thureos: cambian con el tema y están verificadas
// para deuteranopía y protanopía.
const CHART_COLORS = [
  "var(--chart-1)",
  "var(--chart-2)",
  "var(--chart-3)",
  "var(--chart-4)",
  "var(--chart-5)",
  "var(--chart-6)",
  "var(--chart-7)",
  "var(--chart-8)",
];

const WIDGET_TYPE_ICONS: Record<string, React.ReactNode> = {
  bar_chart: <BarChart3 className="h-4 w-4" />,
  line_chart: <LineChartIcon className="h-4 w-4" />,
  pie_chart: <PieChartIcon className="h-4 w-4" />,
  area_chart: <AreaChartIcon className="h-4 w-4" />,
  stat: <Hash className="h-4 w-4" />,
};

const fmt = new Intl.NumberFormat("es", {
  maximumFractionDigits: 2,
  notation: "compact",
});

const fmtFull = new Intl.NumberFormat("es", { maximumFractionDigits: 2 });

// Mismo tope que aplica el backend en DrillDown cuando export=true
// (dashboard_service.go) — solo para avisar aquí, no para hacerlo cumplir.
const EXPORT_ALL_LIMIT = 10000;

function CustomTooltip({
  active,
  payload,
  label,
}: {
  active?: boolean;
  payload?: { color: string; value: number; name: string }[];
  label?: React.ReactNode;
}) {
  if (!active || !payload?.length) return null;
  return (
    <div className="rounded-lg border bg-popover px-3 py-2 shadow-xl">
      <p className="mb-1 text-xs font-medium text-muted-foreground">{label}</p>
      {payload.map(
        (entry: { color: string; value: number; name: string }, i: number) => (
          <p
            key={i}
            className="text-sm font-semibold"
            style={{ color: entry.color }}
          >
            {fmtFull.format(entry.value)}
          </p>
        ),
      )}
    </div>
  );
}

function PieTooltip({
  active,
  payload,
}: {
  active?: boolean;
  payload?: {
    name?: string;
    value: number;
    payload?: { total?: number; fill?: string };
  }[];
}) {
  if (!active || !payload?.length) return null;
  const entry = payload[0];
  const total = entry.payload?.total || 0;
  const pct = total > 0 ? ((entry.value / total) * 100).toFixed(1) : "0";
  return (
    <div className="rounded-lg border bg-popover px-3 py-2 shadow-xl">
      <p className="mb-1 text-xs font-medium text-muted-foreground">
        {entry.name}
      </p>
      <p
        className="text-sm font-semibold"
        style={{ color: entry.payload?.fill }}
      >
        {fmtFull.format(entry.value)} ({pct}%)
      </p>
    </div>
  );
}

interface DrilldownState {
  widgetId: string;
  widgetTitle: string;
  groupValue: string; // display label (may be "(Sin valor)")
  dbGroupValue: string; // actual value sent to the backend API
}

export function DashboardTabContent({
  params,
  tabId,
}: {
  params: Record<string, string>;
  tabId: string;
}) {
  const { id } = params;
  const [dashboard, setDashboard] = useState<Dashboard | null>(null);
  const [widgetData, setWidgetData] = useState<
    Record<string, Record<string, unknown>[]>
  >({});
  const [widgetErrors, setWidgetErrors] = useState<Record<string, string>>({});
  const [monitors, setMonitors] = useState<Monitor[]>([]);
  const [rules, setRules] = useState<Rule[]>([]);
  const [isAddOpen, setIsAddOpen] = useState(false);
  const [widgetMode, setWidgetMode] = useState<"manual" | "rule">("manual");
  const { toastError } = useToast();
  const updateTabLabel = useTabStore((s) => s.updateTabLabel);
  const registerEntityLabel = useTabStore((s) => s.registerEntityLabel);

  useEffect(() => {
    if (dashboard?.name) {
      updateTabLabel(tabId, dashboard.name);
      registerEntityLabel(id, dashboard.name);
    }
  }, [dashboard?.name, tabId, id, updateTabLabel, registerEntityLabel]);

  // Drill-down state
  const [drilldown, setDrilldown] = useState<DrilldownState | null>(null);
  const [drillRecords, setDrillRecords] = useState<Record<string, unknown>[]>(
    [],
  );
  const [drillTotal, setDrillTotal] = useState(0);
  const [drillPage, setDrillPage] = useState(1);
  const [drillSearch, setDrillSearch] = useState("");
  const [drillLoading, setDrillLoading] = useState(false);
  const [drillPageSize, setDrillPageSize] = useState(50);
  const [drillExporting, setDrillExporting] = useState(false);
  const [expandedRow, setExpandedRow] = useState<number | null>(null);

  // New widget form
  const [newWidget, setNewWidget] = useState({
    title: "",
    type: "bar_chart" as WidgetType,
    monitorId: "",
    field: "",
    aggregation: "count" as AggregationType,
    groupBy: "",
    ruleId: "",
  });

  useEffect(() => {
    if (id) {
      loadDashboard();
      loadData();
      monitorsApi
        .list()
        .then(setMonitors)
        .catch((err) => {
          console.error("Failed to load monitors:", err);
          toastError("Error al cargar monitores");
        });
      rulesApi
        .list()
        .then(setRules)
        .catch((err) => {
          console.error("Failed to load rules:", err);
        });
    }
  }, [id]);

  async function loadDashboard() {
    try {
      setDashboard(await dashboardsApi.get(id));
    } catch (err) {
      console.error("Failed to load dashboard:", err);
      toastError("Error al cargar dashboard");
    }
  }

  async function loadData() {
    try {
      const result = await dashboardsApi.getData(id);
      const dataMap: Record<string, Record<string, unknown>[]> = {};
      const errorMap: Record<string, string> = {};
      result.widgets?.forEach((w) => {
        dataMap[w.widgetId] = w.data;
        if (w.error) errorMap[w.widgetId] = w.error;
      });
      setWidgetData(dataMap);
      setWidgetErrors(errorMap);
    } catch (err) {
      console.error("Failed to load widget data:", err);
      toastError("Error al cargar datos del dashboard");
    }
  }

  async function addWidget(e: React.FormEvent) {
    e.preventDefault();
    try {
      const payload: Record<string, unknown> = {
        title: newWidget.title,
        type: newWidget.type,
        position: { x: 0, y: 0, w: 6, h: 4 },
      };
      if (widgetMode === "rule" && newWidget.ruleId) {
        payload.ruleId = newWidget.ruleId;
        // Find the rule to get its monitorId
        const rule = rules.find((r) => r.id === newWidget.ruleId);
        if (rule) {
          payload.monitorId = rule.monitorId;
          // Auto-configure from rule
          if (rule.aggregateConditions?.length) {
            const agg = rule.aggregateConditions[0];
            payload.field = agg.field;
            payload.aggregation = agg.function;
            payload.groupBy = agg.groupBy;
          } else if (rule.conditionGroup?.conditions?.length) {
            payload.field = rule.conditionGroup.conditions[0].field;
            payload.aggregation = "count";
          }
        }
      } else {
        payload.monitorId = newWidget.monitorId;
        payload.field = newWidget.field;
        payload.aggregation = newWidget.aggregation;
        payload.groupBy = newWidget.groupBy;
      }
      await dashboardsApi.addWidget(
        id,
        payload as Parameters<typeof dashboardsApi.addWidget>[1],
      );
      setIsAddOpen(false);
      setNewWidget({
        title: "",
        type: "bar_chart",
        monitorId: "",
        field: "",
        aggregation: "count",
        groupBy: "",
        ruleId: "",
      });
      setWidgetMode("manual");
      loadDashboard();
      loadData();
    } catch (err) {
      console.error("Failed to add widget:", err);
      toastError("Error al agregar widget");
    }
  }

  async function deleteWidget(widgetId: string) {
    try {
      await dashboardsApi.deleteWidget(id, widgetId);
      loadDashboard();
      loadData();
    } catch (err) {
      console.error("Failed to delete widget:", err);
      toastError("Error al eliminar widget");
    }
  }

  // Drill-down: load records for a clicked segment
  const loadDrilldown = useCallback(
    async (
      widgetId: string,
      groupValue: string,
      page: number,
      search: string,
      pageSize: number,
    ) => {
      setDrillLoading(true);
      try {
        const result = await dashboardsApi.drilldown(id, widgetId, {
          groupValue,
          page,
          pageSize,
          search,
        });
        setDrillRecords(result.records || []);
        setDrillTotal(result.total || 0);
      } catch (err) {
        console.error("Failed to load drilldown:", err);
        toastError("Error al cargar detalle");
      } finally {
        setDrillLoading(false);
      }
    },
    [id],
  );

  function handleChartClick(
    widgetId: string,
    widgetTitle: string,
    groupValue: string,
  ) {
    if (!groupValue) return;
    // Reverse the "(Sin valor)" normalization — the DB has empty strings
    const dbGroupValue = groupValue === "(Sin valor)" ? "" : groupValue;
    setDrilldown({ widgetId, widgetTitle, groupValue, dbGroupValue });
    setDrillPage(1);
    setDrillSearch("");
    setExpandedRow(null);
    loadDrilldown(widgetId, dbGroupValue, 1, "", drillPageSize);
  }

  // Recharts' <Line>/<Area> don't support a click handler on the curve/fill
  // itself the way <Bar>/<Pie> do (data, index) — a click there only ever
  // gets the native mouse event, with no way back to which point was hit.
  // The one target that does carry the clicked point's data is the
  // activeDot (the dot Recharts shows at the hovered point) — but it wraps
  // the onClick handler through two layers of its own event-adapter helper,
  // and depending on the layer, the point's props (with `.payload`) land in
  // either the first or the second argument. Rather than hard-code one
  // ordering (which is exactly how the previous version of this handler
  // silently never fired), check both.
  function handleActiveDotClick(widgetId: string, widgetTitle: string) {
    // What recharts actually forwards to this onClick — the dot's props
    // land in either argument depending on the adapter layer.
    type DotClickProps = { payload?: { name?: unknown } };
    return (a?: unknown, b?: unknown) => {
      const aDot = a as DotClickProps | undefined;
      const bDot = b as DotClickProps | undefined;
      const dotProps = bDot?.payload ? bDot : aDot?.payload ? aDot : undefined;
      const d = dotProps?.payload;
      if (d) handleChartClick(widgetId, widgetTitle, String(d.name || ""));
    };
  }

  // Reload drill-down when page or pageSize changes
  useEffect(() => {
    if (drilldown) {
      // debt: loadDrilldown fija drillLoading síncrono a propósito — la
      // paginación del drilldown muestra spinner en el mismo frame.
      // Revisar -> al migrar esta página a TanStack Query (ya instalada).
      // eslint-disable-next-line react-hooks/set-state-in-effect
      loadDrilldown(
        drilldown.widgetId,
        drilldown.dbGroupValue,
        drillPage,
        drillSearch,
        drillPageSize,
      );
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [drillPage, drillPageSize]);

  // Trigger search
  function handleDrillSearch() {
    setDrillPage(1);
    setExpandedRow(null);
    if (drilldown) {
      loadDrilldown(
        drilldown.widgetId,
        drilldown.dbGroupValue,
        1,
        drillSearch,
        drillPageSize,
      );
    }
  }

  // CSV export helpers
  function exportCSV(records: Record<string, unknown>[], filename: string) {
    if (!records.length) return;
    const keys = Object.keys(records[0]).filter((k) => k !== "_id");
    const header = keys.join(",");
    const rows = records.map((r) =>
      keys
        .map((k) => {
          const v = String(r[k] ?? "");
          return v.includes(",") || v.includes('"') || v.includes("\n")
            ? `"${v.replace(/"/g, '""')}"`
            : v;
        })
        .join(","),
    );
    const csv = "\uFEFF" + [header, ...rows].join("\n");
    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = filename;
    a.click();
    URL.revokeObjectURL(url);
  }

  function handleExportPage() {
    if (!drilldown || !drillRecords.length) return;
    exportCSV(
      drillRecords,
      `${drilldown.widgetTitle}-${drilldown.groupValue}-pag${drillPage}.csv`,
    );
  }

  async function handleExportAll() {
    if (!drilldown) return;
    setDrillExporting(true);
    try {
      const result = await dashboardsApi.drilldown(id, drilldown.widgetId, {
        groupValue: drilldown.dbGroupValue,
        page: 1,
        pageSize: 10000,
        search: drillSearch,
        export: true,
      });
      exportCSV(
        result.records || [],
        `${drilldown.widgetTitle}-${drilldown.groupValue}-todos.csv`,
      );
    } catch (err) {
      console.error("Failed to export all:", err);
      toastError("Error al exportar datos");
    } finally {
      setDrillExporting(false);
    }
  }

  const selectedMonitor = monitors.find((m) => m.id === newWidget.monitorId);
  const selectedRule = rules.find((r) => r.id === newWidget.ruleId);
  const drillTotalPages = Math.ceil(drillTotal / drillPageSize);
  const drillFrom = (drillPage - 1) * drillPageSize + 1;
  const drillTo = Math.min(drillPage * drillPageSize, drillTotal);

  if (!dashboard) return null;

  return (
    <>
      <Header title={dashboard.name} />
      <div className="p-6">
        <div className="mb-6 flex items-center justify-between">
          <p className="text-sm text-muted-foreground">
            {dashboard.description}
          </p>
          <Dialog open={isAddOpen} onOpenChange={setIsAddOpen}>
            <DialogTrigger asChild>
              <Button>
                <Plus className="h-4 w-4" /> Agregar widget
              </Button>
            </DialogTrigger>
            <DialogContent className="max-w-lg">
              <DialogHeader>
                <DialogTitle>Nuevo widget</DialogTitle>
              </DialogHeader>
              {/* Mode toggle */}
              <div className="flex gap-2 rounded-lg bg-muted p-1">
                <button
                  type="button"
                  className={`flex-1 rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${widgetMode === "manual" ? "bg-background shadow-sm" : "text-muted-foreground hover:text-foreground"}`}
                  onClick={() => setWidgetMode("manual")}
                >
                  Manual
                </button>
                <button
                  type="button"
                  className={`flex-1 rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${widgetMode === "rule" ? "bg-background shadow-sm" : "text-muted-foreground hover:text-foreground"}`}
                  onClick={() => setWidgetMode("rule")}
                >
                  Desde regla
                </button>
              </div>

              <form onSubmit={addWidget} className="space-y-4">
                {widgetMode === "rule" ? (
                  <>
                    <div className="space-y-2">
                      <Label>Regla</Label>
                      <Select
                        value={newWidget.ruleId}
                        onValueChange={(v) => {
                          const rule = rules.find((r) => r.id === v);
                          setNewWidget({
                            ...newWidget,
                            ruleId: v,
                            title: rule ? `Regla: ${rule.name}` : "",
                            // Sin aggregateConditions, el widget queda en
                            // count sin agrupar — una sola barra "(Sin
                            // valor)" es menos útil que el número solo.
                            type: rule?.aggregateConditions?.length
                              ? newWidget.type
                              : "stat",
                          });
                        }}
                      >
                        <SelectTrigger>
                          <SelectValue placeholder="Selecciona una regla" />
                        </SelectTrigger>
                        <SelectContent>
                          {rules.map((r) => (
                            <SelectItem key={r.id} value={r.id}>
                              <div className="flex items-center gap-2">
                                <span>{r.name}</span>
                                <Badge
                                  variant="outline"
                                  className="text-[10px]"
                                >
                                  {r.severity}
                                </Badge>
                              </div>
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      {selectedRule && (
                        <p className="text-xs text-muted-foreground">
                          Monitor:{" "}
                          {monitors.find((m) => m.id === selectedRule.monitorId)
                            ?.name || selectedRule.monitorId}
                          {selectedRule.aggregateConditions?.length
                            ? ` | ${selectedRule.aggregateConditions[0].function}(${selectedRule.aggregateConditions[0].field}) por ${selectedRule.aggregateConditions[0].groupBy}`
                            : ` | ${selectedRule.conditionGroup.conditions.length} condiciones`}
                        </p>
                      )}
                    </div>
                    <div className="grid grid-cols-2 gap-3">
                      <div className="space-y-2">
                        <Label>Título</Label>
                        <Input
                          value={newWidget.title}
                          onChange={(e) =>
                            setNewWidget({
                              ...newWidget,
                              title: e.target.value,
                            })
                          }
                          required
                        />
                      </div>
                      <div className="space-y-2">
                        <Label>Tipo de grafico</Label>
                        <Select
                          value={newWidget.type}
                          onValueChange={(v) =>
                            setNewWidget({
                              ...newWidget,
                              type: v as WidgetType,
                            })
                          }
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="bar_chart">
                              <span className="flex items-center gap-2">
                                {WIDGET_TYPE_ICONS.bar_chart} Barras
                              </span>
                            </SelectItem>
                            <SelectItem value="line_chart">
                              <span className="flex items-center gap-2">
                                {WIDGET_TYPE_ICONS.line_chart} Lineas
                              </span>
                            </SelectItem>
                            <SelectItem value="pie_chart">
                              <span className="flex items-center gap-2">
                                {WIDGET_TYPE_ICONS.pie_chart} Circular
                              </span>
                            </SelectItem>
                            <SelectItem value="area_chart">
                              <span className="flex items-center gap-2">
                                {WIDGET_TYPE_ICONS.area_chart} Area
                              </span>
                            </SelectItem>
                            <SelectItem value="stat">
                              <span className="flex items-center gap-2">
                                {WIDGET_TYPE_ICONS.stat} Estadistica
                              </span>
                            </SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                    </div>
                  </>
                ) : (
                  <>
                    <div className="space-y-2">
                      <Label>Título</Label>
                      <Input
                        value={newWidget.title}
                        onChange={(e) =>
                          setNewWidget({ ...newWidget, title: e.target.value })
                        }
                        required
                      />
                    </div>
                    <div className="grid grid-cols-2 gap-3">
                      <div className="space-y-2">
                        <Label>Tipo</Label>
                        <Select
                          value={newWidget.type}
                          onValueChange={(v) =>
                            setNewWidget({
                              ...newWidget,
                              type: v as WidgetType,
                            })
                          }
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="bar_chart">
                              <span className="flex items-center gap-2">
                                {WIDGET_TYPE_ICONS.bar_chart} Barras
                              </span>
                            </SelectItem>
                            <SelectItem value="line_chart">
                              <span className="flex items-center gap-2">
                                {WIDGET_TYPE_ICONS.line_chart} Lineas
                              </span>
                            </SelectItem>
                            <SelectItem value="pie_chart">
                              <span className="flex items-center gap-2">
                                {WIDGET_TYPE_ICONS.pie_chart} Circular
                              </span>
                            </SelectItem>
                            <SelectItem value="area_chart">
                              <span className="flex items-center gap-2">
                                {WIDGET_TYPE_ICONS.area_chart} Area
                              </span>
                            </SelectItem>
                            <SelectItem value="stat">
                              <span className="flex items-center gap-2">
                                {WIDGET_TYPE_ICONS.stat} Estadistica
                              </span>
                            </SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                      <div className="space-y-2">
                        <Label>Monitor</Label>
                        <Select
                          value={newWidget.monitorId}
                          onValueChange={(v) =>
                            setNewWidget({ ...newWidget, monitorId: v })
                          }
                        >
                          <SelectTrigger>
                            <SelectValue placeholder="Monitor" />
                          </SelectTrigger>
                          <SelectContent>
                            {monitors.map((m) => (
                              <SelectItem key={m.id} value={m.id}>
                                {m.name}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                    </div>
                    <div className="grid grid-cols-2 gap-3">
                      <div className="space-y-2">
                        <Label>Campo</Label>
                        <Select
                          value={newWidget.field}
                          onValueChange={(v) =>
                            setNewWidget({ ...newWidget, field: v })
                          }
                        >
                          <SelectTrigger>
                            <SelectValue placeholder="Campo" />
                          </SelectTrigger>
                          <SelectContent>
                            {selectedMonitor?.schema?.map((f) => (
                              <SelectItem key={f.name} value={f.name}>
                                {f.name}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                      <div className="space-y-2">
                        <Label>Agregacion</Label>
                        <Select
                          value={newWidget.aggregation}
                          onValueChange={(v) =>
                            setNewWidget({
                              ...newWidget,
                              aggregation: v as AggregationType,
                            })
                          }
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="count">Contar</SelectItem>
                            <SelectItem value="sum">Suma</SelectItem>
                            <SelectItem value="avg">Promedio</SelectItem>
                            <SelectItem value="min">Mínimo</SelectItem>
                            <SelectItem value="max">Máximo</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                    </div>
                    <div className="space-y-2">
                      <Label>Agrupar por (opcional)</Label>
                      <Select
                        value={newWidget.groupBy || "__none__"}
                        onValueChange={(v) =>
                          setNewWidget({
                            ...newWidget,
                            groupBy: v === "__none__" ? "" : v,
                          })
                        }
                      >
                        <SelectTrigger>
                          <SelectValue placeholder="Sin agrupacion" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="__none__">Ninguno</SelectItem>
                          {selectedMonitor?.schema?.map((f) => (
                            <SelectItem key={f.name} value={f.name}>
                              {f.name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                  </>
                )}
                <Button type="submit" className="w-full">
                  Agregar widget
                </Button>
              </form>
            </DialogContent>
          </Dialog>
        </div>

        {dashboard.widgets?.length === 0 ? (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12">
              <BarChart3 className="mb-3 h-10 w-10 text-muted-foreground/50" />
              <p className="text-sm text-muted-foreground">
                Agrega widgets para visualizar tus datos
              </p>
            </CardContent>
          </Card>
        ) : (
          <div className="grid gap-4 sm:grid-cols-2">
            {dashboard.widgets?.map((widget) => {
              const rawData = (widgetData[widget.id] || []) as Record<
                string,
                unknown
              >[];
              // Normalize empty names to "(Sin valor)"
              const data = rawData.map((d) => ({
                ...d,
                name:
                  d.name && String(d.name).trim()
                    ? String(d.name)
                    : "(Sin valor)",
              })) as Record<string, unknown>[];
              // Add total for pie chart percentage calc
              const pieTotal = data.reduce(
                (sum, d) => sum + (Number(d.value) || 0),
                0,
              );
              const pieData = data.map((d) => ({ ...d, total: pieTotal }));
              const widgetError = widgetErrors[widget.id];

              return (
                <Card key={widget.id} className="group overflow-hidden">
                  <CardHeader className="pb-2">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <span className="text-muted-foreground">
                          {WIDGET_TYPE_ICONS[widget.type]}
                        </span>
                        <CardTitle className="text-sm font-semibold">
                          {widget.title}
                        </CardTitle>
                      </div>
                      <div className="flex items-center gap-1">
                        {widget.groupBy && (
                          <span className="flex items-center gap-1 text-[10px] text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100">
                            <MousePointerClick className="h-3 w-3" /> Click para
                            detalle
                          </span>
                        )}
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-6 w-6 opacity-0 group-hover:opacity-100"
                          onClick={() => deleteWidget(widget.id)}
                        >
                          <Trash2 className="h-3 w-3 text-destructive" />
                        </Button>
                      </div>
                    </div>
                    <p className="text-[11px] text-muted-foreground">
                      {widget.aggregation.toUpperCase()}({widget.field || "*"})
                      {widget.groupBy ? ` por ${widget.groupBy}` : ""}
                    </p>
                  </CardHeader>
                  <CardContent>
                    <div className="h-80">
                      {widgetError ? (
                        <div className="flex h-full flex-col items-center justify-center gap-2 text-center">
                          <AlertTriangle className="h-8 w-8 text-muted-foreground/50" />
                          <p className="max-w-[85%] text-sm text-muted-foreground">
                            {widgetError}
                          </p>
                        </div>
                      ) : widget.type === "bar_chart" ? (
                        <ResponsiveContainer width="100%" height="100%">
                          <BarChart
                            data={data}
                            margin={{ top: 5, right: 10, left: 0, bottom: 5 }}
                          >
                            <defs>
                              <linearGradient
                                id={`barGrad-${widget.id}`}
                                x1="0"
                                y1="0"
                                x2="0"
                                y2="1"
                              >
                                <stop
                                  offset="0%"
                                  stopColor={CHART_COLORS[0]}
                                  stopOpacity={1}
                                />
                                <stop
                                  offset="100%"
                                  stopColor={CHART_COLORS[0]}
                                  stopOpacity={0.6}
                                />
                              </linearGradient>
                            </defs>
                            <CartesianGrid
                              strokeDasharray="3 3"
                              className="stroke-muted/30"
                              vertical={false}
                            />
                            <XAxis
                              dataKey="name"
                              tick={{ fontSize: 11 }}
                              tickLine={false}
                              axisLine={false}
                              className="fill-muted-foreground"
                            />
                            <YAxis
                              tick={{ fontSize: 11 }}
                              tickLine={false}
                              axisLine={false}
                              tickFormatter={(v) => fmt.format(v)}
                              className="fill-muted-foreground"
                              width={55}
                            />
                            <RechartsTooltip
                              content={<CustomTooltip />}
                              cursor={{
                                fill: "hsl(var(--muted))",
                                opacity: 0.5,
                              }}
                            />
                            <Bar
                              dataKey="value"
                              fill={`url(#barGrad-${widget.id})`}
                              radius={[6, 6, 0, 0]}
                              cursor="pointer"
                              onClick={(d) =>
                                handleChartClick(
                                  widget.id,
                                  widget.title,
                                  String(d.name || ""),
                                )
                              }
                            />
                          </BarChart>
                        </ResponsiveContainer>
                      ) : widget.type === "line_chart" ? (
                        <ResponsiveContainer width="100%" height="100%">
                          <LineChart
                            data={data}
                            margin={{ top: 5, right: 10, left: 0, bottom: 5 }}
                          >
                            <defs>
                              <linearGradient
                                id={`lineArea-${widget.id}`}
                                x1="0"
                                y1="0"
                                x2="0"
                                y2="1"
                              >
                                <stop
                                  offset="0%"
                                  stopColor={CHART_COLORS[1]}
                                  stopOpacity={0.15}
                                />
                                <stop
                                  offset="100%"
                                  stopColor={CHART_COLORS[1]}
                                  stopOpacity={0}
                                />
                              </linearGradient>
                            </defs>
                            <CartesianGrid
                              strokeDasharray="3 3"
                              className="stroke-muted/30"
                              vertical={false}
                            />
                            <XAxis
                              dataKey="name"
                              tick={{ fontSize: 11 }}
                              tickLine={false}
                              axisLine={false}
                              className="fill-muted-foreground"
                            />
                            <YAxis
                              tick={{ fontSize: 11 }}
                              tickLine={false}
                              axisLine={false}
                              tickFormatter={(v) => fmt.format(v)}
                              className="fill-muted-foreground"
                              width={55}
                            />
                            <RechartsTooltip content={<CustomTooltip />} />
                            <Line
                              type="monotone"
                              dataKey="value"
                              stroke={CHART_COLORS[1]}
                              strokeWidth={2.5}
                              dot={{
                                r: 4,
                                fill: CHART_COLORS[1],
                                strokeWidth: 0,
                              }}
                              activeDot={{
                                r: 6,
                                strokeWidth: 2,
                                stroke: "hsl(var(--background))",
                                cursor: "pointer",
                                onClick: handleActiveDotClick(
                                  widget.id,
                                  widget.title,
                                ),
                              }}
                            />
                          </LineChart>
                        </ResponsiveContainer>
                      ) : widget.type === "area_chart" ? (
                        <ResponsiveContainer width="100%" height="100%">
                          <AreaChart
                            data={data}
                            margin={{ top: 5, right: 10, left: 0, bottom: 5 }}
                          >
                            <defs>
                              <linearGradient
                                id={`areaGrad-${widget.id}`}
                                x1="0"
                                y1="0"
                                x2="0"
                                y2="1"
                              >
                                <stop
                                  offset="0%"
                                  stopColor={CHART_COLORS[0]}
                                  stopOpacity={0.3}
                                />
                                <stop
                                  offset="100%"
                                  stopColor={CHART_COLORS[0]}
                                  stopOpacity={0.02}
                                />
                              </linearGradient>
                            </defs>
                            <CartesianGrid
                              strokeDasharray="3 3"
                              className="stroke-muted/30"
                              vertical={false}
                            />
                            <XAxis
                              dataKey="name"
                              tick={{ fontSize: 11 }}
                              tickLine={false}
                              axisLine={false}
                              className="fill-muted-foreground"
                            />
                            <YAxis
                              tick={{ fontSize: 11 }}
                              tickLine={false}
                              axisLine={false}
                              tickFormatter={(v) => fmt.format(v)}
                              className="fill-muted-foreground"
                              width={55}
                            />
                            <RechartsTooltip content={<CustomTooltip />} />
                            <Area
                              type="monotone"
                              dataKey="value"
                              stroke={CHART_COLORS[0]}
                              strokeWidth={2}
                              fill={`url(#areaGrad-${widget.id})`}
                              activeDot={{
                                cursor: "pointer",
                                onClick: handleActiveDotClick(
                                  widget.id,
                                  widget.title,
                                ),
                              }}
                            />
                          </AreaChart>
                        </ResponsiveContainer>
                      ) : widget.type === "pie_chart" ? (
                        <ResponsiveContainer width="100%" height="100%">
                          <PieChart>
                            <Pie
                              data={pieData}
                              cx="50%"
                              cy="45%"
                              innerRadius="40%"
                              outerRadius="70%"
                              dataKey="value"
                              nameKey="name"
                              paddingAngle={2}
                              cursor="pointer"
                              onClick={(_, idx) => {
                                const d = data[idx];
                                if (d)
                                  handleChartClick(
                                    widget.id,
                                    widget.title,
                                    String(d.name || ""),
                                  );
                              }}
                            >
                              {pieData.map((_, i) => (
                                <Cell
                                  key={i}
                                  fill={CHART_COLORS[i % CHART_COLORS.length]}
                                  stroke="var(--bg-canvas)"
                                  strokeWidth={2}
                                />
                              ))}
                            </Pie>
                            <RechartsTooltip content={<PieTooltip />} />
                            <Legend
                              verticalAlign="bottom"
                              height={36}
                              formatter={(value: string) => (
                                <span className="text-xs text-muted-foreground">
                                  {value}
                                </span>
                              )}
                            />
                          </PieChart>
                        </ResponsiveContainer>
                      ) : widget.type === "stat" ? (
                        <div className="flex h-full flex-col items-center justify-center">
                          <TrendingUp className="mb-2 h-8 w-8 text-muted-foreground/40" />
                          <p className="text-5xl font-bold tracking-tight">
                            {fmtFull.format(Number(data[0]?.value) || 0)}
                          </p>
                          <p className="mt-2 text-sm text-muted-foreground">
                            {widget.aggregation.toUpperCase()}({widget.field})
                          </p>
                          {Boolean(rawData[0]?.name) && (
                            <Badge variant="outline" className="mt-1 text-xs">
                              {String(data[0].name)}
                            </Badge>
                          )}
                        </div>
                      ) : (
                        <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
                          Tipo no soportado
                        </div>
                      )}
                    </div>
                  </CardContent>
                </Card>
              );
            })}
          </div>
        )}
      </div>

      {/* Drill-down Sheet */}
      <Sheet
        open={!!drilldown}
        onOpenChange={(open) => {
          if (!open) {
            setDrilldown(null);
            setExpandedRow(null);
          }
        }}
      >
        <SheetContent className="w-full sm:max-w-5xl overflow-y-auto">
          <SheetHeader>
            <div className="flex items-center gap-3">
              <SheetTitle className="text-base">
                {drilldown?.widgetTitle}
              </SheetTitle>
              <Badge variant="secondary" className="text-xs">
                {drilldown?.groupValue}
              </Badge>
              <Badge variant="outline" className="text-xs">
                {drillTotal.toLocaleString("es")} registros
              </Badge>
            </div>
          </SheetHeader>

          <div className="mt-4 space-y-3">
            {/* Toolbar: Search + Export + PageSize */}
            <div className="flex flex-wrap items-center gap-2">
              <div className="relative flex-1 min-w-[200px]">
                <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  placeholder="Buscar en registros..."
                  className="pl-9 pr-20 h-9"
                  value={drillSearch}
                  onChange={(e) => setDrillSearch(e.target.value)}
                  onKeyDown={(e) => e.key === "Enter" && handleDrillSearch()}
                />
                <Button
                  variant="ghost"
                  size="sm"
                  className="absolute right-1 top-1/2 -translate-y-1/2 h-7 px-2 text-xs"
                  onClick={handleDrillSearch}
                >
                  Buscar
                </Button>
              </div>

              <div className="flex items-center gap-1">
                <Button
                  variant="outline"
                  size="sm"
                  className="h-9 gap-1.5 text-xs"
                  onClick={handleExportPage}
                  disabled={!drillRecords.length}
                >
                  <Download className="h-3.5 w-3.5" /> Pagina
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  className="h-9 gap-1.5 text-xs"
                  onClick={handleExportAll}
                  disabled={drillExporting || !drillTotal}
                  title={
                    drillTotal > EXPORT_ALL_LIMIT
                      ? `El export se limita a los primeros ${EXPORT_ALL_LIMIT.toLocaleString("es")} registros`
                      : undefined
                  }
                >
                  {drillExporting ? (
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <Download className="h-3.5 w-3.5" />
                  )}
                  Todos ({drillTotal.toLocaleString("es")})
                </Button>
              </div>

              <Select
                value={String(drillPageSize)}
                onValueChange={(v) => {
                  setDrillPageSize(Number(v));
                  setDrillPage(1);
                }}
              >
                <SelectTrigger className="w-[90px] h-9 text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="25">25 / pag</SelectItem>
                  <SelectItem value="50">50 / pag</SelectItem>
                  <SelectItem value="100">100 / pag</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {drillTotal > EXPORT_ALL_LIMIT && (
              <p className="text-xs text-warning-fg">
                &quot;Todos&quot; exporta como máximo los primeros{" "}
                {EXPORT_ALL_LIMIT.toLocaleString("es")} registros.
              </p>
            )}

            {/* Range indicator */}
            {drillTotal > 0 && (
              <p className="text-xs text-muted-foreground">
                Mostrando {drillFrom}–{drillTo} de{" "}
                {drillTotal.toLocaleString("es")}
              </p>
            )}

            {/* Loading skeleton */}
            {drillLoading && (
              <div className="space-y-2">
                {Array.from({ length: 6 }).map((_, i) => (
                  <div key={i} className="flex gap-3">
                    {Array.from({ length: 5 }).map((_, j) => (
                      <div
                        key={j}
                        className="h-4 flex-1 animate-pulse rounded bg-muted"
                      />
                    ))}
                  </div>
                ))}
              </div>
            )}

            {/* Records table */}
            {!drillLoading &&
              drillRecords.length > 0 &&
              (() => {
                const columns = Object.keys(drillRecords[0]).filter(
                  (k) => k !== "_id",
                );
                return (
                  <div className="overflow-x-auto rounded-lg border max-h-[calc(100vh-280px)]">
                    <table className="w-full text-xs">
                      <thead className="sticky top-0 z-10">
                        <tr className="border-b bg-muted/80 backdrop-blur-sm">
                          <th className="w-8 px-2 py-2" />
                          {columns.map((key) => (
                            <th
                              key={key}
                              className="whitespace-nowrap px-3 py-2 text-left font-medium text-muted-foreground"
                            >
                              {key}
                            </th>
                          ))}
                        </tr>
                      </thead>
                      <tbody>
                        {drillRecords.map((record, i) => (
                          <React.Fragment key={i}>
                            <tr
                              className={`border-b last:border-0 cursor-pointer transition-colors hover:bg-muted/40 ${i % 2 === 0 ? "bg-transparent" : "bg-muted/15"} ${expandedRow === i ? "bg-primary/5" : ""}`}
                              onClick={() =>
                                setExpandedRow(expandedRow === i ? null : i)
                              }
                            >
                              <td className="px-2 py-2 text-muted-foreground">
                                {expandedRow === i ? (
                                  <ChevronUp className="h-3 w-3" />
                                ) : (
                                  <ChevronDown className="h-3 w-3" />
                                )}
                              </td>
                              {columns.map((key) => {
                                const val = record[key];
                                return (
                                  <td
                                    key={key}
                                    className="whitespace-nowrap px-3 py-2 max-w-[250px] truncate"
                                  >
                                    {typeof val === "number"
                                      ? fmtFull.format(val)
                                      : String(val ?? "")}
                                  </td>
                                );
                              })}
                            </tr>
                            {expandedRow === i && (
                              <tr className="border-b bg-muted/10">
                                <td
                                  colSpan={columns.length + 1}
                                  className="px-4 py-3"
                                >
                                  <div className="grid grid-cols-2 gap-x-6 gap-y-1 text-xs">
                                    {columns.map((key) => (
                                      <div key={key} className="flex gap-2">
                                        <span className="font-medium text-muted-foreground min-w-[120px]">
                                          {key}:
                                        </span>
                                        <span className="break-all">
                                          {typeof record[key] === "number"
                                            ? fmtFull.format(
                                                record[key] as number,
                                              )
                                            : String(record[key] ?? "")}
                                        </span>
                                      </div>
                                    ))}
                                  </div>
                                </td>
                              </tr>
                            )}
                          </React.Fragment>
                        ))}
                      </tbody>
                    </table>
                  </div>
                );
              })()}

            {!drillLoading && drillRecords.length === 0 && (
              <p className="py-8 text-center text-sm text-muted-foreground">
                Sin registros
              </p>
            )}

            {/* Pagination */}
            {drillTotalPages > 1 && (
              <div className="flex items-center justify-between pt-1">
                <p className="text-xs text-muted-foreground">
                  Pagina {drillPage} de {drillTotalPages}
                </p>
                <div className="flex items-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={drillPage <= 1}
                    onClick={() => setDrillPage((p) => p - 1)}
                  >
                    <ChevronLeft className="h-4 w-4 mr-1" /> Anterior
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={drillPage >= drillTotalPages}
                    onClick={() => setDrillPage((p) => p + 1)}
                  >
                    Siguiente <ChevronRight className="h-4 w-4 ml-1" />
                  </Button>
                </div>
              </div>
            )}
          </div>
        </SheetContent>
      </Sheet>
    </>
  );
}
