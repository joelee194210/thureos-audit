export type Role = "admin" | "compliance" | "viewer";

export interface User {
  id: string;
  email: string;
  name: string;
  role: Role;
  active: boolean;
  createdAt: string;
}

export type SourceType = "csv" | "excel" | "json" | "api";

export interface SchemaField {
  name: string;
  type: "string" | "number" | "date" | "boolean";
  required: boolean;
  sample: string;
}

export interface Monitor {
  id: string;
  name: string;
  description: string;
  sourceType: SourceType;
  schema: SchemaField[];
  collectionId: string;
  apiEndpoint?: string;
  schedule?: string;
  ownerId: string;
  recordCount: number;
  lastIngested?: string;
  createdAt: string;
  updatedAt: string;
}

export type Operator =
  | "eq" | "neq" | "gt" | "lt" | "gte" | "lte"
  | "contains" | "regex" | "in" | "not_in"
  | "between" | "is_null" | "is_not_null"
  | "starts_with" | "ends_with";

export type Severity = "low" | "medium" | "high" | "critical";
export type ActionType = "alert" | "flag" | "block" | "log";

export interface Condition {
  field: string;
  operator: Operator;
  value: unknown;
}

export interface ConditionGroup {
  logic: "AND" | "OR";
  conditions: Condition[];
}

export type AggFunction = "sum" | "count" | "avg" | "min" | "max";

export interface AggregateCondition {
  field: string;
  function: AggFunction;
  groupBy: string;
  timeField: string;
  timeWindow: string; // e.g. "24h", "7d", "30d"
  operator: Operator;
  threshold: number;
}

export type SchedulePreset =
  | "" | "daily_6am" | "daily_8am" | "daily_12pm" | "daily_6pm"
  | "weekly_mon_8am" | "weekly_fri_6pm" | "monthly_1st_6am";

export interface RuleSchedule {
  enabled: boolean;
  preset: SchedulePreset;
  cronExpr: string;
  lastEvaluated?: string;
  nextRun?: string;
}

export interface Rule {
  id: string;
  monitorId: string;
  name: string;
  description: string;
  conditionGroup: ConditionGroup;
  aggregateConditions?: AggregateCondition[];
  actions: ActionType[];
  severity: Severity;
  active: boolean;
  aiGenerated: boolean;
  triggerCount: number;
  lastTriggered?: string;
  schedule?: RuleSchedule;
  fatfTypology?: string;
  thresholdJustification?: string;
  regulatoryBasis?: string;
  version: number;
  deletedAt?: string;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
}

export type AlertStatus = "new" | "acknowledged" | "resolved" | "dismissed";
export type AlertType = "row" | "aggregate";

export interface Alert {
  id: string;
  fingerprint: string;
  monitorId: string;
  ruleId: string;
  ruleName: string;
  monitorName: string;
  severity: Severity;
  status: AlertStatus;
  message: string;
  alertType: AlertType;
  matchedData: Record<string, unknown>;
  matchedRecords?: Record<string, unknown>[];
  matchCount: number;
  aggField?: string;
  aggFunction?: string;
  aggValue?: number;
  groupByField?: string;
  groupByValue?: string;
  threshold?: number;
  acknowledgedBy?: string;
  createdAt: string;
  updatedAt: string;
}

export type WidgetType =
  | "bar_chart" | "line_chart" | "pie_chart" | "area_chart"
  | "table" | "stat" | "alert_list" | "timeline";

export type AggregationType = "count" | "sum" | "avg" | "min" | "max" | "distinct";

export interface Widget {
  id: string;
  title: string;
  type: WidgetType;
  monitorId: string;
  field: string;
  aggregation: AggregationType;
  groupBy?: string;
  position: { x: number; y: number; w: number; h: number };
}

export interface Dashboard {
  id: string;
  name: string;
  description: string;
  widgets: Widget[];
  monitorIds: string[];
  ownerId: string;
  isPublic: boolean;
  createdAt: string;
}

export type MCCNetwork = "visa" | "mastercard" | "unionpay" | "amex" | "discover" | "diners" | "jcb" | "all";
export type MCCRiskLevel = "high" | "medium" | "low";

export interface MCC {
  id: string;
  code: string;
  description: string;
  category: string;
  networks: MCCNetwork[];
  riskLevel: MCCRiskLevel;
}

export type CountryRiskLevel = "high" | "medium" | "low";

export interface Country {
  id: string;
  code: string;
  code3: string;
  name: string;
  nameEn: string;
  region: string;
  riskLevel: CountryRiskLevel;
  riskSources: string[];
  active: boolean;
  notes: string;
  updatedAt: string;
  updatedBy: string;
  createdAt: string;
}

export type ActionCategory = "investigation" | "false_positive" | "escalation" | "corrective_action" | "other";

export interface AlertLog {
  id: string;
  alertId: string;
  previousStatus: AlertStatus;
  newStatus: AlertStatus;
  userId: string;
  userName: string;
  userEmail: string;
  category: ActionCategory;
  notes: string;
  alertSnapshot: Alert;
  createdAt: string;
}

export type ActivityType = "login" | "alert_action" | "rule_create" | "rule_update" | "rule_delete" | "rule_execute" | "upload" | "user_manage" | "mcc_update";

export interface ActivityLogEntry {
  id: string;
  userId: string;
  userName: string;
  userEmail: string;
  action: ActivityType;
  detail: string;
  resource: string;
  resourceId?: string;
  ip: string;
  createdAt: string;
}

export interface AIRuleSuggestion {
  name: string;
  description: string;
  conditionGroup: ConditionGroup;
  severity: Severity;
  actions: ActionType[];
  reasoning: string;
}
