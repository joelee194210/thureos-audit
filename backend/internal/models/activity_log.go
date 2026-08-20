package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ActivityType string

const (
	ActivityLogin       ActivityType = "login"
	ActivityAlertAction ActivityType = "alert_action"
	ActivityRuleCreate  ActivityType = "rule_create"
	ActivityRuleUpdate  ActivityType = "rule_update"
	ActivityRuleDelete  ActivityType = "rule_delete"
	ActivityRuleExecute ActivityType = "rule_execute"
	ActivityUpload      ActivityType = "upload"
	ActivityUserManage  ActivityType = "user_manage"
	ActivityMCCUpdate   ActivityType = "mcc_update"
)

type ActivityLog struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID     primitive.ObjectID `bson:"user_id" json:"userId"`
	UserName   string             `bson:"user_name" json:"userName"`
	UserEmail  string             `bson:"user_email" json:"userEmail"`
	Action     ActivityType       `bson:"action" json:"action"`
	Detail     string             `bson:"detail" json:"detail"`
	Resource   string             `bson:"resource" json:"resource"`
	ResourceID string             `bson:"resource_id,omitempty" json:"resourceId,omitempty"`
	IP         string             `bson:"ip" json:"ip"`
	CreatedAt  time.Time          `bson:"created_at" json:"createdAt"`
}
