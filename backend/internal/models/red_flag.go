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
	RedFlagEscalated    RedFlagStatus = "escalated"
	RedFlagResolved     RedFlagStatus = "resolved"
	RedFlagDismissed    RedFlagStatus = "dismissed"
)

// RedFlagDisposition es la disposición final obligatoria al cerrar un
// caso: queda como evidencia para auditoría y semilla del módulo de
// reportes regulatorios.
type RedFlagDisposition string

const (
	DispositionFalsePositive RedFlagDisposition = "false_positive"
	DispositionConfirmedROS  RedFlagDisposition = "confirmed_ros"
	DispositionNoAction      RedFlagDisposition = "no_action"
)

type RedFlagType string

const (
	RedFlagTypeRow       RedFlagType = "row"
	RedFlagTypeAggregate RedFlagType = "aggregate"
	RedFlagTypeVelocity  RedFlagType = "velocity"
)

// EsAgrupada dice si la alerta describe un grupo de registros —y por lo
// tanto trae GroupByField, GroupByValue y MatchCount— en vez de una fila
// suelta. Existe como predicado con nombre porque seis sitios preguntaban
// `== RedFlagTypeAggregate` queriendo saber esto, y al aparecer un segundo
// tipo agrupado los seis habrían quedado mal en silencio.
func (t RedFlagType) EsAgrupada() bool {
	return t == RedFlagTypeAggregate || t == RedFlagTypeVelocity
}

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

	// Capa de caso (investigación)
	AssigneeID           *primitive.ObjectID `bson:"assignee_id,omitempty" json:"assigneeId,omitempty"`
	Priority             int                 `bson:"priority,omitempty" json:"priority,omitempty"` // 1 = más urgente
	SLADueAt             *time.Time          `bson:"sla_due_at,omitempty" json:"slaDueAt,omitempty"`
	EscalationNotifiedAt *time.Time          `bson:"escalation_notified_at,omitempty" json:"escalationNotifiedAt,omitempty"`
	Disposition          RedFlagDisposition  `bson:"disposition,omitempty" json:"disposition,omitempty"`
	ClosedAt             *time.Time          `bson:"closed_at,omitempty" json:"closedAt,omitempty"`
	ClosedBy             *primitive.ObjectID `bson:"closed_by,omitempty" json:"closedBy,omitempty"`

	CreatedAt time.Time `bson:"created_at" json:"createdAt"`
	UpdatedAt time.Time `bson:"updated_at" json:"updatedAt"`
}

// SLADefaultForSeverity son los plazos de primera respuesta por severidad.
// Configurables en el futuro via system_config; hoy, defaults en código.
func SLADefaultForSeverity(s Severity) time.Duration {
	switch s {
	case SeverityCritical:
		return 24 * time.Hour
	case SeverityHigh:
		return 72 * time.Hour
	case SeverityMedium:
		return 7 * 24 * time.Hour
	default:
		return 30 * 24 * time.Hour
	}
}

// RedFlagNote es una entrada del timeline de investigación de un caso.
// Inmutable por diseño: no existe Update ni Delete para esta colección.
type RedFlagNote struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	RedFlagID  primitive.ObjectID `bson:"red_flag_id" json:"redFlagId"`
	AuthorID   primitive.ObjectID `bson:"author_id" json:"authorId"`
	AuthorName string             `bson:"author_name" json:"authorName"`
	Text       string             `bson:"text" json:"text"`
	CreatedAt  time.Time          `bson:"created_at" json:"createdAt"`
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
