"use client";

import { Suspense, useEffect, useMemo, useState } from "react";
import { useSearchParams } from "next/navigation";
import { Header } from "@/components/layout/header";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
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
  Plus,
  Sparkles,
  Trash2,
  ShieldCheck,
  Pencil,
  Calculator,
  Search,
  X,
  CreditCard,
  BarChart3,
  Play,
  Clock,
  Loader2,
  CircleCheck,
  CircleAlert,
} from "lucide-react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { rulesApi, type BacktestResult } from "@/lib/api/rules";
import { monitorsApi } from "@/lib/api/monitors";
import { dashboardsApi } from "@/lib/api/dashboards";
import type { Dashboard, WidgetType } from "@/lib/types";
import { mccApi } from "@/lib/api/mcc";
import { useToast } from "@/lib/use-toast";
import { TemplateGallery } from "@/components/rules/template-gallery";
import { EffectivenessTable } from "@/components/rules/effectiveness-table";
import { formatDate, cn } from "@/lib/utils";
import { RISK_CLASSES } from "@/lib/semantic-colors";
import type {
  Rule,
  Monitor,
  AIRuleSuggestion,
  Severity,
  Condition,
  Operator,
  AggregateCondition,
  AggFunction,
  MCC,
  SchedulePreset,
  RuleSchedule,
} from "@/lib/types";

function wasTriggeredToday(lastTriggered?: string): boolean {
  if (!lastTriggered) return false;
  const today = new Date().toISOString().slice(0, 10);
  return lastTriggered.slice(0, 10) === today;
}

const OPERATOR_SYMBOLS: Record<string, string> = {
  gt: ">",
  gte: "≥",
  lt: "<",
  lte: "≤",
  eq: "=",
  starts_with: "A…",
  ends_with: "…Z",
};

function operatorSymbol(op: string): string {
  return OPERATOR_SYMBOLS[op] ?? "=";
}

const SEVERITY_VARIANTS: Record<
  string,
  "destructive" | "warning" | "secondary"
> = {
  critical: "destructive",
  high: "destructive",
  medium: "warning",
  low: "secondary",
};

function severityVariant(
  severity: string,
): "destructive" | "warning" | "secondary" {
  return SEVERITY_VARIANTS[severity] ?? "secondary";
}

function isMCCField(field: string): boolean {
  return /mcc/i.test(field);
}

// --- Schedule helpers ---
const TIME_OPTIONS = Array.from({ length: 48 }, (_, i) => {
  const h = Math.floor(i / 2);
  const m = i % 2 === 0 ? "00" : "30";
  return `${String(h).padStart(2, "0")}:${m}`;
});

const DAY_LABELS: Record<string, string> = {
  "1": "Lunes",
  "2": "Martes",
  "3": "Miércoles",
  "4": "Jueves",
  "5": "Viernes",
  "6": "Sábado",
  "0": "Domingo",
};

const FREQ_LABELS: Record<string, string> = {
  daily: "Diario",
  weekly: "Semanal",
  monthly: "Mensual",
};

function buildCronExpr(freq: string, day: string, time: string): string {
  const [h, m] = time.split(":").map(Number);
  if (freq === "daily") return `${m} ${h} * * *`;
  if (freq === "weekly") return `${m} ${h} * * ${day}`;
  if (freq === "monthly") return `${m} ${h} ${day} * *`;
  return "";
}

function parseCronToSchedule(cronExpr: string): {
  frequency: string;
  day: string;
  time: string;
} {
  const parts = cronExpr.split(" ");
  if (parts.length !== 5)
    return { frequency: "daily", day: "1", time: "06:00" };
  const [min, hour, dom, , dow] = parts;
  const time = `${hour.padStart(2, "0")}:${min.padStart(2, "0")}`;
  if (dow !== "*") return { frequency: "weekly", day: dow, time };
  if (dom !== "*") return { frequency: "monthly", day: dom, time };
  return { frequency: "daily", day: "1", time };
}

function scheduleLabel(schedule?: RuleSchedule): string {
  if (!schedule?.enabled) return "";
  const cron = schedule.cronExpr || presetToCronFallback(schedule.preset);
  if (!cron) return schedule.preset || "";
  const s = parseCronToSchedule(cron);
  const freq = FREQ_LABELS[s.frequency] || s.frequency;
  if (s.frequency === "weekly")
    return `${DAY_LABELS[s.day] || "Dia " + s.day} ${s.time}`;
  if (s.frequency === "monthly") return `Dia ${s.day} ${s.time}`;
  return `${freq} ${s.time}`;
}

function presetToCronFallback(preset: string): string {
  const map: Record<string, string> = {
    daily_6am: "0 6 * * *",
    daily_8am: "0 8 * * *",
    daily_12pm: "0 12 * * *",
    daily_6pm: "0 18 * * *",
    weekly_mon_8am: "0 8 * * 1",
    weekly_fri_6pm: "0 18 * * 5",
    monthly_1st_6am: "0 6 1 * *",
  };
  return map[preset] || "";
}

