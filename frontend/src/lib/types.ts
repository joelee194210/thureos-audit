export type Role = "admin" | "compliance" | "viewer";

export interface User {
  id: string;
  email: string;
  name: string;
  role: Role;
  active: boolean;
  createdAt: string;
}

export type SourceType = "csv" | "excel" | "json" | "txt" | "api";

export type APIMode = "push" | "pull";
export type APIAuthType = "none" | "api_key_header" | "bearer";

// Lo que devuelve el backend en Monitor.sourceConfig — sin pushToken ni
// pullAuthValue, que nunca se serializan (json:"-" en el backend).
export interface SourceConfig {
  delimiter?: string;
  hasHeaderRow?: boolean;
  sheetName?: string;
  rootPath?: string;
  mode?: APIMode;
  pullUrl?: string;
  pullMethod?: string;
  pullAuthType?: APIAuthType;
  pullAuthHeaderName?: string;
  pullIntervalMinutes?: number;
  nextPullAt?: string;
  lastPullAt?: string;
  lastPullStatus?: "ok" | "error";
  lastPullError?: string;
}

// Lo que se envía al crear un monitor — sí incluye pullAuthValue, que es
// de solo-escritura (el backend lo acepta pero nunca lo devuelve).
export interface CreateSourceConfig {
  delimiter?: string;
  hasHeaderRow?: boolean;
  sheetName?: string;
  rootPath?: string;
  mode?: APIMode;
  pullUrl?: string;
  pullMethod?: string;
  pullAuthType?: APIAuthType;
  pullAuthHeaderName?: string;
  pullAuthValue?: string;
  pullIntervalMinutes?: number;
}

export interface SchemaField {
  name: string;
  type: "string" | "number" | "date" | "boolean";
  required: boolean;
  sample: string;
  impliedDecimals?: number;
  dateFormat?: string;
}

export type UploadStatus = "accepted" | "partial" | "rejected_structure" | "approved";

export interface FieldTypeMismatch {
  field: string;
  expectedType: string;
  actualType: string;
}

export interface SchemaDiff {
  match: boolean;
  missingFields?: string[];
  extraFields?: string[];
  typeMismatches?: FieldTypeMismatch[];
}

export interface RowRejection {
  rowIndex: number;
  field: string;
  reason: string;
}

export interface UploadLogEntry {
  id: string;
  monitorId: string;
  monitorName: string;
  fileName: string;
  sourceType: SourceType;
  uploadedByEmail: string;
  uploadedAt: string;
  status: UploadStatus;
  totalRows: number;
  rowsAccepted: number;
  rowsRejected: number;
  schemaDiff?: SchemaDiff;
  rowRejections?: RowRejection[];
}

export interface UploadCheckResult {
  match: boolean;
  missingFields?: string[];
  extraFields?: string[];
  typeMismatches?: FieldTypeMismatch[];
  totalRows: number;
  rowsThatWouldPass: number;
  rowsThatWouldFail: number;
}

export interface Monitor {
  id: string;
  name: string;
  description: string;
  sourceType: SourceType;
  sourceConfig?: SourceConfig;
  schema: SchemaField[];
  collectionId: string;
  ownerId: string;
  recordCount: number;
  lastIngested?: string;
  createdAt: string;
  updatedAt: string;
}

export type Operator =
  | "eq"
  | "neq"
  | "gt"
  | "lt"
  | "gte"
  | "lte"
  | "contains"
  | "regex"
  | "in"
  | "not_in"
  | "between"
  | "is_null"
  | "is_not_null"
  | "starts_with"
  | "ends_with";

export type Severity = "low" | "medium" | "high" | "critical";
export type ActionType = "red_flag" | "flag" | "block" | "log";

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
  | ""
  | "daily_6am"
  | "daily_8am"
  | "daily_12pm"
  | "daily_6pm"
  | "weekly_mon_8am"
  | "weekly_fri_6pm"
  | "monthly_1st_6am";

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
  screeningFields?: string[];
  version: number;
  deletedAt?: string;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
}

export type RedFlagStatus =
  "new" | "acknowledged" | "escalated" | "resolved" | "dismissed";
export type RedFlagType = "row" | "aggregate";

export interface RedFlag {
  id: string;
  fingerprint: string;
  monitorId: string;
  ruleId: string;
  ruleName: string;
  monitorName: string;
  severity: Severity;
  status: RedFlagStatus;
  message: string;
  redFlagType: RedFlagType;
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
  assigneeId?: string;
  priority?: number;
  slaDueAt?: string;
  disposition?: "false_positive" | "confirmed_ros" | "no_action";
  closedAt?: string;
  closedBy?: string;
  createdAt: string;
  updatedAt: string;
}

export type RedFlagDisposition =
  "false_positive" | "confirmed_ros" | "no_action";

export const CASE_LABELS: Record<string, string> = {
  new: "Nueva",
  acknowledged: "En investigación",
  escalated: "Escalada",
  resolved: "Cerrada (resuelta)",
  dismissed: "Cerrada (descartada)",
};

export type WidgetType =
  | "bar_chart"
  | "line_chart"
  | "pie_chart"
  | "area_chart"
  | "table"
  | "stat"
  | "red_flag_list"
  | "timeline";

export type AggregationType =
  "count" | "sum" | "avg" | "min" | "max" | "distinct";

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

export type MCCNetwork =
  | "visa"
  | "mastercard"
  | "unionpay"
  | "amex"
  | "discover"
  | "diners"
  | "jcb"
  | "all";
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

export type ActionCategory =
  | "investigation"
  | "false_positive"
  | "escalation"
  | "corrective_action"
  | "other";

export interface RedFlagLog {
  id: string;
  redFlagId: string;
  previousStatus: RedFlagStatus;
  newStatus: RedFlagStatus;
  userId: string;
  userName: string;
  userEmail: string;
  category: ActionCategory;
  notes: string;
  redFlagSnapshot: RedFlag;
  createdAt: string;
}

export type ActivityType =
  | "login"
  | "red_flag_action"
  | "rule_create"
  | "rule_update"
  | "rule_delete"
  | "rule_execute"
  | "upload"
  | "user_manage"
  | "mcc_update";

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
  aggregateConditions?: AggregateCondition[];
  severity: Severity;
  actions: ActionType[];
  reasoning: string;
}
