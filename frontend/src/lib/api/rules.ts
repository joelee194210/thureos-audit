import { api } from "./client";
import type {
  Rule,
  RuleSchedule,
  AIRuleSuggestion,
  RedFlag,
  ConditionGroup,
  AggregateCondition,
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
    actions: ActionType[];
    severity: Severity;
    schedule?: RuleSchedule;
    fatfTypology?: string;
    thresholdJustification?: string;
    regulatoryBasis?: string;
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
  }) =>
    api.post<{ suggestions: AIRuleSuggestion[] }>("/rules/ai-generate", data),

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
};

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