// MCC Picker component
function MCCPicker({
  value,
  onChange,
  multi,
}: {
  value: unknown;
  onChange: (val: string) => void;
  multi?: boolean;
}) {
  const [mccs, setMccs] = useState<MCC[]>([]);
  const [categories, setCategories] = useState<string[]>([]);
  const [search, setSearch] = useState("");
  const [selectedCat, setSelectedCat] = useState("");
  const [isOpen, setIsOpen] = useState(false);
  const [loading, setLoading] = useState(false);

  // Parse current value to array of selected codes
  const selectedCodes: string[] = (() => {
    if (!value) return [];
    const v = String(value);
    if (!v) return [];
    return v
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean);
  })();

  useEffect(() => {
    if (!isOpen) return;
    // debt: setLoading síncrono al abrir el modal — el spinner debe
    // aparecer en el mismo frame de la apertura (loading arranca en false).
    // Revisar -> al migrar la carga de MCC a TanStack Query.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setLoading(true);
    Promise.all([mccApi.categories(), mccApi.list()])
      .then(([cats, all]) => {
        setCategories(cats);
        setMccs(all);
      })
      .catch((err) => {
        console.error("Failed to load MCC data:", err);
      })
      .finally(() => setLoading(false));
  }, [isOpen]);

  const filtered = mccs.filter((m) => {
    if (selectedCat && m.category !== selectedCat) return false;
    if (search) {
      const q = search.toLowerCase();
      return m.code.includes(q) || m.description.toLowerCase().includes(q);
    }
    return true;
  });

  function toggleCode(code: string) {
    if (multi) {
      const next = selectedCodes.includes(code)
        ? selectedCodes.filter((c) => c !== code)
        : [...selectedCodes, code];
      onChange(next.join(","));
    } else {
      onChange(code);
      setIsOpen(false);
    }
  }

  function removeCode(code: string) {
    onChange(selectedCodes.filter((c) => c !== code).join(","));
  }

  const riskColors: Record<string, string> = {
    high: RISK_CLASSES.high,
    medium: RISK_CLASSES.medium,
    low: RISK_CLASSES.low,
  };
  const riskColor = (level: string) => riskColors[level] ?? riskColors.low;

  return (
    <div className="relative">
      <div
        className="flex min-h-[36px] cursor-pointer flex-wrap items-center gap-1 rounded-md border bg-background px-2 py-1 text-xs"
        onClick={() => setIsOpen(!isOpen)}
      >
        {selectedCodes.length === 0 ? (
          <span className="text-muted-foreground flex items-center gap-1">
            <CreditCard className="h-3 w-3" /> Seleccionar MCCs...
          </span>
        ) : (
          selectedCodes.map((code) => {
            const m = mccs.find((x) => x.code === code);
            return (
              <span
                key={code}
                className="inline-flex items-center gap-1 rounded bg-primary/10 px-1.5 py-0.5 text-[10px]"
              >
                {code}
                {m ? ` - ${m.description.slice(0, 20)}` : ""}
                <X
                  className="h-3 w-3 cursor-pointer hover:text-destructive"
                  onClick={(e) => {
                    e.stopPropagation();
                    removeCode(code);
                  }}
                />
              </span>
            );
          })
        )}
      </div>

      {isOpen && (
        <div className="absolute right-0 z-50 mt-1 w-[480px] rounded-md border bg-popover p-3 shadow-lg">
          <div className="flex gap-2 mb-2">
            <div className="relative flex-1">
              <Search className="absolute left-2 top-1/2 h-3 w-3 -translate-y-1/2 text-muted-foreground" />
              <Input
                className="h-7 pl-7 text-xs"
                placeholder="Buscar código o descripción..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                autoFocus
              />
            </div>
          </div>
          <div className="flex gap-1 mb-2 flex-wrap">
            <button
              type="button"
              className={cn(
                "rounded px-2 py-0.5 text-[10px] border transition-colors",
                !selectedCat
                  ? "bg-primary text-primary-foreground"
                  : "hover:bg-accent",
              )}
              onClick={() => setSelectedCat("")}
            >
              Todas
            </button>
            {categories.map((c) => (
              <button
                key={c}
                type="button"
                className={cn(
                  "rounded px-2 py-0.5 text-[10px] border transition-colors",
                  selectedCat === c
                    ? "bg-primary text-primary-foreground"
                    : "hover:bg-accent",
                )}
                onClick={() => setSelectedCat(selectedCat === c ? "" : c)}
              >
                {c}
              </button>
            ))}
          </div>

          {loading ? (
            <p className="py-4 text-center text-xs text-muted-foreground">
              Cargando MCCs...
            </p>
          ) : (
            <div className="max-h-[240px] overflow-y-auto space-y-0.5">
              {filtered.length === 0 ? (
                <p className="py-4 text-center text-xs text-muted-foreground">
                  Sin resultados
                </p>
              ) : (
                filtered.map((m) => (
                  <div
                    key={m.code}
                    className={cn(
                      "flex cursor-pointer items-center justify-between rounded px-2 py-1 text-xs hover:bg-accent",
                      selectedCodes.includes(m.code) &&
                        "bg-primary/10 font-medium",
                    )}
                    onClick={() => toggleCode(m.code)}
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      <span className="font-mono text-[11px] shrink-0">
                        {m.code}
                      </span>
                      <span className="truncate">{m.description}</span>
                    </div>
                    <div className="flex items-center gap-1 shrink-0 ml-2">
                      <span
                        className={cn(
                          "rounded px-1 py-0.5 text-[9px] font-medium",
                          riskColor(m.riskLevel),
                        )}
                      >
                        {m.riskLevel === "high"
                          ? "Alto"
                          : m.riskLevel === "medium"
                            ? "Medio"
                            : "Bajo"}
                      </span>
                      {selectedCodes.includes(m.code) && (
                        <span className="text-primary">✓</span>
                      )}
                    </div>
                  </div>
                ))
              )}
            </div>
          )}

          <div className="mt-2 flex items-center justify-between border-t pt-2">
            <span className="text-[10px] text-muted-foreground">
              {selectedCodes.length} seleccionado(s) / {filtered.length}{" "}
              mostrados
            </span>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-6 text-xs"
              onClick={() => setIsOpen(false)}
            >
              Cerrar
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}

export default function RulesPage() {
  return (
    <Suspense
      fallback={
        <>
          <Header title="Reglas" />
          <div className="p-6">
            <p className="text-sm text-muted-foreground">Cargando...</p>
          </div>
        </>
      }
    >
      <RulesContent />
    </Suspense>
  );
}

function RulesContent() {
  const searchParams = useSearchParams();
  const monitorIdParam = searchParams.get("monitorId");

  const [rules, setRules] = useState<Rule[]>([]);
  const [monitors, setMonitors] = useState<Monitor[]>([]);
  const [filterMonitorId, setFilterMonitorId] = useState(
    monitorIdParam || "all",
  );
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [isTemplatesOpen, setIsTemplatesOpen] = useState(false);
  const [isAIOpen, setIsAIOpen] = useState(false);
  const [aiLoading, setAILoading] = useState(false);
  const [aiSuggestions, setAiSuggestions] = useState<AIRuleSuggestion[]>([]);
  const [aiNoResults, setAiNoResults] = useState(false);
  const [selectedMonitor, setSelectedMonitor] = useState(monitorIdParam || "");
  const [aiPrompt, setAiPrompt] = useState("");
  const [editingRule, setEditingRule] = useState<Rule | null>(null);
  const [editForm, setEditForm] = useState({
    name: "",
    description: "",
    severity: "medium" as Severity,
    logic: "AND" as "AND" | "OR",
    conditions: [] as Condition[],
    aggregateConditions: [] as AggregateCondition[],
    scheduleEnabled: false,
    scheduleFrequency: "daily",
    scheduleDay: "1",
    scheduleTime: "06:00",
    screeningFields: [] as string[],
  });
  const [activeTab, setActiveTab] = useState("daily");
  const [executingRuleId, setExecutingRuleId] = useState<string | null>(null);
  const [executingAll, setExecutingAll] = useState(false);
  const [executeAllProgress, setExecuteAllProgress] = useState({
    current: 0,
    total: 0,
    redFlags: 0,
  });
  const { toastError, toastSuccess } = useToast();

  // Create chart from rule state
  const [chartDialogRule, setChartDialogRule] = useState<Rule | null>(null);
  const [dashboards, setDashboards] = useState<Dashboard[]>([]);
  const [chartForm, setChartForm] = useState({
    dashboardId: "",
    chartType: "bar_chart" as WidgetType,
    title: "",
  });
  const [chartCreating, setChartCreating] = useState(false);
  const [deleteConfirm, setDeleteConfirm] = useState<{
    rule: Rule;
    input: string;
  } | null>(null);

  // Create form (same structure as edit)
  const [createForm, setCreateForm] = useState({
    name: "",
    description: "",
    monitorId: monitorIdParam || "",
    severity: "medium" as Severity,
    logic: "AND" as "AND" | "OR",
    conditions: [
      { field: "", operator: "gt" as Operator, value: "" as unknown },
    ] as Condition[],
    aggregateConditions: [] as AggregateCondition[],
    scheduleEnabled: false,
    scheduleFrequency: "daily",
    scheduleDay: "1",
    scheduleTime: "06:00",
    fatfTypology: "",
    thresholdJustification: "",
    regulatoryBasis: "",
    screeningFields: [] as string[],
  });
  const [backtesting, setBacktesting] = useState(false);
  const [backtestResult, setBacktestResult] = useState<BacktestResult | null>(
    null,
  );

  useEffect(() => {
    loadRules();
    loadMonitors();
  }, []);

  async function runBacktest(
    monitorId: string,
    logic: "AND" | "OR",
    conditions: Condition[],
    aggregateConditions: AggregateCondition[],
    severity: Severity,
  ) {
    if (!monitorId) {
      toastError("Elegí un monitor primero");
      return;
    }
    setBacktesting(true);
    setBacktestResult(null);
    try {
      const result = await rulesApi.backtest(monitorId, {
        conditionGroup: { logic, conditions },
        aggregateConditions:
          aggregateConditions.length > 0 ? aggregateConditions : undefined,
        severity,
      });
      setBacktestResult(result);
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Error al probar la regla");
    } finally {
      setBacktesting(false);
    }
  }

  async function loadRules() {
    try {
      const data = await rulesApi.list(monitorIdParam || undefined);
      setRules(data);
    } catch (err) {
      console.error("Failed to load rules:", err);
      toastError(err instanceof Error ? err.message : "Error al cargar reglas");
    }
  }

  async function loadMonitors() {
    try {
      setMonitors(await monitorsApi.list());
    } catch (err) {
      console.error("Failed to load monitors:", err);
      toastError(
        err instanceof Error ? err.message : "Error al cargar monitores",
      );
    }
  }

  async function openChartDialog(rule: Rule) {
    setChartDialogRule(rule);
    setChartForm({
      dashboardId: "",
      chartType: "bar_chart",
      title: `Regla: ${rule.name}`,
    });
    try {
      setDashboards(await dashboardsApi.list());
    } catch (err) {
      console.error("Failed to load dashboards:", err);
    }
  }

  async function createChartFromRule() {
    if (!chartDialogRule || !chartForm.dashboardId) return;
    setChartCreating(true);
    try {
      await dashboardsApi.addWidget(chartForm.dashboardId, {
        title: chartForm.title,
        type: chartForm.chartType,
        ruleId: chartDialogRule.id,
        position: { x: 0, y: 0, w: 6, h: 4 },
      });
      setChartDialogRule(null);
    } catch (err) {
      console.error("Failed to create chart:", err);
      toastError(err instanceof Error ? err.message : "Error al crear gráfico");
    } finally {
      setChartCreating(false);
    }
  }

  async function createRule(e: React.FormEvent) {
    e.preventDefault();
    try {
      await rulesApi.create({
        monitorId: createForm.monitorId,
        name: createForm.name,
        description: createForm.description,
        conditionGroup: {
          logic: createForm.logic,
          conditions: createForm.conditions,
        },
        aggregateConditions:
          createForm.aggregateConditions.length > 0
            ? createForm.aggregateConditions
            : undefined,
        actions: ["red_flag"],
        severity: createForm.severity,
        schedule: createForm.scheduleEnabled
          ? {
              enabled: true,
              preset: "" as SchedulePreset,
              cronExpr: buildCronExpr(
                createForm.scheduleFrequency,
                createForm.scheduleDay,
                createForm.scheduleTime,
              ),
            }
          : undefined,
        fatfTypology: createForm.fatfTypology || undefined,
        thresholdJustification: createForm.thresholdJustification || undefined,
        regulatoryBasis: createForm.regulatoryBasis || undefined,
        screeningFields: createForm.screeningFields.length > 0 ? createForm.screeningFields : undefined,
      });
      setIsCreateOpen(false);
      setBacktestResult(null);
      setCreateForm({
        name: "",
        description: "",
        monitorId: monitorIdParam || "",
        severity: "medium",
        logic: "AND",
        conditions: [{ field: "", operator: "gt" as Operator, value: "" }],
        aggregateConditions: [],
        scheduleEnabled: false,
        scheduleFrequency: "daily",
        scheduleDay: "1",
        scheduleTime: "06:00",
        fatfTypology: "",
        thresholdJustification: "",
        regulatoryBasis: "",
        screeningFields: [],
      });
      loadRules();
    } catch (err) {
      console.error("Failed to create rule:", err);
      toastError(err instanceof Error ? err.message : "Error al crear regla");
    }
  }

  // Create form helpers
  function updateCreateCondition(
    index: number,
    field: keyof Condition,
    value: unknown,
  ) {
    setCreateForm((prev) => {
      const conditions = [...prev.conditions];
      conditions[index] = { ...conditions[index], [field]: value };
      return { ...prev, conditions };
    });
  }
  function removeCreateCondition(index: number) {
    setCreateForm((prev) => ({
      ...prev,
      conditions: prev.conditions.filter((_, i) => i !== index),
    }));
  }
  function addCreateCondition() {
    setCreateForm((prev) => ({
      ...prev,
      conditions: [
        ...prev.conditions,
        { field: "", operator: "gt" as Operator, value: "" },
      ],
    }));
  }
  function addCreateAggCondition() {
    setCreateForm((prev) => ({
      ...prev,
      aggregateConditions: [
        ...prev.aggregateConditions,
        {
          field: "",
          function: "sum" as AggFunction,
          groupBy: "",
          timeField: "",
          timeWindow: "30d",
          operator: "gt" as Operator,
          threshold: 0,
        },
      ],
    }));
  }
  function updateCreateAggCondition(
    index: number,
    field: keyof AggregateCondition,
    value: unknown,
  ) {
    setCreateForm((prev) => {
      const agg = [...prev.aggregateConditions];
      agg[index] = { ...agg[index], [field]: value };
      return { ...prev, aggregateConditions: agg };
    });
  }
  function removeCreateAggCondition(index: number) {
    setCreateForm((prev) => ({
      ...prev,
      aggregateConditions: prev.aggregateConditions.filter(
        (_, i) => i !== index,
      ),
    }));
  }

  async function generateAIRules() {
    if (!selectedMonitor) return;
    setAILoading(true);
    setAiNoResults(false);
    try {
      const result = await rulesApi.generateAI({
        monitorId: selectedMonitor,
        prompt: aiPrompt || "Generate monitoring rules for anomaly detection",
      });
      setAiSuggestions(result.suggestions);
      // El backend descarta en silencio las sugerencias que referencian
      // campos fuera del esquema o ventanas de tiempo inválidas: sin este
      // aviso, el usuario ve terminar "Generando..." y nada más.
      setAiNoResults(result.suggestions.length === 0);
    } catch (err) {
      console.error("Failed to generate AI rules:", err);
      toastError(
        err instanceof Error ? err.message : "Error al generar reglas con IA",
      );
    } finally {
      setAILoading(false);
    }
  }

  async function applyAISuggestion(suggestion: AIRuleSuggestion) {
    try {
      await rulesApi.create({
        monitorId: selectedMonitor,
        name: suggestion.name,
        description: suggestion.description,
        conditionGroup: suggestion.conditionGroup,
        aggregateConditions: suggestion.aggregateConditions,
        actions: suggestion.actions,
        severity: suggestion.severity,
        aiGenerated: true,
      });
      setAiSuggestions((prev) => prev.filter((s) => s !== suggestion));
      toastSuccess(`Regla "${suggestion.name}" creada`);
      loadRules();
    } catch (err) {
      console.error("Failed to apply AI suggestion:", err);
      toastError(
        err instanceof Error ? err.message : "Error al aplicar sugerencia",
      );
    }
  }

  function openEditRule(rule: Rule) {
    setEditForm({
      name: rule.name,
      description: rule.description || "",
      severity: rule.severity,
      logic: rule.conditionGroup?.logic || "AND",
      conditions: rule.conditionGroup?.conditions?.map((c) => ({ ...c })) || [],
      aggregateConditions:
        rule.aggregateConditions?.map((a) => ({ ...a })) || [],
      scheduleEnabled: rule.schedule?.enabled || false,
      screeningFields: rule.screeningFields || [],
      ...(() => {
        const cron =
          rule.schedule?.cronExpr ||
          presetToCronFallback(rule.schedule?.preset || "");
        if (cron) {
          const s = parseCronToSchedule(cron);
          return {
            scheduleFrequency: s.frequency,
            scheduleDay: s.day,
            scheduleTime: s.time,
          };
        }
        return {
          scheduleFrequency: "daily",
          scheduleDay: "1",
          scheduleTime: "06:00",
        };
      })(),
    });
    setEditingRule(rule);
  }

  function updateCondition(
    index: number,
    field: keyof Condition,
    value: unknown,
  ) {
    setEditForm((prev) => {
      const conditions = [...prev.conditions];
      conditions[index] = { ...conditions[index], [field]: value };
      return { ...prev, conditions };
    });
  }

  function removeCondition(index: number) {
    setEditForm((prev) => ({
      ...prev,
      conditions: prev.conditions.filter((_, i) => i !== index),
    }));
  }

  function addCondition() {
    setEditForm((prev) => ({
      ...prev,
      conditions: [
        ...prev.conditions,
        { field: "", operator: "gt" as Operator, value: "" },
      ],
    }));
  }

  function addAggregateCondition() {
    setEditForm((prev) => ({
      ...prev,
      aggregateConditions: [
        ...prev.aggregateConditions,
        {
          field: "",
          function: "sum" as AggFunction,
          groupBy: "",
          timeField: "",
          timeWindow: "30d",
          operator: "gt" as Operator,
          threshold: 0,
        },
      ],
    }));
  }

  function updateAggCondition(
    index: number,
    field: keyof AggregateCondition,
    value: unknown,
  ) {
    setEditForm((prev) => {
      const agg = [...prev.aggregateConditions];
      agg[index] = { ...agg[index], [field]: value };
      return { ...prev, aggregateConditions: agg };
    });
  }

  function removeAggCondition(index: number) {
    setEditForm((prev) => ({
      ...prev,
      aggregateConditions: prev.aggregateConditions.filter(
        (_, i) => i !== index,
      ),
    }));
  }

  async function saveEditRule(e: React.FormEvent) {
    e.preventDefault();
    if (!editingRule) return;
    try {
      await rulesApi.update(editingRule.id, {
        name: editForm.name,
        description: editForm.description,
        severity: editForm.severity,
        conditionGroup: {
          logic: editForm.logic,
          conditions: editForm.conditions,
        },
        aggregateConditions:
          editForm.aggregateConditions.length > 0
            ? editForm.aggregateConditions
            : undefined,
        schedule: {
          enabled: editForm.scheduleEnabled,
          preset: "" as SchedulePreset,
          cronExpr: editForm.scheduleEnabled
            ? buildCronExpr(
                editForm.scheduleFrequency,
                editForm.scheduleDay,
                editForm.scheduleTime,
              )
            : "",
        },
        screeningFields: editForm.screeningFields,
      });
      setEditingRule(null);
      setBacktestResult(null);
      loadRules();
    } catch (err) {
      console.error("Failed to update rule:", err);
      toastError(
        err instanceof Error ? err.message : "Error al actualizar regla",
      );
    }
  }

  const editMonitorSchema = editingRule
    ? monitors.find((m) => m.id === editingRule.monitorId)?.schema
    : undefined;
  const numericFields = editMonitorSchema?.filter((f) => f.type === "number");
  const aggFieldOptions =
    numericFields && numericFields.length > 0
      ? numericFields
      : editMonitorSchema;

  async function toggleRule(id: string, active: boolean) {
    try {
      await rulesApi.update(id, { active: !active });
      loadRules();
    } catch (err) {
      console.error("Failed to toggle rule:", err);
      toastError(
        err instanceof Error ? err.message : "Error al actualizar regla",
      );
    }
  }

  function openDeleteConfirm(rule: Rule) {
    setDeleteConfirm({ rule, input: "" });
  }

  async function confirmDelete() {
    if (!deleteConfirm || deleteConfirm.input !== "BORRAR") return;
    try {
      await rulesApi.delete(deleteConfirm.rule.id);
      toastSuccess(`Regla "${deleteConfirm.rule.name}" eliminada`);
      setDeleteConfirm(null);
      loadRules();
    } catch (err) {
      toastError(
        err instanceof Error ? err.message : "Error al eliminar regla",
      );
    }
  }

  async function executeRule(id: string) {
    setExecutingRuleId(id);
    try {
      const result = await rulesApi.execute(id);
      toastSuccess(
        `Regla ejecutada: ${result.redFlagsGenerated} bandera(s) roja(s) generada(s)`,
      );
      loadRules();
    } catch (err) {
      console.error("Failed to execute rule:", err);
      toastError(
        err instanceof Error ? err.message : "Error al ejecutar regla",
      );
    } finally {
      setExecutingRuleId(null);
    }
  }

  async function executeAllRules() {
    const activeRules = rules.filter((r) => r.active);
    if (activeRules.length === 0) return;
    setExecutingAll(true);
    setExecuteAllProgress({
      current: 0,
      total: activeRules.length,
      redFlags: 0,
    });
    let totalRedFlags = 0;
    let failedCount = 0;
    for (let i = 0; i < activeRules.length; i++) {
      const rule = activeRules[i];
      setExecutingRuleId(rule.id);
      setExecuteAllProgress((prev) => ({ ...prev, current: i + 1 }));
      try {
        const result = await rulesApi.execute(rule.id);
        totalRedFlags += result.redFlagsGenerated;
        setExecuteAllProgress((prev) => ({ ...prev, redFlags: totalRedFlags }));
      } catch (err) {
        failedCount++;
        console.error("Failed to execute rule:", rule.name, err);
      }
    }
    setExecutingRuleId(null);
    setExecutingAll(false);
    if (failedCount > 0) {
      toastError(
        `${failedCount} de ${activeRules.length} reglas fallaron. ${totalRedFlags} bandera(s) roja(s) generada(s)`,
      );
    } else {
      toastSuccess(
        `Todas las reglas ejecutadas: ${totalRedFlags} bandera(s) roja(s) generada(s)`,
      );
    }
    loadRules();
  }

  async function toggleSchedule(rule: Rule) {
    const newEnabled = !rule.schedule?.enabled;
    const cron =
      rule.schedule?.cronExpr ||
      presetToCronFallback(rule.schedule?.preset || "") ||
      "0 6 * * *";
    try {
      await rulesApi.update(rule.id, {
        schedule: {
          enabled: newEnabled,
          preset: "" as SchedulePreset,
          cronExpr: cron,
        },
      });
      loadRules();
    } catch (err) {
      toastError(
        err instanceof Error ? err.message : "Error al actualizar programacion",
      );
    }
  }

  async function updateScheduleFromMatrix(
    ruleId: string,
    freq: string,
    day: string,
    time: string,
  ) {
    try {
      await rulesApi.update(ruleId, {
        schedule: {
          enabled: true,
          preset: "" as SchedulePreset,
          cronExpr: buildCronExpr(freq, day, time),
        },
      });
      loadRules();
    } catch (err) {
      toastError(
        err instanceof Error ? err.message : "Error al actualizar programacion",
      );
    }
  }

  const createMonitorSchema = monitors.find(
    (m) => m.id === createForm.monitorId,
  )?.schema;
  const createNumericFields = createMonitorSchema?.filter(
    (f) => f.type === "number",
  );
  const createAggFieldOptions =
    createNumericFields && createNumericFields.length > 0
      ? createNumericFields
      : createMonitorSchema;

  const filteredRules = useMemo(() => {
    let filtered = rules.filter(
      (r) => filterMonitorId === "all" || r.monitorId === filterMonitorId,
    );
    if (activeTab === "daily") {
      filtered = filtered.filter(
        (r) => !r.aggregateConditions || r.aggregateConditions.length === 0,
      );
    } else {
      filtered = filtered.filter(
        (r) => r.aggregateConditions && r.aggregateConditions.length > 0,
      );
    }
    return filtered;
  }, [rules, filterMonitorId, activeTab]);

  // Para la pestaña de efectividad: mismo filtro por monitor, sin el split
  // diario/rango (la efectividad importa para cualquier tipo de regla).
  const rulesForEffectiveness = useMemo(
    () =>
      rules.filter(
        (r) => filterMonitorId === "all" || r.monitorId === filterMonitorId,
      ),
    [rules, filterMonitorId],
  );

  const dailyCount = useMemo(
    () =>
      rules.filter(
        (r) => !r.aggregateConditions || r.aggregateConditions.length === 0,
      ).length,
    [rules],
  );
  const rangeCount = useMemo(
    () =>
      rules.filter(
        (r) => r.aggregateConditions && r.aggregateConditions.length > 0,
      ).length,
    [rules],
  );

  return (
    <>
      <Header title="Reglas" />
      <div className="p-6">
        <div className="mb-6 flex items-center justify-between">
          <p className="text-sm text-muted-foreground">
            Configura reglas de monitoreo para detectar anomalias
          </p>
          <div className="flex gap-2">
            <Button
              variant="outline"
              onClick={executeAllRules}
              disabled={
                executingAll || rules.filter((r) => r.active).length === 0
              }
            >
              {executingAll ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin" />
                  {executeAllProgress.current}/{executeAllProgress.total} (
                  {executeAllProgress.redFlags} banderas rojas)
                </>
              ) : (
                <>
                  <Play className="h-4 w-4" /> Ejecutar todas
                </>
              )}
            </Button>
            <Dialog open={isAIOpen} onOpenChange={setIsAIOpen}>
              <DialogTrigger asChild>
                <Button variant="outline">
                  <Sparkles className="h-4 w-4" /> Generar con IA
                </Button>
              </DialogTrigger>
              <DialogContent className="max-w-2xl">
                <DialogHeader>
                  <DialogTitle>Generar reglas con IA</DialogTitle>
                </DialogHeader>
                <div className="space-y-4">
                  <div className="space-y-2">
                    <Label>Monitor</Label>
                    <Select
                      value={selectedMonitor}
                      onValueChange={setSelectedMonitor}
                    >
                      <SelectTrigger>
                        <SelectValue placeholder="Selecciona un monitor" />
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
                  <div className="space-y-2">
                    <Label>Contexto (opcional)</Label>
                    <Textarea
                      value={aiPrompt}
                      onChange={(e) => setAiPrompt(e.target.value)}
                      placeholder="Ej: Detectar transacciones sospechosas con montos altos o patrones inusuales"
                    />
                  </div>
                  <Button
                    onClick={generateAIRules}
                    disabled={aiLoading || !selectedMonitor}
                  >
                    {aiLoading ? (
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    ) : null}
                    {aiLoading ? "Generando..." : "Generar reglas"}
                  </Button>
                  {aiLoading && (
                    <p className="text-xs text-muted-foreground">
                      Puede tardar hasta un minuto — el modelo de IA está
                      analizando el esquema.
                    </p>
                  )}

                  {aiNoResults && !aiLoading && (
                    <div className="rounded-md border border-dashed p-4">
                      <p className="text-sm font-medium">
                        La IA no devolvió ninguna sugerencia válida
                      </p>
                      <p className="mt-1 text-xs text-muted-foreground">
                        Se descartan las sugerencias que referencian campos
                        fuera del esquema de este monitor. Probá describir el
                        criterio usando los nombres exactos de los campos, o
                        elegí otro monitor.
                      </p>
                    </div>
                  )}

                  {aiSuggestions.length > 0 && (
                    <div className="space-y-3 border-t pt-4">
                      <p className="text-sm font-medium">Sugerencias:</p>
                      {aiSuggestions.map((s, i) => (
                        <div key={i} className="rounded-md border p-4">
                          <div className="flex items-start justify-between">
                            <div>
                              <p className="font-medium">{s.name}</p>
                              <p className="mt-1 text-sm text-muted-foreground">
                                {s.description}
                              </p>
                              <p className="mt-2 text-xs text-muted-foreground italic">
                                {s.reasoning}
                              </p>
                            </div>
                            <div className="flex items-center gap-2">
                              <Badge
                                variant={
                                  s.severity === "critical" ||
                                  s.severity === "high"
                                    ? "destructive"
                                    : "secondary"
                                }
                              >
                                {s.severity}
                              </Badge>
                              <Button
                                size="sm"
                                onClick={() => applyAISuggestion(s)}
                              >
                                Aplicar
                              </Button>
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </DialogContent>
            </Dialog>

            <Button variant="outline" onClick={() => setIsTemplatesOpen(true)}>
              Desde plantilla
            </Button>

            <Dialog
              open={isCreateOpen}
              onOpenChange={(open) => {
                setIsCreateOpen(open);
                if (!open) setBacktestResult(null);
              }}
            >
              <DialogTrigger asChild>
                <Button>
                  <Plus className="h-4 w-4" />
                  Nueva regla
                </Button>
              </DialogTrigger>
              <DialogContent className="max-w-2xl">
                <DialogHeader>
                  <DialogTitle>Crear regla</DialogTitle>
                </DialogHeader>
                <form onSubmit={createRule} className="space-y-4">
                  <div className="grid grid-cols-2 gap-4">
                    <div className="space-y-2">
                      <Label>Nombre</Label>
                      <Input
                        value={createForm.name}
                        onChange={(e) =>
                          setCreateForm({ ...createForm, name: e.target.value })
                        }
                        required
                      />
                    </div>
                    <div className="space-y-2">
                      <Label>Monitor</Label>
                      <Select
                        value={createForm.monitorId}
                        onValueChange={(v) =>
                          setCreateForm({ ...createForm, monitorId: v })
                        }
                      >
                        <SelectTrigger>
                          <SelectValue placeholder="Selecciona un monitor" />
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
                  <div className="grid grid-cols-3 gap-2">
                    <div className="space-y-2">
                      <Label>Severidad</Label>
                      <Select
                        value={createForm.severity}
                        onValueChange={(v) =>
                          setCreateForm({
                            ...createForm,
                            severity: v as Severity,
                          })
                        }
                      >
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="low">Baja</SelectItem>
                          <SelectItem value="medium">Media</SelectItem>
                          <SelectItem value="high">Alta</SelectItem>
                          <SelectItem value="critical">Crítica</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-2">
                      <Label>Logica</Label>
                      <Select
                        value={createForm.logic}
                        onValueChange={(v) =>
                          setCreateForm({
                            ...createForm,
                            logic: v as "AND" | "OR",
                          })
                        }
                      >
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="AND">AND (todas)</SelectItem>
                          <SelectItem value="OR">OR (alguna)</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                  </div>

                  {/* Regulatory fields */}
                  <div className="space-y-2">
                    <Label className="text-xs text-muted-foreground">
                      Cumplimiento regulatorio (opcional)
                    </Label>
                    <div className="grid grid-cols-3 gap-2">
                      <Input
                        value={createForm.fatfTypology}
                        onChange={(e) =>
                          setCreateForm({
                            ...createForm,
                            fatfTypology: e.target.value,
                          })
                        }
                        placeholder="Tipologia FATF"
                      />
                      <Input
                        value={createForm.regulatoryBasis}
                        onChange={(e) =>
                          setCreateForm({
                            ...createForm,
                            regulatoryBasis: e.target.value,
                          })
                        }
                        placeholder="Base regulatoria"
                      />
                      <Input
                        value={createForm.thresholdJustification}
                        onChange={(e) =>
                          setCreateForm({
                            ...createForm,
                            thresholdJustification: e.target.value,
                          })
                        }
                        placeholder="Justificacion del umbral"
                      />
                    </div>
                  </div>

                  <div className="space-y-2">
                    <Label className="text-xs text-muted-foreground">
                      Campos a screenear contra sanciones (opcional)
                    </Label>
                    <div className="flex flex-wrap gap-2">
                      {(createMonitorSchema || []).map(
                        (f) => {
                          const active = createForm.screeningFields.includes(f.name);
                          return (
                            <button
                              key={f.name}
                              type="button"
                              onClick={() =>
                                setCreateForm((prev) => ({
                                  ...prev,
                                  screeningFields: active
                                    ? prev.screeningFields.filter((n) => n !== f.name)
                                    : [...prev.screeningFields, f.name],
                                }))
                              }
                              className={`rounded-md border px-2 py-1 text-xs ${
                                active ? "border-primary bg-primary/10" : ""
                              }`}
                            >
                              {f.name}
                            </button>
                          );
                        },
                      )}
                    </div>
                  </div>

                  {/* Conditions */}
                  <div className="space-y-2">
                    <div className="flex items-center justify-between">
                      <Label>Condiciones</Label>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={addCreateCondition}
                      >
                        <Plus className="mr-1 h-3 w-3" /> Agregar
                      </Button>
                    </div>
                    {createForm.conditions.map((cond, i) => (
                      <div
                        key={i}
                        className="grid grid-cols-[1fr_1fr_1fr_auto] gap-2"
                      >
                        <Select
                          value={cond.field}
                          onValueChange={(v) =>
                            updateCreateCondition(i, "field", v)
                          }
                        >
                          <SelectTrigger>
                            <SelectValue placeholder="Campo" />
                          </SelectTrigger>
                          <SelectContent>
                            {createMonitorSchema?.map((f) => (
                              <SelectItem key={f.name} value={f.name}>
                                {f.name}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                        <Select
                          value={cond.operator}
                          onValueChange={(v) =>
                            updateCreateCondition(i, "operator", v)
                          }
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="eq">= Igual</SelectItem>
                            <SelectItem value="neq">
                              &#8800; Diferente
                            </SelectItem>
                            <SelectItem value="gt">&gt; Mayor</SelectItem>
                            <SelectItem value="lt">&lt; Menor</SelectItem>
                            <SelectItem value="gte">
                              &#8805; Mayor o igual
                            </SelectItem>
                            <SelectItem value="lte">
                              &#8804; Menor o igual
                            </SelectItem>
                            <SelectItem value="contains">Contiene</SelectItem>
                            <SelectItem value="regex">Regex</SelectItem>
                            <SelectItem value="in">En lista</SelectItem>
                            <SelectItem value="not_in">No en lista</SelectItem>
                            <SelectItem value="between">
                              Entre (min,max)
                            </SelectItem>
                            <SelectItem value="starts_with">
                              Comienza con
                            </SelectItem>
                            <SelectItem value="ends_with">
                              Finaliza con
                            </SelectItem>
                            <SelectItem value="is_null">Es nulo</SelectItem>
                            <SelectItem value="is_not_null">
                              No es nulo
                            </SelectItem>
                          </SelectContent>
                        </Select>
                        {isMCCField(cond.field) ? (
                          <MCCPicker
                            value={cond.value}
                            onChange={(v) =>
                              updateCreateCondition(i, "value", v)
                            }
                            multi={
                              cond.operator === "in" ||
                              cond.operator === "not_in"
                            }
                          />
                        ) : (
                          <Input
                            value={String(cond.value ?? "")}
                            onChange={(e) =>
                              updateCreateCondition(i, "value", e.target.value)
                            }
                            placeholder={
                              cond.operator === "between" ? "min,max" : "Valor"
                            }
                          />
                        )}
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          onClick={() => removeCreateCondition(i)}
                          disabled={createForm.conditions.length <= 1}
                        >
                          <Trash2 className="h-4 w-4 text-destructive" />
                        </Button>
                      </div>
                    ))}
                  </div>

                  {/* Aggregate conditions */}
                  <div className="space-y-2">
                    <div className="flex items-center justify-between">
                      <Label className="flex items-center gap-1">
                        <Calculator className="h-3 w-3" /> Condiciones agregadas
                      </Label>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={addCreateAggCondition}
                      >
                        <Plus className="mr-1 h-3 w-3" /> Agregar
                      </Button>
                    </div>
                    {createForm.aggregateConditions.map((agg, i) => (
                      <div
                        key={i}
                        className="rounded-md border bg-muted/30 p-3 space-y-2"
                      >
                        <div className="grid grid-cols-[1fr_1fr_1fr_auto] gap-2">
                          <div className="space-y-1">
                            <span className="text-[10px] text-muted-foreground">
                              Funcion
                            </span>
                            <Select
                              value={agg.function}
                              onValueChange={(v) =>
                                updateCreateAggCondition(i, "function", v)
                              }
                            >
                              <SelectTrigger className="h-8 text-xs">
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                <SelectItem value="sum">SUM</SelectItem>
                                <SelectItem value="count">COUNT</SelectItem>
                                <SelectItem value="avg">AVG</SelectItem>
                                <SelectItem value="min">MIN</SelectItem>
                                <SelectItem value="max">MAX</SelectItem>
                              </SelectContent>
                            </Select>
                          </div>
                          <div className="space-y-1">
                            <span className="text-[10px] text-muted-foreground">
                              Campo
                            </span>
                            <Select
                              value={agg.field}
                              onValueChange={(v) =>
                                updateCreateAggCondition(i, "field", v)
                              }
                            >
                              <SelectTrigger className="h-8 text-xs">
                                <SelectValue placeholder="Campo" />
                              </SelectTrigger>
                              <SelectContent>
                                {createAggFieldOptions?.map((f) => (
                                  <SelectItem key={f.name} value={f.name}>
                                    {f.name}
                                  </SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                          </div>
                          <div className="space-y-1">
                            <span className="text-[10px] text-muted-foreground">
                              Agrupar por
                            </span>
                            <Select
                              value={agg.groupBy}
                              onValueChange={(v) =>
                                updateCreateAggCondition(i, "groupBy", v)
                              }
                            >
                              <SelectTrigger className="h-8 text-xs">
                                <SelectValue placeholder="Campo" />
                              </SelectTrigger>
                              <SelectContent>
                                {createMonitorSchema?.map((f) => (
                                  <SelectItem key={f.name} value={f.name}>
                                    {f.name}
                                  </SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                          </div>
                          <div className="flex items-end">
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon"
                              className="h-8 w-8"
                              onClick={() => removeCreateAggCondition(i)}
                            >
                              <Trash2 className="h-4 w-4 text-destructive" />
                            </Button>
                          </div>
                        </div>
                        <div className="grid grid-cols-4 gap-2">
                          <div className="space-y-1">
                            <span className="text-[10px] text-muted-foreground">
                              Campo fecha
                            </span>
                            <Select
                              value={agg.timeField}
                              onValueChange={(v) =>
                                updateCreateAggCondition(i, "timeField", v)
                              }
                            >
                              <SelectTrigger className="h-8 text-xs">
                                <SelectValue placeholder="Fecha" />
                              </SelectTrigger>
                              <SelectContent>
                                {createMonitorSchema?.map((f) => (
                                  <SelectItem key={f.name} value={f.name}>
                                    {f.name}
                                  </SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                          </div>
                          <div className="space-y-1">
                            <span className="text-[10px] text-muted-foreground">
                              Ventana
                            </span>
                            <Select
                              value={agg.timeWindow}
                              onValueChange={(v) =>
                                updateCreateAggCondition(i, "timeWindow", v)
                              }
                            >
                              <SelectTrigger className="h-8 text-xs">
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                <SelectItem value="30s">30 segundos</SelectItem>
                                <SelectItem value="60s">60 segundos</SelectItem>
                                <SelectItem value="5min">5 minutos</SelectItem>
                                <SelectItem value="15min">15 minutos</SelectItem>
                                <SelectItem value="1h">1 hora</SelectItem>
                                <SelectItem value="6h">6 horas</SelectItem>
                                <SelectItem value="12h">12 horas</SelectItem>
                                <SelectItem value="24h">24 horas</SelectItem>
                                <SelectItem value="7d">7 dias</SelectItem>
                                <SelectItem value="15d">15 dias</SelectItem>
                                <SelectItem value="30d">30 dias</SelectItem>
                                <SelectItem value="90d">90 dias</SelectItem>
                                <SelectItem value="365d">1 año</SelectItem>
                              </SelectContent>
                            </Select>
                          </div>
                          <div className="space-y-1">
                            <span className="text-[10px] text-muted-foreground">
                              Operador
                            </span>
                            <Select
                              value={agg.operator}
                              onValueChange={(v) =>
                                updateCreateAggCondition(
                                  i,
                                  "operator",
                                  v as Operator,
                                )
                              }
                            >
                              <SelectTrigger className="h-8 text-xs">
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                <SelectItem value="gt">&gt; Mayor</SelectItem>
                                <SelectItem value="gte">
                                  &#8805; Mayor o igual
                                </SelectItem>
                                <SelectItem value="lt">&lt; Menor</SelectItem>
                                <SelectItem value="lte">
                                  &#8804; Menor o igual
                                </SelectItem>
                                <SelectItem value="eq">= Igual</SelectItem>
                              </SelectContent>
                            </Select>
                          </div>
                          <div className="space-y-1">
                            <span className="text-[10px] text-muted-foreground">
                              Umbral
                            </span>
                            <Input
                              type="number"
                              className="h-8 text-xs"
                              value={agg.threshold}
                              onChange={(e) =>
                                updateCreateAggCondition(
                                  i,
                                  "threshold",
                                  parseFloat(e.target.value) || 0,
                                )
                              }
                              placeholder="50000"
                            />
                          </div>
                        </div>
                        <p className="text-[10px] text-muted-foreground italic">
                          {agg.function.toUpperCase()}({agg.field || "?"})
                          agrupado por {agg.groupBy || "?"} en ultimos{" "}
                          {agg.timeWindow} {operatorSymbol(agg.operator)}{" "}
                          {agg.threshold.toLocaleString()}
                        </p>
                      </div>
                    ))}
                    {createForm.aggregateConditions.length === 0 && (
                      <p className="text-xs text-muted-foreground">
                        Sin condiciones agregadas. Agrega una para detectar
                        acumulados por periodo.
                      </p>
                    )}
                  </div>

                  <div className="space-y-2">
                    <Label>Descripción</Label>
                    <Textarea
                      value={createForm.description}
                      onChange={(e) =>
                        setCreateForm({
                          ...createForm,
                          description: e.target.value,
                        })
                      }
                    />
                  </div>

                  {/* Schedule */}
                  <div className="space-y-2 rounded-md border p-3">
                    <div className="flex items-center justify-between">
                      <Label className="flex items-center gap-1">
                        <Clock className="h-3 w-3" /> Programacion
                      </Label>
                      <button
                        type="button"
                        role="switch"
                        aria-checked={createForm.scheduleEnabled}
                        onClick={() =>
                          setCreateForm((prev) => ({
                            ...prev,
                            scheduleEnabled: !prev.scheduleEnabled,
                          }))
                        }
                        className={cn(
                          "relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors",
                          createForm.scheduleEnabled
                            ? "bg-primary"
                            : "bg-muted",
                        )}
                      >
                        <span
                          className={cn(
                            "pointer-events-none block h-4 w-4 rounded-full bg-background shadow-lg ring-0 transition-transform",
                            createForm.scheduleEnabled
                              ? "translate-x-4"
                              : "translate-x-0",
                          )}
                        />
                      </button>
                    </div>
                    {createForm.scheduleEnabled && (
                      <div className="grid grid-cols-3 gap-2">
                        <div className="space-y-1">
                          <span className="text-[10px] text-muted-foreground">
                            Frecuencia
                          </span>
                          <Select
                            value={createForm.scheduleFrequency}
                            onValueChange={(v) =>
                              setCreateForm((prev) => ({
                                ...prev,
                                scheduleFrequency: v,
                              }))
                            }
                          >
                            <SelectTrigger className="h-8 text-xs">
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="daily">Diario</SelectItem>
                              <SelectItem value="weekly">Semanal</SelectItem>
                              <SelectItem value="monthly">Mensual</SelectItem>
                            </SelectContent>
                          </Select>
                        </div>
                        {createForm.scheduleFrequency === "weekly" && (
                          <div className="space-y-1">
                            <span className="text-[10px] text-muted-foreground">
                              Dia
                            </span>
                            <Select
                              value={createForm.scheduleDay}
                              onValueChange={(v) =>
                                setCreateForm((prev) => ({
                                  ...prev,
                                  scheduleDay: v,
                                }))
                              }
                            >
                              <SelectTrigger className="h-8 text-xs">
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                <SelectItem value="1">Lunes</SelectItem>
                                <SelectItem value="2">Martes</SelectItem>
                                <SelectItem value="3">Miercoles</SelectItem>
                                <SelectItem value="4">Jueves</SelectItem>
                                <SelectItem value="5">Viernes</SelectItem>
                                <SelectItem value="6">Sabado</SelectItem>
                                <SelectItem value="0">Domingo</SelectItem>
                              </SelectContent>
                            </Select>
                          </div>
                        )}
                        {createForm.scheduleFrequency === "monthly" && (
                          <div className="space-y-1">
                            <span className="text-[10px] text-muted-foreground">
                              Dia del mes
                            </span>
                            <Select
                              value={createForm.scheduleDay}
                              onValueChange={(v) =>
                                setCreateForm((prev) => ({
                                  ...prev,
                                  scheduleDay: v,
                                }))
                              }
                            >
                              <SelectTrigger className="h-8 text-xs">
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                {Array.from({ length: 28 }, (_, i) => (
                                  <SelectItem key={i + 1} value={String(i + 1)}>
                                    {i + 1}
                                  </SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                          </div>
                        )}
                        <div className="space-y-1">
                          <span className="text-[10px] text-muted-foreground">
                            Hora
                          </span>
                          <Select
                            value={createForm.scheduleTime}
                            onValueChange={(v) =>
                              setCreateForm((prev) => ({
                                ...prev,
                                scheduleTime: v,
                              }))
                            }
                          >
                            <SelectTrigger className="h-8 text-xs">
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              {TIME_OPTIONS.map((t) => (
                                <SelectItem key={t} value={t}>
                                  {t}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        </div>
                      </div>
                    )}
                  </div>

                  {backtestResult && (
                    <div className="rounded-md border bg-muted/30 p-3 text-xs space-y-1">
                      <p className="font-medium">
                        {backtestResult.matchCount === 0
                          ? "No habría generado ninguna alerta contra el histórico"
                          : `Habría generado ${backtestResult.matchCount} alerta(s) contra el histórico`}
                      </p>
                      {backtestResult.sample.slice(0, 5).map((rf, i) => (
                        <p key={i} className="text-muted-foreground truncate">
                          {rf.message}
                        </p>
                      ))}
                    </div>
                  )}

                  <div className="flex gap-2">
                    <Button
                      type="button"
                      variant="outline"
                      className="flex-1"
                      disabled={backtesting || !createForm.monitorId}
                      onClick={() =>
                        runBacktest(
                          createForm.monitorId,
                          createForm.logic,
                          createForm.conditions,
                          createForm.aggregateConditions,
                          createForm.severity,
                        )
                      }
                    >
                      {backtesting ? "Probando…" : "Probar contra histórico"}
                    </Button>
                    <Button
                      type="submit"
                      className="flex-1"
                      disabled={!createForm.monitorId || !createForm.name}
                    >
                      Crear regla
                    </Button>
                  </div>
                </form>
              </DialogContent>
            </Dialog>
          </div>
        </div>

        {/* Monitor filter */}
        <div className="mb-4 flex items-center gap-3">
          <Select value={filterMonitorId} onValueChange={setFilterMonitorId}>
            <SelectTrigger className="w-64">
              <SelectValue placeholder="Filtrar por monitor" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">
                Todos los monitores ({rules.length})
              </SelectItem>
              {monitors.map((m) => {
                const count = rules.filter((r) => r.monitorId === m.id).length;
                return (
                  <SelectItem key={m.id} value={m.id}>
                    {m.name} ({count})
                  </SelectItem>
                );
              })}
            </SelectContent>
          </Select>
          {filterMonitorId !== "all" && (
            <Button
              variant="ghost"
              size="sm"
              className="h-8 text-xs"
              onClick={() => setFilterMonitorId("all")}
            >
              <X className="h-3 w-3 mr-1" /> Limpiar filtro
            </Button>
          )}
        </div>

        <Tabs value={activeTab} onValueChange={setActiveTab}>
          <TabsList className="mb-4">
            <TabsTrigger value="daily">
              Reglas diarias ({dailyCount})
            </TabsTrigger>
            <TabsTrigger value="range">
              Reglas de rango ({rangeCount})
            </TabsTrigger>
            <TabsTrigger value="schedule">
              Programacion ({rules.filter((r) => r.schedule?.enabled).length})
            </TabsTrigger>
            <TabsTrigger value="effectiveness">Efectividad</TabsTrigger>
          </TabsList>

          <TabsContent value={activeTab} className="mt-0">
            {filteredRules.length === 0 ? (
              <Card>
                <CardContent className="flex flex-col items-center justify-center py-12">
                  <ShieldCheck className="mb-4 h-12 w-12 text-muted-foreground" />
                  <p className="mb-2 font-medium">
                    No hay reglas{" "}
                    {activeTab === "daily" ? "diarias" : "de rango"}
                  </p>
                  <p className="text-sm text-muted-foreground">
                    Crea reglas manualmente o genera con IA
                  </p>
                </CardContent>
              </Card>
            ) : (
              <div className="space-y-3">
                {filteredRules.map((rule, idx) => (
                  <Card key={rule.id}>
                    <CardContent className="flex items-center justify-between p-4">
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2 flex-wrap">
                          <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-muted text-xs font-bold text-muted-foreground">
                            {idx + 1}
                          </span>
                          {wasTriggeredToday(rule.lastTriggered) ? (
                            <span
                              title="Ejecutada hoy"
                              className="flex h-5 w-5 shrink-0 items-center justify-center"
                            >
                              <CircleCheck className="h-4 w-4 text-success-fg" />
                            </span>
                          ) : (
                            <span
                              title="No ejecutada hoy"
                              className="flex h-5 w-5 shrink-0 items-center justify-center"
                            >
                              <CircleAlert className="h-4 w-4 text-warning-fg" />
                            </span>
                          )}
                          <p className="font-medium">{rule.name}</p>
                          {rule.aiGenerated && (
                            <Badge variant="secondary">
                              <Sparkles className="mr-1 h-3 w-3" />
                              IA
                            </Badge>
                          )}
                          <Badge
                            variant={rule.active ? "success" : "secondary"}
                            className="cursor-pointer"
                            onClick={() => toggleRule(rule.id, rule.active)}
                          >
                            {rule.active ? "Activa" : "Inactiva"}
                          </Badge>
                          {rule.schedule?.enabled && (
                            <Badge variant="outline" className="gap-1 text-xs">
                              <Clock className="h-3 w-3" />
                              {scheduleLabel(rule.schedule)}
                            </Badge>
                          )}
                        </div>
                        <p className="mt-1 text-sm text-muted-foreground">
                          {rule.description}
                        </p>
                        <p className="mt-1 text-xs text-muted-foreground">
                          Monitor:{" "}
                          <span className="font-medium text-foreground">
                            {monitors.find((m) => m.id === rule.monitorId)
                              ?.name ?? "—"}
                          </span>
                          {" · "}
                          {rule.triggerCount} activaciones
                          {rule.lastTriggered &&
                            ` · Última: ${formatDate(rule.lastTriggered)}`}
                        </p>
                        {rule.aggregateConditions &&
                          rule.aggregateConditions.length > 0 && (
                            <p className="mt-1 flex items-center gap-1 text-xs text-accent-fg">
                              <Calculator className="h-3 w-3" />
                              {rule.aggregateConditions
                                .map(
                                  (a) =>
                                    `${a.function.toUpperCase()}(${a.field}) por ${a.groupBy} / ${a.timeWindow}`,
                                )
                                .join(", ")}
                            </p>
                          )}
                        {rule.schedule?.enabled && (
                          <p className="mt-1 text-xs text-muted-foreground">
                            {rule.schedule.lastEvaluated && (
                              <>
                                Última eval:{" "}
                                {formatDate(rule.schedule.lastEvaluated)}
                              </>
                            )}
                            {rule.schedule.nextRun && (
                              <>
                                {" "}
                                · Proxima: {formatDate(rule.schedule.nextRun)}
                              </>
                            )}
                          </p>
                        )}
                      </div>
                      <div className="flex items-center gap-1">
                        <Badge variant={severityVariant(rule.severity)}>
                          {rule.severity}
                        </Badge>
                        <Button
                          variant="ghost"
                          size="icon"
                          title="Ejecutar ahora"
                          disabled={executingRuleId === rule.id}
                          onClick={() => executeRule(rule.id)}
                        >
                          {executingRuleId === rule.id ? (
                            <Loader2 className="h-4 w-4 animate-spin" />
                          ) : (
                            <Play className="h-4 w-4 text-success-fg" />
                          )}
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          title="Crear gráfico"
                          onClick={() => openChartDialog(rule)}
                        >
                          <BarChart3 className="h-4 w-4 text-accent-fg" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => openEditRule(rule)}
                        >
                          <Pencil className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => openDeleteConfirm(rule)}
                        >
                          <Trash2 className="h-4 w-4 text-destructive" />
                        </Button>
                      </div>
                    </CardContent>
                  </Card>
                ))}
              </div>
            )}
          </TabsContent>

          <TabsContent value="schedule" className="mt-0">
            <Card>
              <CardContent className="p-0">
                <div className="overflow-x-auto">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b bg-muted/50">
                        <th className="px-4 py-2 text-left font-medium">
                          Regla
                        </th>
                        <th className="px-4 py-2 text-left font-medium">
                          Monitor
                        </th>
                        <th className="px-4 py-2 text-left font-medium">
                          Frecuencia
                        </th>
                        <th className="px-4 py-2 text-left font-medium">
                          Hora
                        </th>
                        <th className="px-4 py-2 text-left font-medium">
                          Activa
                        </th>
                        <th className="px-4 py-2 text-left font-medium">
                          Programada
                        </th>
                        <th className="px-4 py-2 text-left font-medium">
                          Última eval.
                        </th>
                        <th className="px-4 py-2 text-left font-medium">
                          Proxima
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {rules.map((rule) => {
                        const cron =
                          rule.schedule?.cronExpr ||
                          presetToCronFallback(rule.schedule?.preset || "");
                        const sched = cron
                          ? parseCronToSchedule(cron)
                          : { frequency: "daily", day: "1", time: "06:00" };
                        return (
                          <tr
                            key={rule.id}
                            className="border-b last:border-0 hover:bg-muted/30"
                          >
                            <td className="px-4 py-2 font-medium">
                              {rule.name}
                            </td>
                            <td className="px-4 py-2 text-muted-foreground">
                              {monitors.find((m) => m.id === rule.monitorId)
                                ?.name ?? "—"}
                            </td>
                            <td className="px-4 py-2">
                              <Select
                                value={sched.frequency}
                                onValueChange={(v) =>
                                  updateScheduleFromMatrix(
                                    rule.id,
                                    v,
                                    sched.day,
                                    sched.time,
                                  )
                                }
                                disabled={!rule.schedule?.enabled}
                              >
                                <SelectTrigger className="h-7 w-24 text-xs">
                                  <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                  <SelectItem value="daily">Diario</SelectItem>
                                  <SelectItem value="weekly">
                                    Semanal
                                  </SelectItem>
                                  <SelectItem value="monthly">
                                    Mensual
                                  </SelectItem>
                                </SelectContent>
                              </Select>
                            </td>
                            <td className="px-4 py-2">
                              <Select
                                value={sched.time}
                                onValueChange={(v) =>
                                  updateScheduleFromMatrix(
                                    rule.id,
                                    sched.frequency,
                                    sched.day,
                                    v,
                                  )
                                }
                                disabled={!rule.schedule?.enabled}
                              >
                                <SelectTrigger className="h-7 w-20 text-xs">
                                  <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                  {TIME_OPTIONS.map((t) => (
                                    <SelectItem key={t} value={t}>
                                      {t}
                                    </SelectItem>
                                  ))}
                                </SelectContent>
                              </Select>
                            </td>
                            <td className="px-4 py-2">
                              <Badge
                                variant={rule.active ? "success" : "secondary"}
                                className="text-[10px]"
                              >
                                {rule.active ? "Si" : "No"}
                              </Badge>
                            </td>
                            <td className="px-4 py-2">
                              <button
                                type="button"
                                role="switch"
                                aria-checked={rule.schedule?.enabled || false}
                                onClick={() => toggleSchedule(rule)}
                                className={cn(
                                  "relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors",
                                  rule.schedule?.enabled
                                    ? "bg-primary"
                                    : "bg-muted",
                                )}
                              >
                                <span
                                  className={cn(
                                    "pointer-events-none block h-4 w-4 rounded-full bg-background shadow-lg ring-0 transition-transform",
                                    rule.schedule?.enabled
                                      ? "translate-x-4"
                                      : "translate-x-0",
                                  )}
                                />
                              </button>
                            </td>
                            <td className="px-4 py-2 text-xs text-muted-foreground">
                              {rule.schedule?.lastEvaluated
                                ? formatDate(rule.schedule.lastEvaluated)
                                : "—"}
                            </td>
                            <td className="px-4 py-2 text-xs text-muted-foreground">
                              {rule.schedule?.nextRun
                                ? formatDate(rule.schedule.nextRun)
                                : "—"}
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
                {rules.length === 0 && (
                  <div className="flex flex-col items-center justify-center py-12">
                    <Clock className="mb-4 h-12 w-12 text-muted-foreground" />
                    <p className="text-sm text-muted-foreground">
                      No hay reglas creadas
                    </p>
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="effectiveness" className="mt-0">
            <EffectivenessTable rules={rulesForEffectiveness} />
          </TabsContent>
        </Tabs>
      </div>

      <Dialog
        open={!!editingRule}
        onOpenChange={(open) => {
          if (!open) {
            setEditingRule(null);
            setBacktestResult(null);
          }
        }}
      >
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>Editar regla</DialogTitle>
          </DialogHeader>
          <form onSubmit={saveEditRule} className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>Nombre</Label>
                <Input
                  value={editForm.name}
                  onChange={(e) =>
                    setEditForm({ ...editForm, name: e.target.value })
                  }
                  required
                />
              </div>
              <div className="grid grid-cols-2 gap-2">
                <div className="space-y-2">
                  <Label>Severidad</Label>
                  <Select
                    value={editForm.severity}
                    onValueChange={(v) =>
                      setEditForm({ ...editForm, severity: v as Severity })
                    }
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="low">Baja</SelectItem>
                      <SelectItem value="medium">Media</SelectItem>
                      <SelectItem value="high">Alta</SelectItem>
                      <SelectItem value="critical">Crítica</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2">
                  <Label>Logica</Label>
                  <Select
                    value={editForm.logic}
                    onValueChange={(v) =>
                      setEditForm({ ...editForm, logic: v as "AND" | "OR" })
                    }
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="AND">AND (todas)</SelectItem>
                      <SelectItem value="OR">OR (alguna)</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
            </div>

            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <Label>Condiciones</Label>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={addCondition}
                >
                  <Plus className="mr-1 h-3 w-3" /> Agregar
                </Button>
              </div>
              {editForm.conditions.map((cond, i) => (
                <div
                  key={i}
                  className="grid grid-cols-[1fr_1fr_1fr_auto] gap-2"
                >
                  <Select
                    value={cond.field}
                    onValueChange={(v) => updateCondition(i, "field", v)}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder="Campo" />
                    </SelectTrigger>
                    <SelectContent>
                      {editMonitorSchema?.map((f) => (
                        <SelectItem key={f.name} value={f.name}>
                          {f.name}
                        </SelectItem>
                      ))}
                      {!editMonitorSchema && cond.field && (
                        <SelectItem value={cond.field}>{cond.field}</SelectItem>
                      )}
                    </SelectContent>
                  </Select>
                  <Select
                    value={cond.operator}
                    onValueChange={(v) => updateCondition(i, "operator", v)}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="eq">= Igual</SelectItem>
                      <SelectItem value="neq">≠ Diferente</SelectItem>
                      <SelectItem value="gt">&gt; Mayor</SelectItem>
                      <SelectItem value="lt">&lt; Menor</SelectItem>
                      <SelectItem value="gte">≥ Mayor o igual</SelectItem>
                      <SelectItem value="lte">≤ Menor o igual</SelectItem>
                      <SelectItem value="contains">Contiene</SelectItem>
                      <SelectItem value="regex">Regex</SelectItem>
                      <SelectItem value="in">En lista</SelectItem>
                      <SelectItem value="not_in">No en lista</SelectItem>
                      <SelectItem value="between">Entre (min,max)</SelectItem>
                      <SelectItem value="starts_with">Comienza con</SelectItem>
                      <SelectItem value="ends_with">Finaliza con</SelectItem>
                      <SelectItem value="is_null">Es nulo</SelectItem>
                      <SelectItem value="is_not_null">No es nulo</SelectItem>
                    </SelectContent>
                  </Select>
                  {isMCCField(cond.field) ? (
                    <MCCPicker
                      value={cond.value}
                      onChange={(v) => updateCondition(i, "value", v)}
                      multi={
                        cond.operator === "in" || cond.operator === "not_in"
                      }
                    />
                  ) : (
                    <Input
                      value={String(cond.value ?? "")}
                      onChange={(e) =>
                        updateCondition(i, "value", e.target.value)
                      }
                      placeholder={
                        cond.operator === "between" ? "min,max" : "Valor"
                      }
                    />
                  )}
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    onClick={() => removeCondition(i)}
                  >
                    <Trash2 className="h-4 w-4 text-destructive" />
                  </Button>
                </div>
              ))}
              {editForm.conditions.length === 0 && (
                <p className="text-xs text-muted-foreground">
                  Sin condiciones. Agrega al menos una.
                </p>
              )}
            </div>

            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <Label className="flex items-center gap-1">
                  <Calculator className="h-3 w-3" /> Condiciones agregadas
                </Label>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={addAggregateCondition}
                >
                  <Plus className="mr-1 h-3 w-3" /> Agregar
                </Button>
              </div>
              {editForm.aggregateConditions.map((agg, i) => (
                <div
                  key={i}
                  className="rounded-md border bg-muted/30 p-3 space-y-2"
                >
                  <div className="grid grid-cols-[1fr_1fr_1fr_auto] gap-2">
                    <div className="space-y-1">
                      <span className="text-[10px] text-muted-foreground">
                        Funcion
                      </span>
                      <Select
                        value={agg.function}
                        onValueChange={(v) =>
                          updateAggCondition(i, "function", v)
                        }
                      >
                        <SelectTrigger className="h-8 text-xs">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="sum">SUM (Suma)</SelectItem>
                          <SelectItem value="count">COUNT (Contar)</SelectItem>
                          <SelectItem value="avg">AVG (Promedio)</SelectItem>
                          <SelectItem value="min">MIN (Mínimo)</SelectItem>
                          <SelectItem value="max">MAX (Máximo)</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-1">
                      <span className="text-[10px] text-muted-foreground">
                        Campo numerico
                      </span>
                      <Select
                        value={agg.field}
                        onValueChange={(v) => updateAggCondition(i, "field", v)}
                      >
                        <SelectTrigger className="h-8 text-xs">
                          <SelectValue placeholder="Campo" />
                        </SelectTrigger>
                        <SelectContent>
                          {aggFieldOptions?.map((f) => (
                            <SelectItem key={f.name} value={f.name}>
                              {f.name}
                            </SelectItem>
                          ))}
                          {!editMonitorSchema && agg.field && (
                            <SelectItem value={agg.field}>
                              {agg.field}
                            </SelectItem>
                          )}
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-1">
                      <span className="text-[10px] text-muted-foreground">
                        Agrupar por
                      </span>
                      <Select
                        value={agg.groupBy}
                        onValueChange={(v) =>
                          updateAggCondition(i, "groupBy", v)
                        }
                      >
                        <SelectTrigger className="h-8 text-xs">
                          <SelectValue placeholder="Campo" />
                        </SelectTrigger>
                        <SelectContent>
                          {editMonitorSchema?.map((f) => (
                            <SelectItem key={f.name} value={f.name}>
                              {f.name}
                            </SelectItem>
                          ))}
                          {!editMonitorSchema && agg.groupBy && (
                            <SelectItem value={agg.groupBy}>
                              {agg.groupBy}
                            </SelectItem>
                          )}
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="flex items-end">
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8"
                        onClick={() => removeAggCondition(i)}
                      >
                        <Trash2 className="h-4 w-4 text-destructive" />
                      </Button>
                    </div>
                  </div>
                  <div className="grid grid-cols-4 gap-2">
                    <div className="space-y-1">
                      <span className="text-[10px] text-muted-foreground">
                        Campo fecha
                      </span>
                      <Select
                        value={agg.timeField}
                        onValueChange={(v) =>
                          updateAggCondition(i, "timeField", v)
                        }
                      >
                        <SelectTrigger className="h-8 text-xs">
                          <SelectValue placeholder="Fecha" />
                        </SelectTrigger>
                        <SelectContent>
                          {editMonitorSchema?.map((f) => (
                            <SelectItem key={f.name} value={f.name}>
                              {f.name}
                            </SelectItem>
                          ))}
                          {!editMonitorSchema && agg.timeField && (
                            <SelectItem value={agg.timeField}>
                              {agg.timeField}
                            </SelectItem>
                          )}
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-1">
                      <span className="text-[10px] text-muted-foreground">
                        Ventana
                      </span>
                      <Select
                        value={agg.timeWindow}
                        onValueChange={(v) =>
                          updateAggCondition(i, "timeWindow", v)
                        }
                      >
                        <SelectTrigger className="h-8 text-xs">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="30s">30 segundos</SelectItem>
                          <SelectItem value="60s">60 segundos</SelectItem>
                          <SelectItem value="5min">5 minutos</SelectItem>
                          <SelectItem value="15min">15 minutos</SelectItem>
                          <SelectItem value="1h">1 hora</SelectItem>
                          <SelectItem value="6h">6 horas</SelectItem>
                          <SelectItem value="12h">12 horas</SelectItem>
                          <SelectItem value="24h">24 horas</SelectItem>
                          <SelectItem value="7d">7 dias</SelectItem>
                          <SelectItem value="15d">15 dias</SelectItem>
                          <SelectItem value="30d">30 dias</SelectItem>
                          <SelectItem value="90d">90 dias</SelectItem>
                          <SelectItem value="365d">1 año</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-1">
                      <span className="text-[10px] text-muted-foreground">
                        Operador
                      </span>
                      <Select
                        value={agg.operator}
                        onValueChange={(v) =>
                          updateAggCondition(i, "operator", v as Operator)
                        }
                      >
                        <SelectTrigger className="h-8 text-xs">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="gt">&gt; Mayor</SelectItem>
                          <SelectItem value="gte">≥ Mayor o igual</SelectItem>
                          <SelectItem value="lt">&lt; Menor</SelectItem>
                          <SelectItem value="lte">≤ Menor o igual</SelectItem>
                          <SelectItem value="eq">= Igual</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-1">
                      <span className="text-[10px] text-muted-foreground">
                        Umbral
                      </span>
                      <Input
                        type="number"
                        className="h-8 text-xs"
                        value={agg.threshold}
                        onChange={(e) =>
                          updateAggCondition(
                            i,
                            "threshold",
                            parseFloat(e.target.value) || 0,
                          )
                        }
                        placeholder="50000"
                      />
                    </div>
                  </div>
                  <p className="text-[10px] text-muted-foreground italic">
                    {agg.function.toUpperCase()}({agg.field || "?"}) agrupado
                    por {agg.groupBy || "?"} en ultimos {agg.timeWindow}{" "}
                    {operatorSymbol(agg.operator)}{" "}
                    {agg.threshold.toLocaleString()}
                  </p>
                </div>
              ))}
              {editForm.aggregateConditions.length === 0 && (
                <p className="text-xs text-muted-foreground">
                  Sin condiciones agregadas. Agrega una para detectar acumulados
                  por periodo.
                </p>
              )}
            </div>

            <div className="space-y-2">
              <Label>Descripción</Label>
              <Textarea
                value={editForm.description}
                onChange={(e) =>
                  setEditForm({ ...editForm, description: e.target.value })
                }
              />
            </div>

            <div className="space-y-2">
              <Label className="text-xs text-muted-foreground">
                Campos a screenear contra sanciones (opcional)
              </Label>
              <div className="flex flex-wrap gap-2">
                {(editMonitorSchema || []).map((f) => {
                  const active = editForm.screeningFields.includes(f.name);
                  return (
                    <button
                      key={f.name}
                      type="button"
                      onClick={() =>
                        setEditForm((prev) => ({
                          ...prev,
                          screeningFields: active
                            ? prev.screeningFields.filter((n) => n !== f.name)
                            : [...prev.screeningFields, f.name],
                        }))
                      }
                      className={`rounded-md border px-2 py-1 text-xs ${
                        active ? "border-primary bg-primary/10" : ""
                      }`}
                    >
                      {f.name}
                    </button>
                  );
                })}
              </div>
            </div>

            {/* Schedule */}
            <div className="space-y-2 rounded-md border p-3">
              <div className="flex items-center justify-between">
                <Label className="flex items-center gap-1">
                  <Clock className="h-3 w-3" /> Programacion
                </Label>
                <button
                  type="button"
                  role="switch"
                  aria-checked={editForm.scheduleEnabled}
                  onClick={() =>
                    setEditForm((prev) => ({
                      ...prev,
                      scheduleEnabled: !prev.scheduleEnabled,
                    }))
                  }
                  className={cn(
                    "relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors",
                    editForm.scheduleEnabled ? "bg-primary" : "bg-muted",
                  )}
                >
                  <span
                    className={cn(
                      "pointer-events-none block h-4 w-4 rounded-full bg-background shadow-lg ring-0 transition-transform",
                      editForm.scheduleEnabled
                        ? "translate-x-4"
                        : "translate-x-0",
                    )}
                  />
                </button>
              </div>
              {editForm.scheduleEnabled && (
                <div className="grid grid-cols-3 gap-2">
                  <div className="space-y-1">
                    <span className="text-[10px] text-muted-foreground">
                      Frecuencia
                    </span>
                    <Select
                      value={editForm.scheduleFrequency}
                      onValueChange={(v) =>
                        setEditForm((prev) => ({
                          ...prev,
                          scheduleFrequency: v,
                        }))
                      }
                    >
                      <SelectTrigger className="h-8 text-xs">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="daily">Diario</SelectItem>
                        <SelectItem value="weekly">Semanal</SelectItem>
                        <SelectItem value="monthly">Mensual</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  {editForm.scheduleFrequency === "weekly" && (
                    <div className="space-y-1">
                      <span className="text-[10px] text-muted-foreground">
                        Dia
                      </span>
                      <Select
                        value={editForm.scheduleDay}
                        onValueChange={(v) =>
                          setEditForm((prev) => ({ ...prev, scheduleDay: v }))
                        }
                      >
                        <SelectTrigger className="h-8 text-xs">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="1">Lunes</SelectItem>
                          <SelectItem value="2">Martes</SelectItem>
                          <SelectItem value="3">Miercoles</SelectItem>
                          <SelectItem value="4">Jueves</SelectItem>
                          <SelectItem value="5">Viernes</SelectItem>
                          <SelectItem value="6">Sabado</SelectItem>
                          <SelectItem value="0">Domingo</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                  )}
                  {editForm.scheduleFrequency === "monthly" && (
                    <div className="space-y-1">
                      <span className="text-[10px] text-muted-foreground">
                        Dia del mes
                      </span>
                      <Select
                        value={editForm.scheduleDay}
                        onValueChange={(v) =>
                          setEditForm((prev) => ({ ...prev, scheduleDay: v }))
                        }
                      >
                        <SelectTrigger className="h-8 text-xs">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {Array.from({ length: 28 }, (_, i) => (
                            <SelectItem key={i + 1} value={String(i + 1)}>
                              {i + 1}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                  )}
                  <div className="space-y-1">
                    <span className="text-[10px] text-muted-foreground">
                      Hora
                    </span>
                    <Select
                      value={editForm.scheduleTime}
                      onValueChange={(v) =>
                        setEditForm((prev) => ({ ...prev, scheduleTime: v }))
                      }
                    >
                      <SelectTrigger className="h-8 text-xs">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {TIME_OPTIONS.map((t) => (
                          <SelectItem key={t} value={t}>
                            {t}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                </div>
              )}
            </div>

            {backtestResult && (
              <div className="rounded-md border bg-muted/30 p-3 text-xs space-y-1">
                <p className="font-medium">
                  {backtestResult.matchCount === 0
                    ? "No habría generado ninguna alerta contra el histórico"
                    : `Habría generado ${backtestResult.matchCount} alerta(s) contra el histórico`}
                </p>
                {backtestResult.sample.slice(0, 5).map((rf, i) => (
                  <p key={i} className="text-muted-foreground truncate">
                    {rf.message}
                  </p>
                ))}
              </div>
            )}

            <div className="flex gap-2">
              <Button
                type="button"
                variant="outline"
                className="flex-1"
                disabled={backtesting || !editingRule}
                onClick={() =>
                  editingRule &&
                  runBacktest(
                    editingRule.monitorId,
                    editForm.logic,
                    editForm.conditions,
                    editForm.aggregateConditions,
                    editForm.severity,
                  )
                }
              >
                {backtesting ? "Probando…" : "Probar contra histórico"}
              </Button>
              <Button type="submit" className="flex-1">
                Guardar
              </Button>
            </div>
          </form>
        </DialogContent>
      </Dialog>

      {/* Create Chart from Rule Dialog */}
      <Dialog
        open={!!chartDialogRule}
        onOpenChange={(open) => !open && setChartDialogRule(null)}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Crear gráfico desde regla</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <p className="text-sm text-muted-foreground">
              Crear un widget visual basado en{" "}
              <strong>{chartDialogRule?.name}</strong>
            </p>
            <div className="space-y-2">
              <Label>Dashboard destino</Label>
              <Select
                value={chartForm.dashboardId}
                onValueChange={(v) =>
                  setChartForm({ ...chartForm, dashboardId: v })
                }
              >
                <SelectTrigger>
                  <SelectValue placeholder="Seleccionar dashboard" />
                </SelectTrigger>
                <SelectContent>
                  {dashboards.map((d) => (
                    <SelectItem key={d.id} value={d.id}>
                      {d.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label>Tipo de gráfico</Label>
              <Select
                value={chartForm.chartType}
                onValueChange={(v) =>
                  setChartForm({ ...chartForm, chartType: v as WidgetType })
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="bar_chart">Barras</SelectItem>
                  <SelectItem value="line_chart">Líneas</SelectItem>
                  <SelectItem value="pie_chart">Circular (Pie)</SelectItem>
                  <SelectItem value="area_chart">Área</SelectItem>
                  <SelectItem value="stat">Estadística</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label>Título del widget</Label>
              <Input
                value={chartForm.title}
                onChange={(e) =>
                  setChartForm({ ...chartForm, title: e.target.value })
                }
              />
            </div>
            <Button
              className="w-full"
              disabled={!chartForm.dashboardId || chartCreating}
              onClick={createChartFromRule}
            >
              {chartCreating ? "Creando..." : "Crear gráfico"}
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      {/* Delete confirmation dialog */}
      <Dialog
        open={!!deleteConfirm}
        onOpenChange={(open) => !open && setDeleteConfirm(null)}
      >
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-destructive">
              <Trash2 className="h-5 w-5" />
              Eliminar regla
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4 pt-2">
            <p className="text-sm">
              Estas a punto de eliminar la regla{" "}
              <strong>&quot;{deleteConfirm?.rule.name}&quot;</strong>. Esta
              accion no se puede deshacer.
            </p>
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">
                Escribe{" "}
                <span className="font-mono font-bold text-destructive">
                  BORRAR
                </span>{" "}
                para confirmar
              </Label>
              <Input
                value={deleteConfirm?.input ?? ""}
                onChange={(e) =>
                  setDeleteConfirm((prev) =>
                    prev ? { ...prev, input: e.target.value } : null,
                  )
                }
                placeholder="Escribe BORRAR"
                className="font-mono"
              />
            </div>
            <div className="flex justify-end gap-2">
              <Button variant="outline" onClick={() => setDeleteConfirm(null)}>
                Cancelar
              </Button>
              <Button
                variant="destructive"
                disabled={deleteConfirm?.input !== "BORRAR"}
                onClick={confirmDelete}
              >
                Eliminar regla
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      <TemplateGallery
        open={isTemplatesOpen}
        onOpenChange={setIsTemplatesOpen}
        monitors={monitors}
        defaultMonitorId={
          filterMonitorId !== "all" ? filterMonitorId : undefined
        }
        onCreated={loadRules}
      />
    </>
  );
}
