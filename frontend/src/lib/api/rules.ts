import { api } from "./client";
import type {
  Rule,
  RuleSchedule,
  AIRuleSuggestion,
  RedFlag,
  ConditionGroup,
  AggregateCondition,
  VelocityCondition,
  ActionType,
  Severity,
} from "@/lib/types";

export const rulesApi = {
  list: (monitorId?: string) => {
    const params = monitorId ? `?monitorId=${monitorId}` : "";
    return api.get<Rule[]>(`/rules${params}`);
  },

  get: (id: string) => api.get<Rule>(`/rules/${id}`),

  create: (data: {
    monitorId: string;
    name: string;
    description: string;
    conditionGroup: ConditionGroup;
    aggregateConditions?: AggregateCondition[];
    velocityConditions?: VelocityCondition[];
    actions: ActionType[];
    severity: Severity;
    schedule?: RuleSchedule;
    fatfTypology?: string;
    thresholdJustification?: string;
    regulatoryBasis?: string;
    screeningFields?: string[];
    aiGenerated?: boolean;
  }) => api.post<Rule>("/rules", data),

  update: (id: string, data: Partial<Rule>) =>
    api.put<{ message: string }>(`/rules/${id}`, data),

  delete: (id: string) => api.delete<{ message: string }>(`/rules/${id}`),

  execute: (id: string) =>
    api.post<{ redFlagsGenerated: number; redFlags: RedFlag[] }>(
      `/rules/${id}/execute`,
    ),

  generateAI: (data: {
    monitorId: string;
    prompt: string;
    dataSample?: string;
    fields?: string[];
  }) => api.post<AIGenerationResult>("/rules/ai-generate", data),

  // Tipologías AML pre-armadas: catálogo + instanciación por monitor.
  listTemplates: () => api.get<RuleTemplate[]>("/rule-templates"),

  instantiateTemplate: (
    monitorId: string,
    data: {
      templateId: string;
      fieldMap: Record<string, string>;
      params?: Record<string, number>;
      name: string;
    },
  ) => api.post<Rule>(`/monitors/${monitorId}/rules/from-template`, data),

  // Efectividad: cuenta cuántas red flags generó cada regla/tipología y
  // cómo se resolvieron (disposition de cada caso cerrado).
  effectiveness: (ruleId: string) =>
    api.get<EffectivenessStats>(`/rules/${ruleId}/effectiveness`),

  effectivenessByTemplate: () =>
    api.get<TemplateEffectiveness[]>("/rule-templates/effectiveness"),

  // Backtest: qué habría generado la regla contra el histórico ya
  // ingerido del monitor, sin guardar nada — para juzgar señal vs. ruido
  // antes de activarla.
  backtest: (
    monitorId: string,
    data: {
      conditionGroup: ConditionGroup;
      aggregateConditions?: AggregateCondition[];
      severity: Severity;
    },
  ) =>
    api.post<BacktestResult>(
      `/monitors/${monitorId}/rules/backtest`,
      data,
    ),
};

export interface BacktestResult {
  matchCount: number;
  sample: RedFlag[];
}

export interface DiscardedSuggestion {
  name: string;
  reason: string;
}

export interface AIGenerationResult {
  suggestions: AIRuleSuggestion[];
  generated: number;
  discarded?: DiscardedSuggestion[];
}

export interface EffectivenessStats {
  total: number;
  open: number;
  falsePositive: number;
  confirmedRos: number;
  noAction: number;
  falsePositiveRate: number;
  avgCloseHours: number;
}

export interface TemplateEffectiveness {
  templateId: string;
  name: string;
  stats: EffectivenessStats;
}

export interface RuleTemplate {
  id: string;
  name: string;
  description: string;
  suggestedSeverity: Severity;
  requiredFields: string[];
  params: {
    key: string;
    label: string;
    description: string;
    default: number;
    min: number;
  }[];
}
