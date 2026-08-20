package models

import (
	"crypto/sha256"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AlertStatus string

const (
	AlertNew          AlertStatus = "new"
	AlertAcknowledged AlertStatus = "acknowledged"
	AlertResolved     AlertStatus = "resolved"
	AlertDismissed    AlertStatus = "dismissed"
)

type AlertType string

const (
	AlertTypeRow       AlertType = "row"
	AlertTypeAggregate AlertType = "aggregate"
)

type Alert struct {
	ID             primitive.ObjectID       `bson:"_id,omitempty" json:"id"`
	Fingerprint    string                   `bson:"fingerprint" json:"fingerprint"`
	MonitorID      primitive.ObjectID       `bson:"monitor_id" json:"monitorId"`
	RuleID         primitive.ObjectID       `bson:"rule_id" json:"ruleId"`
	RuleName       string                   `bson:"rule_name" json:"ruleName"`
	MonitorName    string                   `bson:"monitor_name" json:"monitorName"`
	Severity       Severity                 `bson:"severity" json:"severity"`
	Status         AlertStatus              `bson:"status" json:"status"`
	Message        string                   `bson:"message" json:"message"`
	AlertType      AlertType                `bson:"alert_type" json:"alertType"`
	MatchedData    map[string]interface{}   `bson:"matched_data" json:"matchedData"`
	MatchedRecords []map[string]interface{} `bson:"matched_records,omitempty" json:"matchedRecords,omitempty"`
	MatchCount     int                      `bson:"match_count" json:"matchCount"`
	AggField       string                   `bson:"agg_field,omitempty" json:"aggField,omitempty"`
	AggFunction    string                   `bson:"agg_function,omitempty" json:"aggFunction,omitempty"`
	AggValue       float64                  `bson:"agg_value,omitempty" json:"aggValue,omitempty"`
	GroupByField   string                   `bson:"group_by_field,omitempty" json:"groupByField,omitempty"`
	GroupByValue   string                   `bson:"group_by_value,omitempty" json:"groupByValue,omitempty"`
	Threshold      float64                  `bson:"threshold,omitempty" json:"threshold,omitempty"`
	AcknowledgedBy *primitive.ObjectID      `bson:"acknowledged_by,omitempty" json:"acknowledgedBy,omitempty"`
	CreatedAt      time.Time                `bson:"created_at" json:"createdAt"`
	UpdatedAt      time.Time                `bson:"updated_at" json:"updatedAt"`
}

// RowAlertFingerprint generates a dedup key for row-level alerts.
// Same rule + monitor + day = same fingerprint (no duplicates on re-execution).
func RowAlertFingerprint(ruleID, monitorID primitive.ObjectID, day time.Time) string {
	raw := fmt.Sprintf("row:%s:%s:%s", ruleID.Hex(), monitorID.Hex(), day.Format("2006-01-02"))
	h := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", h[:16])
}

// AggAlertFingerprint generates a dedup key for aggregate alerts.
// Same rule + monitor + day + groupByValue = same fingerprint.
func AggAlertFingerprint(ruleID, monitorID primitive.ObjectID, day time.Time, groupByValue string) string {
	raw := fmt.Sprintf("agg:%s:%s:%s:%s", ruleID.Hex(), monitorID.Hex(), day.Format("2006-01-02"), groupByValue)
	h := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", h[:16])
}
