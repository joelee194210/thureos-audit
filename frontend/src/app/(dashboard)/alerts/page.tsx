"use client";

import { useEffect, useState, useMemo, useCallback } from "react";
import { Header } from "@/components/layout/header";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
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
} from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import {
  Bell,
  Check,
  Eye,
  X,
  ChevronDown,
  ChevronUp,
  ChevronLeft,
  ChevronRight,
  Search,
  AlertTriangle,
  ShieldAlert,
  Info,
  Database,
  Calculator,
  Calendar,
  Loader2,
  ClipboardList,
} from "lucide-react";
import { alertsApi } from "@/lib/api/alerts";
import { monitorsApi } from "@/lib/api/monitors";
import type { AlertRecordsResponse, CalendarDay } from "@/lib/api/alerts";
import { useToast } from "@/lib/use-toast";
import { formatDate } from "@/lib/utils";
import type { Alert, AlertStatus, AlertLog, ActionCategory, Severity, Monitor } from "@/lib/types";

const severityConfig: Record<
  Severity,
  {
    variant: "destructive" | "warning" | "secondary" | "default";
    color: string;
    icon: typeof AlertTriangle;
    label: string;
  }
> = {
  critical: {
    variant: "destructive",
    color: "border-l-red-500 bg-red-500/5",
    icon: ShieldAlert,
    label: "Critica",
  },
  high: {
    variant: "destructive",
    color: "border-l-orange-500 bg-orange-500/5",
    icon: AlertTriangle,
    label: "Alta",
  },
  medium: {
    variant: "warning",
    color: "border-l-yellow-500 bg-yellow-500/5",
    icon: Info,
    label: "Media",
  },
  low: {
    variant: "secondary",
    color: "border-l-blue-500 bg-blue-500/5",
    icon: Info,
    label: "Baja",
  },
};

const statusLabels: Record<AlertStatus, string> = {
  new: "Nueva",
  acknowledged: "Reconocida",
  resolved: "Resuelta",
  dismissed: "Descartada",
};

function formatNumber(val: unknown): string {
  const num = Number(val);
  if (isNaN(num)) return String(val ?? "—");
  if (Math.abs(num) >= 1000) {
    return num.toLocaleString("es-US", {
      minimumFractionDigits: num % 1 === 0 ? 0 : 2,
      maximumFractionDigits: 2,
    });
  }
  return String(val);
}

function isNumericField(val: unknown): boolean {
  return typeof val === "number" || (typeof val === "string" && !isNaN(Number(val)) && val.trim() !== "");
}

const AGG_RE = /Aggregate rule '.*?': (\w+)\((\w+)\) for (\w+)='(.+?)' = ([\d.]+) \(threshold: (\w+) ([\d.]+)\)/;

function parseAggregateFromMessage(alert: Alert): Alert {
  if (alert.alertType === "aggregate") return alert;
  const m = alert.message.match(AGG_RE);
  if (!m) return alert;
  return {
    ...alert,
    alertType: "aggregate",
    aggFunction: m[1],
    aggField: m[2],
    groupByField: m[3],
    groupByValue: m[4],
    aggValue: parseFloat(m[5]),
    threshold: parseFloat(m[7]),
    matchCount: alert.matchedData?.count != null ? Number(alert.matchedData.count) : alert.matchCount,
  };
}

const MONTH_NAMES = [
  "Enero", "Febrero", "Marzo", "Abril", "Mayo", "Junio",
  "Julio", "Agosto", "Septiembre", "Octubre", "Noviembre", "Diciembre",
];
const DAY_NAMES = ["Lun", "Mar", "Mié", "Jue", "Vie", "Sáb", "Dom"];

