package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ActionCategory string

const (
	CategoryInvestigation    ActionCategory = "investigation"
	CategoryFalsePositive    ActionCategory = "false_positive"
	CategoryEscalation       ActionCategory = "escalation"
	CategoryCorrectiveAction ActionCategory = "corrective_action"
	CategoryOther            ActionCategory = "other"
)

type AlertLog struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	AlertID        primitive.ObjectID `bson:"alert_id" json:"alertId"`
	PreviousStatus AlertStatus        `bson:"previous_status" json:"previousStatus"`
	NewStatus      AlertStatus        `bson:"new_status" json:"newStatus"`
	UserID         primitive.ObjectID `bson:"user_id" json:"userId"`
	UserName       string             `bson:"user_name" json:"userName"`
	UserEmail      string             `bson:"user_email" json:"userEmail"`
	Category       ActionCategory     `bson:"category" json:"category"`
	Notes          string             `bson:"notes" json:"notes"`
	AlertSnapshot  Alert              `bson:"alert_snapshot" json:"alertSnapshot"`
	CreatedAt      time.Time          `bson:"created_at" json:"createdAt"`
}
