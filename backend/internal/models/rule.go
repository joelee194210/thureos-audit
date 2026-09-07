package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Operator string

const (
	OpEqual        Operator = "eq"
	OpNotEqual     Operator = "neq"
	OpGreaterThan  Operator = "gt"
	OpLessThan     Operator = "lt"
	OpGreaterEqual Operator = "gte"
	OpLessEqual    Operator = "lte"
	OpContains     Operator = "contains"
	OpRegex        Operator = "regex"
	OpIn           Operator = "in"
	OpNotIn        Operator = "not_in"
	OpBetween      Operator = "between"
	OpIsNull       Operator = "is_null"
	OpIsNotNull    Operator = "is_not_null"
	OpStartsWith   Operator = "starts_with"
	OpEndsWith     Operator = "ends_with"
)

type LogicOperator string

const (
	LogicAND LogicOperator = "AND"
	LogicOR  LogicOperator = "OR"
)

type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

type ActionType string

const (
	ActionRedFlag ActionType = "red_flag"
	ActionFlag    ActionType = "flag"
	ActionBlock   ActionType = "block"
	ActionLog     ActionType = "log"
)

type Condition struct {
	Field    string      `bson:"field" json:"field"`
	Operator Operator    `bson:"operator" json:"operator"`
	Value    interface{} `bson:"value" json:"value"`
}

// AggFunction defines the aggregation function for aggregate conditions
type AggFunction string

const (
	AggFuncSum   AggFunction = "sum"
	AggFuncCount AggFunction = "count"
	AggFuncAvg   AggFunction = "avg"
	AggFuncMin   AggFunction = "min"
	AggFuncMax   AggFunction = "max"
)

// AggregateCondition evaluates an aggregate (SUM, COUNT, etc.) over a time window.
// Example: SUM of "monto" grouped by "nombrecliente" in last "30d" > 50000
type AggregateCondition struct {
	Field      string      `bson:"field" json:"field"`            // numeric field to aggregate
	Function   AggFunction `bson:"function" json:"function"`      // sum, count, avg, min, max
	GroupBy    string      `bson:"group_by" json:"groupBy"`       // field to group by (e.g. client)
	TimeField  string      `bson:"time_field" json:"timeField"`   // date field for the window
	TimeWindow string      `bson:"time_window" json:"timeWindow"` // duration: "24h", "7d", "30d"
	Operator   Operator    `bson:"operator" json:"operator"`      // comparison: gt, gte, lt, etc.
	Threshold  float64     `bson:"threshold" json:"threshold"`    // value to compare against
	// Filter reduce el universo ANTES de agrupar/agregar (ej. solo montos
	// en la franja de estructuración) — opcional, aditivo: una regla sin
	// Filter agrega sobre todos los registros, igual que siempre.
	Filter []Condition `bson:"filter,omitempty" json:"filter,omitempty"`
}

type ConditionGroup struct {
	Logic      LogicOperator `bson:"logic" json:"logic"`
	Conditions []Condition   `bson:"conditions" json:"conditions"`
}

// SchedulePreset defines named schedule presets for rule evaluation
type SchedulePreset string

const (
	ScheduleDaily6AM  SchedulePreset = "daily_6am"
	ScheduleDaily8AM  SchedulePreset = "daily_8am"
	ScheduleDaily12PM SchedulePreset = "daily_12pm"
	ScheduleDaily6PM  SchedulePreset = "daily_6pm"
	ScheduleWeeklyMon SchedulePreset = "weekly_mon_8am"
	ScheduleWeeklyFri SchedulePreset = "weekly_fri_6pm"
	ScheduleMonthly   SchedulePreset = "monthly_1st_6am"
)

