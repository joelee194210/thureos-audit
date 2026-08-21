package models

import (
	"crypto/sha256"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RedFlagStatus string

const (
	RedFlagNew          RedFlagStatus = "new"
	RedFlagAcknowledged RedFlagStatus = "acknowledged"
	RedFlagResolved     RedFlagStatus = "resolved"
	RedFlagDismissed    RedFlagStatus = "dismissed"
)

type RedFlagType string

const (
	RedFlagTypeRow       RedFlagType = "row"
	RedFlagTypeAggregate RedFlagType = "aggregate"
)

type RedFlag struct {
	ID             primitive.ObjectID       `bson:"_id,omitempty" json:"id"`
	Fingerprint    string                   `bson:"fingerprint" json:"fingerprint"`
	MonitorID      primitive.ObjectID       `bson:"monitor_id" json:"monitorId"`
	RuleID         primitive.ObjectID       `bson:"rule_id" json:"ruleId"`
	RuleName       string                   `bson:"rule_name" json:"ruleName"`
	MonitorName    string                   `bson:"monitor_name" json:"monitorName"`
	Severity       Severity                 `bson:"severity" json:"severity"`
	Status         RedFlagStatus            `bson:"status" json:"status"`
	Message        string                   `bson:"message" json:"message"`
	RedFlagType    RedFlagType              `bson:"red_flag_type" json:"redFlagType"`
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

// RowRedFlagFingerprint generates a dedup key for row-level red flags.
// Same rule + monitor + day = same fingerprint (no duplicates on re-execution).
func RowRedFlagFingerprint(ruleID, monitorID primitive.ObjectID, day time.Time) string {
	raw := fmt.Sprintf("row:%s:%s:%s", ruleID.Hex(), monitorID.Hex(), day.Format("2006-01-02"))
	h := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", h[:16])
}

// AggRedFlagFingerprint generates a dedup key for aggregate red flags.
// Same rule + monitor + day + groupByValue = same fingerprint.
func AggRedFlagFingerprint(ruleID, monitorID primitive.ObjectID, day time.Time, groupByValue string) string {
	raw := fmt.Sprintf("agg:%s:%s:%s:%s", ruleID.Hex(), monitorID.Hex(), day.Format("2006-01-02"), groupByValue)
	h := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", h[:16])
}
