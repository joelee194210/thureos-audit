package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type WidgetType string

const (
	WidgetBarChart    WidgetType = "bar_chart"
	WidgetLineChart   WidgetType = "line_chart"
	WidgetPieChart    WidgetType = "pie_chart"
	WidgetAreaChart   WidgetType = "area_chart"
	WidgetTable       WidgetType = "table"
	WidgetStat        WidgetType = "stat"
	WidgetRedFlagList WidgetType = "red_flag_list"
	WidgetTimeline    WidgetType = "timeline"
)

type AggregationType string

const (
	AggCount    AggregationType = "count"
	AggSum      AggregationType = "sum"
	AggAvg      AggregationType = "avg"
	AggMin      AggregationType = "min"
	AggMax      AggregationType = "max"
	AggDistinct AggregationType = "distinct"
)

type Widget struct {
	ID          string                 `bson:"id" json:"id"`
	Title       string                 `bson:"title" json:"title"`
	Type        WidgetType             `bson:"type" json:"type"`
	MonitorID   string                 `bson:"monitor_id" json:"monitorId"`
	Field       string                 `bson:"field" json:"field"`
	Aggregation AggregationType        `bson:"aggregation" json:"aggregation"`
	GroupBy     string                 `bson:"group_by,omitempty" json:"groupBy,omitempty"`
	RuleID      string                 `bson:"rule_id,omitempty" json:"ruleId,omitempty"`
	Filters     []Condition            `bson:"filters,omitempty" json:"filters,omitempty"`
	Position    WidgetPosition         `bson:"position" json:"position"`
	Config      map[string]interface{} `bson:"config,omitempty" json:"config,omitempty"`
}

type WidgetPosition struct {
	X int `bson:"x" json:"x"`
	Y int `bson:"y" json:"y"`
	W int `bson:"w" json:"w"`
	H int `bson:"h" json:"h"`
}

type Dashboard struct {
	ID          primitive.ObjectID   `bson:"_id,omitempty" json:"id"`
	Name        string               `bson:"name" json:"name"`
	Description string               `bson:"description" json:"description"`
	Widgets     []Widget             `bson:"widgets" json:"widgets"`
	MonitorIDs  []primitive.ObjectID `bson:"monitor_ids" json:"monitorIds"`
	OwnerID     primitive.ObjectID   `bson:"owner_id" json:"ownerId"`
	SharedWith  []primitive.ObjectID `bson:"shared_with,omitempty" json:"sharedWith,omitempty"`
	IsPublic    bool                 `bson:"is_public" json:"isPublic"`
	CreatedAt   time.Time            `bson:"created_at" json:"createdAt"`
	UpdatedAt   time.Time            `bson:"updated_at" json:"updatedAt"`
}

type CreateDashboardRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	MonitorIDs  []string `json:"monitorIds"`
}

type AddWidgetRequest struct {
	Title       string          `json:"title"`
	Type        WidgetType      `json:"type"`
	MonitorID   string          `json:"monitorId"`
	Field       string          `json:"field"`
	Aggregation AggregationType `json:"aggregation"`
	GroupBy     string          `json:"groupBy,omitempty"`
	RuleID      string          `json:"ruleId,omitempty"`
	Position    WidgetPosition  `json:"position"`
}

type DrilldownRequest struct {
	GroupValue string `json:"groupValue"`
	Page       int    `json:"page"`
	PageSize   int    `json:"pageSize"`
	Search     string `json:"search"`
	Export     bool   `json:"export"`
}
