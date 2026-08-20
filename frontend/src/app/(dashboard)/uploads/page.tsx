"use client";

import { useEffect, useMemo, useState } from "react";
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
} from "lucide-react";
import { api } from "@/lib/api/client";
import { cn } from "@/lib/utils";
import { CATEGORY_CLASSES } from "@/lib/semantic-colors";
import type { SourceType } from "@/lib/types";

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
  api: Globe,
};

const SOURCE_LABELS: Record<SourceType, string> = {
  csv: "CSV",
  excel: "Excel",
  json: "JSON",
  api: "API",
};

// Tipo de fuente: eje categórico. Una carga por API no es más peligrosa
// que una por CSV, así que no usa la escala de riesgo.
const SOURCE_COLORS: Record<SourceType, string> = {
  csv: CATEGORY_CLASSES[0],
  excel: CATEGORY_CLASSES[1],
  json: CATEGORY_CLASSES[2],
  api: CATEGORY_CLASSES[3],
};

function formatDate(dateStr: string) {
  const [y, m, d] = dateStr.split("-");
  return `${d}/${m}/${y}`;
}

function formatNumber(n: number) {
  return new Intl.NumberFormat("es-PA").format(n);
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

  useEffect(() => {
    async function load() {
      try {
        const data = await api.get<IngestionEntry[]>("/ingestion-history");
        setEntries(data);
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
                <p className="text-xs text-muted-foreground">Dias</p>
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
      </div>
    </>
  );
}
