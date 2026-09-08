"use client";

import { Fragment, useEffect, useMemo, useState } from "react";
import { Header } from "@/components/layout/header";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Upload,
  Calendar,
  Database,
  FileSpreadsheet,
  FileJson,
  FileText,
  Globe,
  Search,
  ArrowUpDown,
  TrendingUp,
  ChevronDown,
  ChevronRight,
  CheckCircle2,
  XCircle,
  AlertTriangle,
} from "lucide-react";
import { api } from "@/lib/api/client";
import { cn } from "@/lib/utils";
import { CATEGORY_CLASSES, STATUS_CLASSES } from "@/lib/semantic-colors";
import { useAuthStore } from "@/stores/auth-store";
import { uploadLogApi } from "@/lib/api/upload-log";
import type { SourceType, UploadLogEntry, UploadStatus } from "@/lib/types";

interface IngestionEntry {
  monitorId: string;
  monitorName: string;
  sourceType: SourceType;
  date: string;
  recordCount: number;
}

const SOURCE_ICONS: Record<SourceType, typeof FileText> = {
  csv: FileText,
  excel: FileSpreadsheet,
  json: FileJson,
  txt: FileText,
  api: Globe,
};

const SOURCE_LABELS: Record<SourceType, string> = {
  csv: "CSV",
  excel: "Excel",
  json: "JSON",
  txt: "TXT",
  api: "API",
};

// Tipo de fuente: eje categórico. Una carga por API no es más peligrosa
// que una por CSV, así que no usa la escala de riesgo.
const SOURCE_COLORS: Record<SourceType, string> = {
  csv: CATEGORY_CLASSES[0],
  excel: CATEGORY_CLASSES[1],
  json: CATEGORY_CLASSES[2],
  txt: CATEGORY_CLASSES[3],
  api: CATEGORY_CLASSES[4],
};

function formatDate(dateStr: string) {
  const [y, m, d] = dateStr.split("-");
  return `${d}/${m}/${y}`;
}

function formatNumber(n: number) {
  return new Intl.NumberFormat("es-PA").format(n);
}

const STATUS_LABELS: Record<UploadStatus, string> = {
  accepted: "Aceptado",
  partial: "Parcial",
  rejected_structure: "Rechazado",
  approved: "Aprobado",
};

const STATUS_ICONS: Record<UploadStatus, typeof CheckCircle2> = {
  accepted: CheckCircle2,
  partial: AlertTriangle,
  rejected_structure: XCircle,
  approved: CheckCircle2,
};

// success/warning/danger/info: eje de estado, no de riesgo — ver CLAUDE.md.
const STATUS_KIND: Record<UploadStatus, "success" | "warning" | "danger" | "info"> = {
  accepted: "success",
  partial: "warning",
  rejected_structure: "danger",
  approved: "info",
};