// PresetToCron maps a SchedulePreset to its cron expression (minute hour day month weekday)
func PresetToCron(preset SchedulePreset) string {
	switch preset {
	case ScheduleDaily6AM:
		return "0 6 * * *"
	case ScheduleDaily8AM:
		return "0 8 * * *"
	case ScheduleDaily12PM:
		return "0 12 * * *"
	case ScheduleDaily6PM:
		return "0 18 * * *"
	case ScheduleWeeklyMon:
		return "0 8 * * 1"
	case ScheduleWeeklyFri:
		return "0 18 * * 5"
	case ScheduleMonthly:
		return "0 6 1 * *"
	default:
		return ""
	}
}

// RuleSchedule configures automatic scheduled evaluation for a rule
type RuleSchedule struct {
	Enabled       bool           `bson:"enabled" json:"enabled"`
	Preset        SchedulePreset `bson:"preset" json:"preset"`
	CronExpr      string         `bson:"cron_expr" json:"cronExpr"`
	LastEvaluated *time.Time     `bson:"last_evaluated,omitempty" json:"lastEvaluated,omitempty"`
	NextRun       *time.Time     `bson:"next_run,omitempty" json:"nextRun,omitempty"`
}

type Rule struct {
	ID                     primitive.ObjectID   `bson:"_id,omitempty" json:"id"`
	MonitorID              primitive.ObjectID   `bson:"monitor_id" json:"monitorId"`
	Name                   string               `bson:"name" json:"name"`
	Description            string               `bson:"description" json:"description"`
	ConditionGroup         ConditionGroup       `bson:"condition_group" json:"conditionGroup"`
	AggregateConditions    []AggregateCondition `bson:"aggregate_conditions,omitempty" json:"aggregateConditions,omitempty"`
	Actions                []ActionType         `bson:"actions" json:"actions"`
	Severity               Severity             `bson:"severity" json:"severity"`
	Active                 bool                 `bson:"active" json:"active"`
	AIGenerated            bool                 `bson:"ai_generated" json:"aiGenerated"`
	TriggerCount           int64                `bson:"trigger_count" json:"triggerCount"`
	LastTriggered          *time.Time           `bson:"last_triggered,omitempty" json:"lastTriggered,omitempty"`
	Schedule               RuleSchedule         `bson:"schedule,omitempty" json:"schedule,omitempty"`
	TemplateID             string               `bson:"template_id,omitempty" json:"templateId,omitempty"`
	FatfTypology           string               `bson:"fatf_typology,omitempty" json:"fatfTypology,omitempty"`
	ThresholdJustification string               `bson:"threshold_justification,omitempty" json:"thresholdJustification,omitempty"`
	RegulatoryBasis        string               `bson:"regulatory_basis,omitempty" json:"regulatoryBasis,omitempty"`
	Version                int                  `bson:"version" json:"version"`
	DeletedAt              *time.Time           `bson:"deleted_at,omitempty" json:"deletedAt,omitempty"`
	CreatedBy              primitive.ObjectID   `bson:"created_by" json:"createdBy"`
	CreatedAt              time.Time            `bson:"created_at" json:"createdAt"`
	UpdatedAt              time.Time            `bson:"updated_at" json:"updatedAt"`
}

type CreateRuleRequest struct {
	MonitorID              string               `json:"monitorId"`
	Name                   string               `json:"name"`
	Description            string               `json:"description"`
	ConditionGroup         ConditionGroup       `json:"conditionGroup"`
	AggregateConditions    []AggregateCondition `json:"aggregateConditions,omitempty"`
	Actions                []ActionType         `json:"actions"`
	Severity               Severity             `json:"severity"`
	Schedule               *RuleSchedule        `json:"schedule,omitempty"`
	FatfTypology           string               `json:"fatfTypology,omitempty"`
	ThresholdJustification string               `json:"thresholdJustification,omitempty"`
	RegulatoryBasis        string               `json:"regulatoryBasis,omitempty"`
	AIGenerated            bool                 `json:"aiGenerated,omitempty"`
}

type AIRuleRequest struct {
	MonitorID  string `json:"monitorId"`
	Prompt     string `json:"prompt"`
	DataSample string `json:"dataSample,omitempty"`
}
