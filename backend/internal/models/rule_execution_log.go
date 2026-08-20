package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RuleExecutionLog records each rule evaluation for audit trail and performance monitoring.
// Regulators may request evidence that monitoring rules are being executed as configured.
type RuleExecutionLog struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	RuleID         primitive.ObjectID `bson:"rule_id" json:"ruleId"`
	RuleName       string             `bson:"rule_name" json:"ruleName"`
	MonitorID      primitive.ObjectID `bson:"monitor_id" json:"monitorId"`
	MonitorName    string             `bson:"monitor_name" json:"monitorName"`
	RecordsScanned int                `bson:"records_scanned" json:"recordsScanned"`
	AlertsGenerated int               `bson:"alerts_generated" json:"alertsGenerated"`
	DurationMs     int64              `bson:"duration_ms" json:"durationMs"`
	Trigger        string             `bson:"trigger" json:"trigger"` // "scheduled", "manual", "upload"
	Success        bool               `bson:"success" json:"success"`
	ErrorMessage   string             `bson:"error_message,omitempty" json:"errorMessage,omitempty"`
	ExecutedAt     time.Time          `bson:"executed_at" json:"executedAt"`
}