// ─── Calendar Component ──────────────────────────────────────
function AlertCalendar({
  selectedDate,
  onSelectDate,
}: {
  selectedDate: string | null;
  onSelectDate: (date: string | null) => void;
}) {
  const today = new Date();
  const [year, setYear] = useState(today.getFullYear());
  const [month, setMonth] = useState(today.getMonth() + 1);
  const [calendarData, setCalendarData] = useState<CalendarDay[]>([]);

  useEffect(() => {
    alertsApi.calendar(year, month).then(data => setCalendarData(data ?? [])).catch((err) => {
      console.error("Failed to load calendar data:", err);
    });
  }, [year, month]);

  const dayMap = useMemo(() => {
    const m = new Map<string, CalendarDay>();
    for (const d of calendarData) m.set(d._id, d);
    return m;
  }, [calendarData]);

  const firstDay = new Date(year, month - 1, 1);
  const daysInMonth = new Date(year, month, 0).getDate();
  // Monday=0, Sunday=6
  let startWeekday = firstDay.getDay() - 1;
  if (startWeekday < 0) startWeekday = 6;

  const prevMonth = () => {
    if (month === 1) { setMonth(12); setYear(y => y - 1); }
    else setMonth(m => m - 1);
  };
  const nextMonth = () => {
    if (month === 12) { setMonth(1); setYear(y => y + 1); }
    else setMonth(m => m + 1);
  };

  const cells: (number | null)[] = [];
  for (let i = 0; i < startWeekday; i++) cells.push(null);
  for (let d = 1; d <= daysInMonth; d++) cells.push(d);

  const todayStr = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, "0")}-${String(today.getDate()).padStart(2, "0")}`;

  return (
    <Card>
      <CardContent className="p-4">
        <div className="flex items-center justify-between mb-3">
          <Button variant="ghost" size="icon" onClick={prevMonth}>
            <ChevronLeft className="h-4 w-4" />
          </Button>
          <h3 className="text-sm font-semibold">
            {MONTH_NAMES[month - 1]} {year}
          </h3>
          <Button variant="ghost" size="icon" onClick={nextMonth}>
            <ChevronRight className="h-4 w-4" />
          </Button>
        </div>

        <div className="grid grid-cols-7 gap-1 text-center">
          {DAY_NAMES.map((d) => (
            <div key={d} className="text-[10px] font-medium text-muted-foreground py-1">
              {d}
            </div>
          ))}
          {cells.map((day, idx) => {
            if (day === null) return <div key={`empty-${idx}`} />;
            const dateStr = `${year}-${String(month).padStart(2, "0")}-${String(day).padStart(2, "0")}`;
            const data = dayMap.get(dateStr);
            const isSelected = selectedDate === dateStr;
            const isToday = dateStr === todayStr;
            const hasAlerts = data && data.total > 0;
            const dotColor = data?.critical ? "bg-red-500" : data?.high ? "bg-orange-500" : "bg-yellow-500";

            return (
              <button
                key={dateStr}
                onClick={() => onSelectDate(isSelected ? null : dateStr)}
                className={`
                  relative flex flex-col items-center justify-center rounded-md p-1 text-xs transition-colors
                  ${isSelected ? "bg-primary text-primary-foreground" : ""}
                  ${isToday && !isSelected ? "ring-1 ring-primary" : ""}
                  ${!isSelected && hasAlerts ? "hover:bg-accent" : "hover:bg-muted/50"}
                `}
              >
                <span className="tabular-nums">{day}</span>
                {hasAlerts && (
                  <div className="flex gap-0.5 mt-0.5">
                    <div className={`h-1 w-1 rounded-full ${dotColor}`} />
                    {data!.total > 5 && <div className="h-1 w-1 rounded-full bg-muted-foreground" />}
                  </div>
                )}
                {hasAlerts && !isSelected && (
                  <span className="absolute -top-0.5 -right-0.5 flex h-3.5 w-3.5 items-center justify-center rounded-full bg-destructive text-[8px] text-destructive-foreground font-bold">
                    {data!.total > 99 ? "99+" : data!.total}
                  </span>
                )}
              </button>
            );
          })}
        </div>

        {selectedDate && (
          <div className="mt-3 flex items-center justify-between border-t pt-2">
            <span className="text-xs text-muted-foreground">
              <Calendar className="h-3 w-3 inline mr-1" />
              Filtrando: {selectedDate}
            </span>
            <Button variant="ghost" size="sm" className="h-6 text-xs" onClick={() => onSelectDate(null)}>
              Limpiar
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

// ─── AlertDataTable with lazy loading ──────────────────────────
function AlertDataTable({ alert }: { alert: Alert }) {
  const [data, setData] = useState<AlertRecordsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [dataSearch, setDataSearch] = useState("");
  const pageSize = 50;

  const loadPage = useCallback(async (p: number) => {
    setLoading(true);
    try {
      const res = await alertsApi.records(alert.id, p, pageSize);
      setData(res);
      setPage(p);
    } catch (err) {
      console.error("Failed to load paginated records, using embedded fallback:", err);
      const records = alert.matchedRecords ?? (alert.matchedData ? [alert.matchedData] : []);
      setData({
        records: records as Record<string, unknown>[],
        total: alert.matchCount || records.length,
        page: 1,
        limit: records.length,
        totalPages: 1,
      });
    } finally {
      setLoading(false);
    }
  }, [alert]);

  useEffect(() => {
    loadPage(1);
  }, [loadPage]);

  if (loading && !data) {
    return (
      <div className="mt-3 flex items-center justify-center gap-2 rounded-md border p-6 text-sm text-muted-foreground">
        <Loader2 className="h-4 w-4 animate-spin" />
        Cargando registros...
      </div>
    );
  }

  if (!data || data.records.length === 0) {
    return (
      <div className="mt-3 rounded-md border p-4 text-center text-sm text-muted-foreground">
        No hay datos disponibles para esta alerta
      </div>
    );
  }

  const columns = Object.keys(data.records[0]).filter((k) => !k.startsWith("_"));

  const filteredRecords = dataSearch
    ? data.records.filter((rec) => {
        const q = dataSearch.toLowerCase();
        return columns.some((col) => String(rec[col] ?? "").toLowerCase().includes(q));
      })
    : data.records;

  return (
    <div className="mt-3 space-y-2">
      <div className="flex items-center justify-between gap-2">
        <p className="text-xs font-medium text-muted-foreground shrink-0">
          <Database className="h-3 w-3 inline mr-1" />
          {dataSearch
            ? `${filteredRecords.length} de ${data.records.length} registros`
            : `Mostrando ${(page - 1) * pageSize + 1}-${Math.min(page * pageSize, data.total)} de ${data.total.toLocaleString()} registros`}
        </p>
        <div className="relative w-64">
          <Search className="absolute left-2.5 top-1/2 h-3 w-3 -translate-y-1/2 text-muted-foreground" />
          <Input
            className="h-7 pl-7 text-xs"
            placeholder="Buscar en datos..."
            value={dataSearch}
            onChange={(e) => setDataSearch(e.target.value)}
          />
          {dataSearch && (
            <button
              onClick={() => setDataSearch("")}
              className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
            >
              <X className="h-3 w-3" />
            </button>
          )}
        </div>
        <Badge variant="outline" className="text-[10px] shrink-0">
          {columns.length} campos
        </Badge>
      </div>
      <div className="rounded-md border overflow-hidden">
        <div className="overflow-x-auto max-h-[400px] overflow-y-auto">
          <table className="w-full text-xs">
            <thead className="bg-muted/70 sticky top-0 z-10">
              <tr>
                <th className="px-3 py-2 text-left font-medium text-muted-foreground w-8">#</th>
                {columns.map((col) => (
                  <th
                    key={col}
                    className="px-3 py-2 text-left font-medium text-muted-foreground whitespace-nowrap"
                  >
                    {col}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-border/50">
              {filteredRecords.map((rec, idx) => (
                <tr key={idx} className="hover:bg-muted/30 transition-colors">
                  <td className="px-3 py-1.5 text-muted-foreground tabular-nums">
                    {dataSearch ? idx + 1 : (page - 1) * pageSize + idx + 1}
                  </td>
                  {columns.map((col) => {
                    const val = rec[col];
                    const isNum = isNumericField(val);
                    return (
                      <td
                        key={col}
                        className={`px-3 py-1.5 whitespace-nowrap max-w-[200px] truncate ${
                          isNum ? "text-right tabular-nums font-medium" : ""
                        }`}
                        title={String(val ?? "")}
                      >
                        {isNum ? formatNumber(val) : String(val ?? "—")}
                      </td>
                    );
                  })}
                </tr>
              ))}
            </tbody>
          </table>
          {dataSearch && filteredRecords.length === 0 && (
            <div className="p-6 text-center text-xs text-muted-foreground">
              Sin resultados para &quot;{dataSearch}&quot;
            </div>
          )}
        </div>
      </div>
      {/* Pagination */}
      {data.totalPages > 1 && !dataSearch && (
        <div className="flex items-center justify-between pt-1">
          <p className="text-[10px] text-muted-foreground">
            Pagina {page} de {data.totalPages}
          </p>
          <div className="flex gap-1">
            <Button
              variant="outline"
              size="sm"
              className="h-7 text-xs"
              disabled={page <= 1 || loading}
              onClick={() => loadPage(page - 1)}
            >
              <ChevronLeft className="h-3 w-3 mr-1" />
              Anterior
            </Button>
            <Button
              variant="outline"
              size="sm"
              className="h-7 text-xs"
              disabled={page >= data.totalPages || loading}
              onClick={() => loadPage(page + 1)}
            >
              Siguiente
              <ChevronRight className="h-3 w-3 ml-1" />
            </Button>
          </div>
        </div>
      )}
      {loading && (
        <div className="flex justify-center py-1">
          <Loader2 className="h-3 w-3 animate-spin text-muted-foreground" />
        </div>
      )}
    </div>
  );
}

// ─── Main Page ──────────────────────────────────────────────
export default function AlertsPage() {
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [monitors, setMonitors] = useState<Monitor[]>([]);
  const [filter, setFilter] = useState<string>("all");
  const [filterMonitorId, setFilterMonitorId] = useState<string>("all");
  const [searchTerm, setSearchTerm] = useState("");
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [selectedDate, setSelectedDate] = useState<string | null>(null);
  const { toastError } = useToast();

  useEffect(() => {
    monitorsApi.list().then(setMonitors).catch((err) => {
      console.error("Failed to load monitors:", err);
    });
  }, []);

  useEffect(() => {
    loadAlerts();
  }, [selectedDate]);

  async function loadAlerts() {
    try {
      let raw: Alert[];
      if (selectedDate) {
        raw = await alertsApi.byDate(selectedDate);
      } else {
        raw = await alertsApi.list({ limit: 100 });
      }
      setAlerts((raw ?? []).map(parseAggregateFromMessage));
    } catch {
      toastError("Error al cargar alertas");
    }
  }

  // Action dialog state
  const [actionDialog, setActionDialog] = useState<{ alert: Alert; targetStatus: AlertStatus } | null>(null);
  const [actionCategory, setActionCategory] = useState<ActionCategory>("investigation");
  const [actionNotes, setActionNotes] = useState("");
  const [actionLoading, setActionLoading] = useState(false);

  // Log timeline dialog state
  const [logDialog, setLogDialog] = useState<Alert | null>(null);
  const [alertLogs, setAlertLogs] = useState<AlertLog[]>([]);
  const [logsLoading, setLogsLoading] = useState(false);

  function openActionDialog(alert: Alert, targetStatus: AlertStatus) {
    setActionDialog({ alert, targetStatus });
    setActionCategory("investigation");
    setActionNotes("");
  }

  async function submitAction() {
    if (!actionDialog) return;
    if (!actionNotes.trim()) { toastError("Las notas son requeridas"); return; }
    setActionLoading(true);
    try {
      await alertsApi.updateStatus(actionDialog.alert.id, actionDialog.targetStatus, actionCategory, actionNotes);
      setActionDialog(null);
      loadAlerts();
    } catch {
      toastError("Error al actualizar alerta");
    } finally {
      setActionLoading(false);
    }
  }

  async function openLogTimeline(alert: Alert) {
    setLogDialog(alert);
    setLogsLoading(true);
    try {
      setAlertLogs(await alertsApi.getLogs(alert.id));
    } catch {
      toastError("Error al cargar bitácora");
    } finally {
      setLogsLoading(false);
    }
  }

  const filtered = useMemo(() => {
    let result = filter === "all" ? alerts : alerts.filter((a) => a.status === filter);
    if (filterMonitorId !== "all") {
      result = result.filter((a) => a.monitorId === filterMonitorId);
    }
    if (searchTerm.trim()) {
      const term = searchTerm.toLowerCase();
      result = result.filter(
        (a) =>
          a.ruleName.toLowerCase().includes(term) ||
          a.monitorName.toLowerCase().includes(term) ||
          a.message.toLowerCase().includes(term) ||
          (a.groupByValue && a.groupByValue.toLowerCase().includes(term))
      );
    }
    return result;
  }, [alerts, filter, filterMonitorId, searchTerm]);

  const stats = useMemo(() => {
    let newCount = 0;
    let criticalCount = 0;
    let highCount = 0;
    let aggCount = 0;
    for (const a of alerts) {
      if (a.status === "new") {
        newCount++;
        if (a.severity === "critical") criticalCount++;
        if (a.severity === "high") highCount++;
      }
      if (a.alertType === "aggregate") aggCount++;
    }
    return { newCount, criticalCount, highCount, aggCount };
  }, [alerts]);

  return (
    <>
      <Header title="Alertas" />
      <div className="p-6 space-y-4">
        {/* Top: Calendar + Stats */}
        <div className="grid gap-4 lg:grid-cols-[280px_1fr]">
          <AlertCalendar selectedDate={selectedDate} onSelectDate={setSelectedDate} />

          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4 content-start">
            <Card>
              <CardContent className="flex items-center gap-3 p-3">
                <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-red-500/10">
                  <Bell className="h-4 w-4 text-red-500" />
                </div>
                <div>
                  <p className="text-2xl font-bold">{stats.newCount}</p>
                  <p className="text-xs text-muted-foreground">Nuevas</p>
                </div>
              </CardContent>
            </Card>
            <Card>
              <CardContent className="flex items-center gap-3 p-3">
                <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-red-500/10">
                  <ShieldAlert className="h-4 w-4 text-red-500" />
                </div>
                <div>
                  <p className="text-2xl font-bold">{stats.criticalCount}</p>
                  <p className="text-xs text-muted-foreground">Criticas</p>
                </div>
              </CardContent>
            </Card>
            <Card>
              <CardContent className="flex items-center gap-3 p-3">
                <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-orange-500/10">
                  <AlertTriangle className="h-4 w-4 text-orange-500" />
                </div>
                <div>
                  <p className="text-2xl font-bold">{stats.highCount}</p>
                  <p className="text-xs text-muted-foreground">Altas</p>
                </div>
              </CardContent>
            </Card>
            <Card>
              <CardContent className="flex items-center gap-3 p-3">
                <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-blue-500/10">
                  <Calculator className="h-4 w-4 text-blue-500" />
                </div>
                <div>
                  <p className="text-2xl font-bold">{stats.aggCount}</p>
                  <p className="text-xs text-muted-foreground">Agregadas</p>
                </div>
              </CardContent>
            </Card>
          </div>
        </div>

        {/* Filters */}
        <div className="flex items-center gap-3">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              placeholder="Buscar por regla, monitor o mensaje..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="pl-9"
            />
          </div>
          <Select value={filterMonitorId} onValueChange={setFilterMonitorId}>
            <SelectTrigger className="w-48">
              <SelectValue placeholder="Monitor" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">Todos los monitores</SelectItem>
              {monitors.map((m) => (
                <SelectItem key={m.id} value={m.id}>{m.name}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select value={filter} onValueChange={setFilter}>
            <SelectTrigger className="w-40">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">Todas ({alerts.length})</SelectItem>
              <SelectItem value="new">Nuevas</SelectItem>
              <SelectItem value="acknowledged">Reconocidas</SelectItem>
              <SelectItem value="resolved">Resueltas</SelectItem>
              <SelectItem value="dismissed">Descartadas</SelectItem>
            </SelectContent>
          </Select>
        </div>

        {/* Date filter banner */}
        {selectedDate && (
          <div className="flex items-center gap-2 rounded-md border border-primary/30 bg-primary/5 px-4 py-2">
            <Calendar className="h-4 w-4 text-primary" />
            <span className="text-sm font-medium">
              Alertas del {selectedDate}
            </span>
            <span className="text-sm text-muted-foreground">
              ({filtered.length} alertas)
            </span>
            <Button
              variant="ghost"
              size="sm"
              className="ml-auto h-6 text-xs"
              onClick={() => setSelectedDate(null)}
            >
              Ver todas
            </Button>
          </div>
        )}

        {/* Alert list */}
        {filtered.length === 0 ? (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12">
              <Bell className="mb-4 h-12 w-12 text-muted-foreground" />
              <p className="mb-2 font-medium">No hay alertas</p>
              <p className="text-sm text-muted-foreground">
                {selectedDate
                  ? `No se encontraron alertas para el ${selectedDate}`
                  : "Las alertas apareceran cuando las reglas se activen"}
              </p>
            </CardContent>
          </Card>
        ) : (
          <div className="space-y-3">
            {filtered.map((alert) => {
              const config = severityConfig[alert.severity];
              const SeverityIcon = config.icon;
              const isExpanded = expandedId === alert.id;
              const isAggregate = alert.alertType === "aggregate";

              return (
                <Card
                  key={alert.id}
                  className={`border-l-4 transition-colors ${config.color}`}
                >
                  <CardContent className="p-4">
                    {/* Header */}
                    <div className="flex items-start justify-between gap-4">
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2 flex-wrap">
                          <SeverityIcon className="h-4 w-4 shrink-0" />
                          <p className="font-semibold">{alert.ruleName}</p>
                          <Badge variant={config.variant}>{config.label}</Badge>
                          <Badge variant="outline">{statusLabels[alert.status]}</Badge>
                          {isAggregate && (
                            <Badge variant="secondary" className="bg-blue-500/10 text-blue-500 gap-1">
                              <Calculator className="h-3 w-3" />
                              Agregada
                            </Badge>
                          )}
                        </div>

                        {/* Aggregate summary */}
                        {isAggregate && alert.aggValue !== undefined ? (
                          <div className="mt-3 rounded-lg border bg-card p-3">
                            <div className="flex items-center justify-between gap-4">
                              <div>
                                <p className="text-xs text-muted-foreground uppercase tracking-wider">
                                  {alert.aggFunction?.toUpperCase()}({alert.aggField}) por {alert.groupByField}
                                </p>
                                <p className="text-lg font-bold text-foreground mt-0.5">
                                  {alert.groupByValue}
                                </p>
                              </div>
                              <div className="text-right">
                                <p className="text-2xl font-bold tabular-nums">
                                  {formatNumber(alert.aggValue)}
                                </p>
                                <p className="text-xs text-muted-foreground">
                                  Umbral: {formatNumber(alert.threshold)}
                                </p>
                              </div>
                            </div>
                            {alert.threshold !== undefined && alert.threshold > 0 && (
                              <div className="mt-2">
                                <div className="h-2 w-full rounded-full bg-muted overflow-hidden">
                                  <div
                                    className="h-full rounded-full bg-red-500 transition-all"
                                    style={{
                                      width: `${Math.min((alert.aggValue! / alert.threshold) * 100, 100)}%`,
                                    }}
                                  />
                                </div>
                                <div className="flex justify-between mt-1">
                                  <span className="text-[10px] text-muted-foreground">0</span>
                                  <span className="text-[10px] text-red-500 font-medium">
                                    {((alert.aggValue! / alert.threshold) * 100).toFixed(0)}% del umbral
                                  </span>
                                  <span className="text-[10px] text-muted-foreground">
                                    {formatNumber(alert.threshold)}
                                  </span>
                                </div>
                              </div>
                            )}
                            {alert.matchCount > 0 && (
                              <p className="text-xs text-muted-foreground mt-2">
                                <Database className="h-3 w-3 inline mr-1" />
                                {alert.matchCount.toLocaleString()} transacciones en el periodo
                              </p>
                            )}
                          </div>
                        ) : (
                          <p className="mt-1 text-sm text-muted-foreground">{alert.message}</p>
                        )}

                        <div className="mt-2 flex items-center gap-3 text-xs text-muted-foreground">
                          <span>Monitor: {alert.monitorName}</span>
                          {!isAggregate && (
                            <span className="flex items-center gap-1">
                              <Database className="h-3 w-3" />
                              {alert.matchCount} coincidencias
                            </span>
                          )}
                          <span>{formatDate(alert.createdAt)}</span>
                        </div>
                      </div>

                      {/* Actions */}
                      <div className="flex gap-1 shrink-0">
                        {alert.status === "new" && (
                          <>
                            <Button
                              variant="ghost"
                              size="icon"
                              onClick={() => openActionDialog(alert, "acknowledged")}
                              title="Reconocer"
                            >
                              <Eye className="h-4 w-4" />
                            </Button>
                            <Button
                              variant="ghost"
                              size="icon"
                              onClick={() => openActionDialog(alert, "resolved")}
                              title="Resolver"
                            >
                              <Check className="h-4 w-4 text-emerald-500" />
                            </Button>
                            <Button
                              variant="ghost"
                              size="icon"
                              onClick={() => openActionDialog(alert, "dismissed")}
                              title="Descartar"
                            >
                              <X className="h-4 w-4 text-muted-foreground" />
                            </Button>
                          </>
                        )}
                        {alert.status === "acknowledged" && (
                          <Button
                            variant="ghost"
                            size="icon"
                            onClick={() => openActionDialog(alert, "resolved")}
                            title="Resolver"
                          >
                            <Check className="h-4 w-4 text-emerald-500" />
                          </Button>
                        )}
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => openLogTimeline(alert)}
                          title="Bitácora"
                        >
                          <ClipboardList className="h-4 w-4 text-primary" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => setExpandedId(isExpanded ? null : alert.id)}
                          title="Ver datos"
                        >
                          {isExpanded ? (
                            <ChevronUp className="h-4 w-4" />
                          ) : (
                            <ChevronDown className="h-4 w-4" />
                          )}
                        </Button>
                      </div>
                    </div>

                    {/* Expanded data table with lazy loading */}
                    {isExpanded && <AlertDataTable alert={alert} />}
                  </CardContent>
                </Card>
              );
            })}
          </div>
        )}
      </div>

      {/* Action Dialog */}
      <Dialog open={!!actionDialog} onOpenChange={(open) => !open && setActionDialog(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>
              {actionDialog?.targetStatus === "acknowledged" && "Reconocer alerta"}
              {actionDialog?.targetStatus === "resolved" && "Resolver alerta"}
              {actionDialog?.targetStatus === "dismissed" && "Descartar alerta"}
            </DialogTitle>
          </DialogHeader>
          {actionDialog && (
            <div className="space-y-4">
              <div className="rounded-md border p-3 text-sm space-y-1">
                <p className="font-medium">{actionDialog.alert.ruleName}</p>
                <p className="text-muted-foreground">{actionDialog.alert.monitorName}</p>
                <div className="flex gap-2 mt-1">
                  <Badge variant={actionDialog.alert.severity === "critical" || actionDialog.alert.severity === "high" ? "destructive" : "secondary"}>
                    {actionDialog.alert.severity}
                  </Badge>
                  <span className="text-xs text-muted-foreground">{actionDialog.alert.matchCount} coincidencias</span>
                </div>
              </div>

              <div className="space-y-2">
                <Label>Categoría</Label>
                <Select value={actionCategory} onValueChange={(v) => setActionCategory(v as ActionCategory)}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="investigation">Investigación</SelectItem>
                    <SelectItem value="false_positive">Falso positivo</SelectItem>
                    <SelectItem value="escalation">Escalado</SelectItem>
                    <SelectItem value="corrective_action">Acción correctiva</SelectItem>
                    <SelectItem value="other">Otro</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label>Notas *</Label>
                <Textarea
                  value={actionNotes}
                  onChange={(e) => setActionNotes(e.target.value)}
                  placeholder="Describe la acción tomada..."
                  rows={3}
                />
              </div>

              <Button onClick={submitAction} disabled={actionLoading || !actionNotes.trim()} className="w-full">
                {actionLoading ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
                Confirmar
              </Button>
            </div>
          )}
        </DialogContent>
      </Dialog>

      {/* Log Timeline Dialog */}
      <Dialog open={!!logDialog} onOpenChange={(open) => !open && setLogDialog(null)}>
        <DialogContent className="max-w-lg max-h-[80vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>Bitácora de alerta</DialogTitle>
          </DialogHeader>
          {logDialog && (
            <div>
              <div className="rounded-md border p-3 text-sm mb-4">
                <p className="font-medium">{logDialog.ruleName}</p>
                <p className="text-muted-foreground text-xs">{logDialog.message}</p>
              </div>

              {logsLoading ? (
                <div className="flex items-center justify-center py-8">
                  <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
                </div>
              ) : alertLogs.length === 0 ? (
                <p className="text-center text-sm text-muted-foreground py-8">
                  No hay acciones registradas para esta alerta
                </p>
              ) : (
                <div className="relative border-l-2 border-border ml-3 space-y-4">
                  {alertLogs.map((log) => {
                    const dotColor =
                      log.newStatus === "resolved" ? "bg-emerald-500" :
                      log.newStatus === "acknowledged" ? "bg-blue-500" :
                      log.newStatus === "dismissed" ? "bg-gray-400" : "bg-yellow-500";
                    const categoryLabels: Record<string, string> = {
                      investigation: "Investigación",
                      false_positive: "Falso positivo",
                      escalation: "Escalado",
                      corrective_action: "Acción correctiva",
                      other: "Otro",
                    };
                    return (
                      <div key={log.id} className="relative pl-6">
                        <div className={`absolute -left-[5px] top-1.5 h-2.5 w-2.5 rounded-full ${dotColor}`} />
                        <div className="text-sm">
                          <div className="flex items-center gap-2 flex-wrap">
                            <span className="font-medium">{log.userName}</span>
                            <Badge variant="outline" className="text-xs">
                              {log.previousStatus} → {log.newStatus}
                            </Badge>
                            <Badge variant="secondary" className="text-xs">
                              {categoryLabels[log.category] || log.category}
                            </Badge>
                          </div>
                          <p className="text-muted-foreground mt-1">{log.notes}</p>
                          <p className="text-xs text-muted-foreground mt-1">{formatDate(log.createdAt)}</p>
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          )}
        </DialogContent>
      </Dialog>
    </>
  );
}