function formatDateTime(iso: string) {
  return new Date(iso).toLocaleString("es-PA", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

type SortField = "date" | "monitorName" | "recordCount" | "sourceType";
type SortDir = "asc" | "desc";

export default function UploadsPage() {
  const [entries, setEntries] = useState<IngestionEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  // Filters
  const [search, setSearch] = useState("");
  const [sourceFilter, setSourceFilter] = useState<string>("all");
  const [dateFrom, setDateFrom] = useState("");
  const [dateTo, setDateTo] = useState("");

  // Sort
  const [sortField, setSortField] = useState<SortField>("date");
  const [sortDir, setSortDir] = useState<SortDir>("desc");

  // Bitácora detallada
  const [logEntries, setLogEntries] = useState<UploadLogEntry[]>([]);
  const [monitorFilter, setMonitorFilter] = useState<string>("all");
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [approvingId, setApprovingId] = useState<string | null>(null);
  const user = useAuthStore((s) => s.user);
  const canApprove = user != null && user.role !== "viewer";

  useEffect(() => {
    async function load() {
      try {
        const [ingestion, log] = await Promise.all([
          api.get<IngestionEntry[]>("/ingestion-history"),
          uploadLogApi.list(),
        ]);
        setEntries(ingestion);
        setLogEntries(log);
      } catch {
        setError("No se pudo cargar el historial de cargas");
      } finally {
        setLoading(false);
      }
    }
    load();
  }, []);

  // Unique source types for filter dropdown
  const sourceTypes = useMemo(() => {
    const types = new Set(entries.map((e) => e.sourceType));
    return Array.from(types).sort();
  }, [entries]);

  // Monitores únicos, derivados de la bitácora, para el filtro
  const logMonitors = useMemo(() => {
    const map = new Map<string, string>();
    for (const e of logEntries) map.set(e.monitorId, e.monitorName);
    return Array.from(map.entries()).sort((a, b) => a[1].localeCompare(b[1]));
  }, [logEntries]);

  const filteredLog = useMemo(() => {
    let result = logEntries;
    if (monitorFilter !== "all") {
      result = result.filter((e) => e.monitorId === monitorFilter);
    }
    if (statusFilter !== "all") {
      result = result.filter((e) => e.status === statusFilter);
    }
    if (search) {
      const q = search.toLowerCase();
      result = result.filter(
        (e) =>
          e.monitorName.toLowerCase().includes(q) ||
          e.fileName.toLowerCase().includes(q),
      );
    }
    return [...result].sort(
      (a, b) => new Date(b.uploadedAt).getTime() - new Date(a.uploadedAt).getTime(),
    );
  }, [logEntries, monitorFilter, statusFilter, search]);

  // Filtered and sorted entries
  const filtered = useMemo(() => {
    let result = entries;

    // Text search (monitor name)
    if (search) {
      const q = search.toLowerCase();
      result = result.filter((e) => e.monitorName.toLowerCase().includes(q));
    }

    // Source type filter
    if (sourceFilter !== "all") {
      result = result.filter((e) => e.sourceType === sourceFilter);
    }

    // Date range
    if (dateFrom) {
      result = result.filter((e) => e.date >= dateFrom);
    }
    if (dateTo) {
      result = result.filter((e) => e.date <= dateTo);
    }

    // Sort
    result = [...result].sort((a, b) => {
      let cmp = 0;
      switch (sortField) {
        case "date":
          cmp = a.date.localeCompare(b.date);
          break;
        case "monitorName":
          cmp = a.monitorName.localeCompare(b.monitorName);
          break;
        case "recordCount":
          cmp = a.recordCount - b.recordCount;
          break;
        case "sourceType":
          cmp = a.sourceType.localeCompare(b.sourceType);
          break;
      }
      return sortDir === "desc" ? -cmp : cmp;
    });

    return result;
  }, [entries, search, sourceFilter, dateFrom, dateTo, sortField, sortDir]);

  // Summary stats
  const stats = useMemo(() => {
    const totalRecords = filtered.reduce((s, e) => s + e.recordCount, 0);
    const uniqueDates = new Set(filtered.map((e) => e.date)).size;
    const uniqueMonitors = new Set(filtered.map((e) => e.monitorId)).size;
    return { totalRecords, uniqueDates, uniqueMonitors, totalEntries: filtered.length };
  }, [filtered]);

  function toggleSort(field: SortField) {
    if (sortField === field) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortField(field);
      setSortDir("desc");
    }
  }

  async function handleApprove(entry: UploadLogEntry) {
    setApprovingId(entry.id);
    try {
      await uploadLogApi.approve(entry.monitorId, entry.id);
      const log = await uploadLogApi.list();
      setLogEntries(log);
    } catch {
      setError("No se pudo aprobar la entrada");
    } finally {
      setApprovingId(null);
    }
  }

  if (error) {
    return (
      <>
        <Header title="Historial de cargas" />
        <div className="p-6">
          <p className="text-sm text-danger-fg">{error}</p>
        </div>
      </>
    );
  }

  return (
    <>
      <Header title="Historial de cargas" />
      <div className="p-6">
        {/* Summary cards */}
        <div className="mb-6 grid grid-cols-2 gap-4 sm:grid-cols-4">
          <Card>
            <CardContent className="flex items-center gap-3 p-4">
              <div className={cn("rounded-lg p-2", CATEGORY_CLASSES[0])}>
                <Upload className="h-4 w-4" />
              </div>
              <div>
                <p className="text-2xl font-bold">{stats.totalEntries}</p>
                <p className="text-xs text-muted-foreground">Cargas</p>
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="flex items-center gap-3 p-4">
              <div className={cn("rounded-lg p-2", CATEGORY_CLASSES[1])}>
                <Database className="h-4 w-4" />
              </div>
              <div>
                <p className="text-2xl font-bold">{formatNumber(stats.totalRecords)}</p>
                <p className="text-xs text-muted-foreground">Registros</p>
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="flex items-center gap-3 p-4">
              <div className={cn("rounded-lg p-2", CATEGORY_CLASSES[2])}>
                <Calendar className="h-4 w-4" />
              </div>
              <div>
                <p className="text-2xl font-bold">{stats.uniqueDates}</p>
                <p className="text-xs text-muted-foreground">Días</p>
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="flex items-center gap-3 p-4">
              <div className={cn("rounded-lg p-2", CATEGORY_CLASSES[3])}>
                <TrendingUp className="h-4 w-4" />
              </div>
              <div>
                <p className="text-2xl font-bold">{stats.uniqueMonitors}</p>
                <p className="text-xs text-muted-foreground">Monitores</p>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Filters */}
        <div className="mb-4 flex flex-wrap items-center gap-3">
          <div className="relative flex-1 min-w-[200px]">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              placeholder="Buscar por monitor..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-9"
            />
          </div>
          <Select value={sourceFilter} onValueChange={setSourceFilter}>
            <SelectTrigger className="w-[160px]">
              <SelectValue placeholder="Tipo" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">Todos los tipos</SelectItem>
              {sourceTypes.map((t) => (
                <SelectItem key={t} value={t}>
                  {SOURCE_LABELS[t]}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select value={monitorFilter} onValueChange={setMonitorFilter}>
            <SelectTrigger className="w-[180px]">
              <SelectValue placeholder="Monitor" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">Todos los monitores</SelectItem>
              {logMonitors.map(([id, name]) => (
                <SelectItem key={id} value={id}>
                  {name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select value={statusFilter} onValueChange={setStatusFilter}>
            <SelectTrigger className="w-[160px]">
              <SelectValue placeholder="Estado" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">Todos los estados</SelectItem>
              <SelectItem value="accepted">Aceptado</SelectItem>
              <SelectItem value="partial">Parcial</SelectItem>
              <SelectItem value="rejected_structure">Rechazado</SelectItem>
              <SelectItem value="approved">Aprobado</SelectItem>
            </SelectContent>
          </Select>
          <Input
            type="date"
            value={dateFrom}
            onChange={(e) => setDateFrom(e.target.value)}
            className="w-[160px]"
            placeholder="Desde"
          />
          <Input
            type="date"
            value={dateTo}
            onChange={(e) => setDateTo(e.target.value)}
            className="w-[160px]"
            placeholder="Hasta"
          />
        </div>

        {/* Grid table */}
        {loading ? (
          <div className="flex items-center justify-center py-12">
            <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
          </div>
        ) : filtered.length === 0 ? (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12">
              <Upload className="mb-3 h-10 w-10 text-muted-foreground/40" />
              <p className="text-sm text-muted-foreground">
                {entries.length === 0
                  ? "No hay cargas registradas"
                  : "No se encontraron cargas con los filtros aplicados"}
              </p>
            </CardContent>
          </Card>
        ) : (
          <Card>
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead>
                  <tr className="border-b bg-muted/50">
                    <th className="px-4 py-3 text-left">
                      <button
                        onClick={() => toggleSort("date")}
                        className="flex items-center gap-1 text-xs font-medium uppercase tracking-wider text-muted-foreground hover:text-foreground"
                      >
                        Fecha
                        <ArrowUpDown className="h-3 w-3" />
                      </button>
                    </th>
                    <th className="px-4 py-3 text-left">
                      <button
                        onClick={() => toggleSort("monitorName")}
                        className="flex items-center gap-1 text-xs font-medium uppercase tracking-wider text-muted-foreground hover:text-foreground"
                      >
                        Monitor
                        <ArrowUpDown className="h-3 w-3" />
                      </button>
                    </th>
                    <th className="px-4 py-3 text-left">
                      <button
                        onClick={() => toggleSort("sourceType")}
                        className="flex items-center gap-1 text-xs font-medium uppercase tracking-wider text-muted-foreground hover:text-foreground"
                      >
                        Tipo
                        <ArrowUpDown className="h-3 w-3" />
                      </button>
                    </th>
                    <th className="px-4 py-3 text-right">
                      <button
                        onClick={() => toggleSort("recordCount")}
                        className="ml-auto flex items-center gap-1 text-xs font-medium uppercase tracking-wider text-muted-foreground hover:text-foreground"
                      >
                        Registros
                        <ArrowUpDown className="h-3 w-3" />
                      </button>
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {filtered.map((entry, i) => {
                    const Icon = SOURCE_ICONS[entry.sourceType];
                    return (
                      <tr
                        key={`${entry.monitorId}-${entry.date}-${i}`}
                        className="border-b transition-colors hover:bg-muted/30 last:border-0"
                      >
                        <td className="px-4 py-3">
                          <div className="flex items-center gap-2">
                            <Calendar className="h-4 w-4 text-muted-foreground" />
                            <span className="text-sm font-medium">
                              {formatDate(entry.date)}
                            </span>
                          </div>
                        </td>
                        <td className="px-4 py-3">
                          <a
                            href={`/monitors/${entry.monitorId}`}
                            className="text-sm font-medium text-primary hover:underline"
                          >
                            {entry.monitorName}
                          </a>
                        </td>
                        <td className="px-4 py-3">
                          <Badge
                            variant="outline"
                            className={`gap-1 ${SOURCE_COLORS[entry.sourceType]}`}
                          >
                            <Icon className="h-3 w-3" />
                            {SOURCE_LABELS[entry.sourceType]}
                          </Badge>
                        </td>
                        <td className="px-4 py-3 text-right">
                          <span className="text-sm font-semibold tabular-nums">
                            {formatNumber(entry.recordCount)}
                          </span>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </Card>
        )}

        {/* Bitácora detallada por archivo */}
        <div className="mt-8">
          <h2 className="mb-3 text-sm font-semibold text-muted-foreground">
            Bitácora de archivos subidos
          </h2>
          {filteredLog.length === 0 ? (
            <Card>
              <CardContent className="flex flex-col items-center justify-center py-12">
                <Upload className="mb-3 h-10 w-10 text-muted-foreground/40" />
                <p className="text-sm text-muted-foreground">
                  No hay archivos en la bitácora con los filtros aplicados
                </p>
              </CardContent>
            </Card>
          ) : (
            <Card>
              <div className="overflow-x-auto">
                <table className="w-full">
                  <thead>
                    <tr className="border-b bg-muted/50">
                      <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground">
                        Archivo
                      </th>
                      <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground">
                        Monitor
                      </th>
                      <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground">
                        Fecha/hora
                      </th>
                      <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground">
                        Subido por
                      </th>
                      <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground">
                        Estado
                      </th>
                      <th className="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-muted-foreground">
                        Totales
                      </th>
                      <th className="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-muted-foreground">
                        Aceptados
                      </th>
                      <th className="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-muted-foreground">
                        Rechazados
                      </th>
                      <th className="px-4 py-3" />
                    </tr>
                  </thead>
                  <tbody>
                    {filteredLog.map((entry) => {
                      const StatusIcon = STATUS_ICONS[entry.status];
                      const expanded = expandedId === entry.id;
                      const hasDetail =
                        (entry.schemaDiff && !entry.schemaDiff.match) ||
                        (entry.rowRejections && entry.rowRejections.length > 0);
                      return (
                        <Fragment key={entry.id}>
                          <tr className="border-b transition-colors hover:bg-muted/30 last:border-0">
                            <td className="px-4 py-3">
                              <div className="flex items-center gap-2">
                                {hasDetail && (
                                  <button
                                    onClick={() => setExpandedId(expanded ? null : entry.id)}
                                    className="text-muted-foreground hover:text-foreground"
                                  >
                                    {expanded ? (
                                      <ChevronDown className="h-4 w-4" />
                                    ) : (
                                      <ChevronRight className="h-4 w-4" />
                                    )}
                                  </button>
                                )}
                                <span className="text-sm font-medium">{entry.fileName}</span>
                              </div>
                            </td>
                            <td className="px-4 py-3">
                              <a
                                href={`/monitors/${entry.monitorId}`}
                                className="text-sm text-primary hover:underline"
                              >
                                {entry.monitorName}
                              </a>
                            </td>
                            <td className="px-4 py-3 text-sm text-muted-foreground">
                              {formatDateTime(entry.uploadedAt)}
                            </td>
                            <td className="px-4 py-3 text-sm text-muted-foreground">
                              {entry.uploadedByEmail}
                            </td>
                            <td className="px-4 py-3">
                              <Badge
                                variant="outline"
                                className={cn("gap-1", STATUS_CLASSES[STATUS_KIND[entry.status]])}
                              >
                                <StatusIcon className="h-3 w-3" />
                                {STATUS_LABELS[entry.status]}
                              </Badge>
                            </td>
                            <td className="px-4 py-3 text-right text-sm tabular-nums">
                              {formatNumber(entry.totalRows)}
                            </td>
                            <td className="px-4 py-3 text-right text-sm tabular-nums text-success-fg">
                              {formatNumber(entry.rowsAccepted)}
                            </td>
                            <td className="px-4 py-3 text-right text-sm tabular-nums text-danger-fg">
                              {formatNumber(entry.rowsRejected)}
                            </td>
                            <td className="px-4 py-3 text-right">
                              {entry.status === "rejected_structure" && canApprove && (
                                <button
                                  onClick={() => handleApprove(entry)}
                                  disabled={approvingId === entry.id}
                                  className="rounded-md border px-2 py-1 text-xs font-medium hover:bg-accent disabled:opacity-50"
                                >
                                  {approvingId === entry.id ? "Aprobando…" : "Aprobar"}
                                </button>
                              )}
                            </td>
                          </tr>
                          {expanded && hasDetail && (
                            <tr className="border-b bg-muted/20 last:border-0">
                              <td colSpan={9} className="px-4 py-3">
                                {entry.schemaDiff && !entry.schemaDiff.match && (
                                  <div className="mb-2 space-y-1 text-xs">
                                    {entry.schemaDiff.missingFields && entry.schemaDiff.missingFields.length > 0 && (
                                      <p>
                                        <span className="font-medium">Faltan columnas: </span>
                                        {entry.schemaDiff.missingFields.join(", ")}
                                      </p>
                                    )}
                                    {entry.schemaDiff.extraFields && entry.schemaDiff.extraFields.length > 0 && (
                                      <p>
                                        <span className="font-medium">Columnas de más: </span>
                                        {entry.schemaDiff.extraFields.join(", ")}
                                      </p>
                                    )}
                                    {entry.schemaDiff.typeMismatches && entry.schemaDiff.typeMismatches.length > 0 && (
                                      <p>
                                        <span className="font-medium">Tipo distinto: </span>
                                        {entry.schemaDiff.typeMismatches
                                          .map((m) => `${m.field} (esperado ${m.expectedType}, encontrado ${m.actualType})`)
                                          .join(", ")}
                                      </p>
                                    )}
                                  </div>
                                )}
                                {entry.rowRejections && entry.rowRejections.length > 0 && (
                                  <ul className="space-y-0.5 text-xs text-muted-foreground">
                                    {entry.rowRejections.map((r, i) => (
                                      <li key={i}>
                                        Fila {r.rowIndex}: {r.field} — {r.reason}
                                      </li>
                                    ))}
                                  </ul>
                                )}
                              </td>
                            </tr>
                          )}
                        </Fragment>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </Card>
          )}
        </div>
      </div>
    </>
  );
}
