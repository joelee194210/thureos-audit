package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SourceType string

const (
	SourceCSV   SourceType = "csv"
	SourceExcel SourceType = "excel"
	SourceJSON  SourceType = "json"
	SourceTXT   SourceType = "txt"
	SourceAPI   SourceType = "api"
)

type FieldType string

const (
	FieldString  FieldType = "string"
	FieldNumber  FieldType = "number"
	FieldDate    FieldType = "date"
	FieldBoolean FieldType = "boolean"
)

type SchemaField struct {
	Name            string    `bson:"name" json:"name"`
	Type            FieldType `bson:"type" json:"type"`
	Required        bool      `bson:"required" json:"required"`
	Sample          string    `bson:"sample" json:"sample"`
	ImpliedDecimals int       `bson:"implied_decimals,omitempty" json:"impliedDecimals,omitempty"`
	DateFormat      string    `bson:"date_format,omitempty" json:"dateFormat,omitempty"`
}

// DerivedTimestampConfig construye un campo de fecha real a partir de dos
// columnas numéricas separadas — patrón habitual en exports bancarios, donde
// fecha y hora vienen en campos distintos. Nil en el monitor significa que no
// hay timestamp derivado y todo se comporta como siempre.
type DerivedTimestampConfig struct {
	DateField  string `bson:"date_field" json:"dateField"`
	DateFormat string `bson:"date_format" json:"dateFormat"` // "YYYYMMDD"
	TimeField  string `bson:"time_field" json:"timeField"`
	TimeFormat string `bson:"time_format" json:"timeFormat"` // "HHMMSS" | "HHMM"
	TargetName string `bson:"target_name" json:"targetName"`
}

type APIMode string

const (
	APIModePush APIMode = "push"
	APIModePull APIMode = "pull"
)

type APIAuthType string

const (
	APIAuthNone   APIAuthType = "none"
	APIAuthAPIKey APIAuthType = "api_key_header"
	APIAuthBearer APIAuthType = "bearer"
)

// SourceConfig holds ingestion parameters specific to a monitor's SourceType.
// Which fields apply depends on SourceType — see the design spec. Every field
// is optional so existing monitors (created before this struct existed)
// behave exactly as before.
type SourceConfig struct {
	// csv / txt
	Delimiter    string `bson:"delimiter,omitempty" json:"delimiter,omitempty"`
	HasHeaderRow *bool  `bson:"has_header_row,omitempty" json:"hasHeaderRow,omitempty"`

	// excel
	SheetName string `bson:"sheet_name,omitempty" json:"sheetName,omitempty"`

	// json / api (nested array extraction)
	RootPath string `bson:"root_path,omitempty" json:"rootPath,omitempty"`

	// api
	Mode                APIMode     `bson:"mode,omitempty" json:"mode,omitempty"`
	PushToken           string      `bson:"push_token,omitempty" json:"-"`
	PullURL             string      `bson:"pull_url,omitempty" json:"pullUrl,omitempty"`
	PullMethod          string      `bson:"pull_method,omitempty" json:"pullMethod,omitempty"`
	PullAuthType        APIAuthType `bson:"pull_auth_type,omitempty" json:"pullAuthType,omitempty"`
	PullAuthHeaderName  string      `bson:"pull_auth_header_name,omitempty" json:"pullAuthHeaderName,omitempty"`
	PullAuthValue       string      `bson:"pull_auth_value,omitempty" json:"-"`
	PullIntervalMinutes int         `bson:"pull_interval_minutes,omitempty" json:"pullIntervalMinutes,omitempty"`
	NextPullAt          *time.Time  `bson:"next_pull_at,omitempty" json:"nextPullAt,omitempty"`
	LastPullAt          *time.Time  `bson:"last_pull_at,omitempty" json:"lastPullAt,omitempty"`
	LastPullStatus      string      `bson:"last_pull_status,omitempty" json:"lastPullStatus,omitempty"`
	LastPullError       string      `bson:"last_pull_error,omitempty" json:"lastPullError,omitempty"`
}

// SourceConfigInput is the client-facing shape for creating a monitor's
// source configuration. Unlike SourceConfig, PullAuthValue has no `json:"-"`
// here — this struct is only ever used to receive input, never to respond,
// so there is no leak risk. PushToken and the LastPull*/NextPullAt fields
// are server-controlled and intentionally absent: a client cannot set them.
type SourceConfigInput struct {
	Delimiter           string      `json:"delimiter,omitempty"`
	HasHeaderRow        *bool       `json:"hasHeaderRow,omitempty"`
	SheetName           string      `json:"sheetName,omitempty"`
	RootPath            string      `json:"rootPath,omitempty"`
	Mode                APIMode     `json:"mode,omitempty"`
	PullURL             string      `json:"pullUrl,omitempty"`
	PullMethod          string      `json:"pullMethod,omitempty"`
	PullAuthType        APIAuthType `json:"pullAuthType,omitempty"`
	PullAuthHeaderName  string      `json:"pullAuthHeaderName,omitempty"`
	PullAuthValue       string      `json:"pullAuthValue,omitempty"`
	PullIntervalMinutes int         `json:"pullIntervalMinutes,omitempty"`
}

// ToSourceConfig converts client input into the stored shape. Returns nil
// for a nil receiver so callers can do `monitor.SourceConfig = req.SourceConfig.ToSourceConfig()`
// unconditionally, even when the client sent no sourceConfig at all.
func (in *SourceConfigInput) ToSourceConfig() *SourceConfig {
	if in == nil {
		return nil
	}
	return &SourceConfig{
		Delimiter:           in.Delimiter,
		HasHeaderRow:        in.HasHeaderRow,
		SheetName:           in.SheetName,
		RootPath:            in.RootPath,
		Mode:                in.Mode,
		PullURL:             in.PullURL,
		PullMethod:          in.PullMethod,
		PullAuthType:        in.PullAuthType,
		PullAuthHeaderName:  in.PullAuthHeaderName,
		PullAuthValue:       in.PullAuthValue,
		PullIntervalMinutes: in.PullIntervalMinutes,
	}
}

type Monitor struct {
	ID               primitive.ObjectID      `bson:"_id,omitempty" json:"id"`
	Name             string                  `bson:"name" json:"name"`
	Description      string                  `bson:"description" json:"description"`
	SourceType       SourceType              `bson:"source_type" json:"sourceType"`
	SourceConfig     *SourceConfig           `bson:"source_config,omitempty" json:"sourceConfig,omitempty"`
	Schema           []SchemaField           `bson:"schema" json:"schema"`
	DerivedTimestamp *DerivedTimestampConfig `bson:"derived_timestamp,omitempty" json:"derivedTimestamp,omitempty"`
	CollectionID     string                  `bson:"collection_id" json:"collectionId"`
	OwnerID          primitive.ObjectID      `bson:"owner_id" json:"ownerId"`
	RecordCount      int64                   `bson:"record_count" json:"recordCount"`
	LastIngested     *time.Time              `bson:"last_ingested,omitempty" json:"lastIngested,omitempty"`
	CreatedAt        time.Time               `bson:"created_at" json:"createdAt"`
	UpdatedAt        time.Time               `bson:"updated_at" json:"updatedAt"`
}

type CreateMonitorRequest struct {
	Name         string             `json:"name"`
	Description  string             `json:"description"`
	SourceType   SourceType         `json:"sourceType"`
	SourceConfig *SourceConfigInput `json:"sourceConfig,omitempty"`
	// Schema is optional and only meaningful for source type "api": the
	// frontend sends it after the user confirms the result of POST
	// /monitors/detect-schema. Every other flow (CSV/Excel/JSON/TXT,
	// API push) omits it, leaving schema detection on first ingest unchanged.
	Schema []SchemaField `json:"schema,omitempty"`
}

// UpdateMonitorRequest is the client-facing shape for editing a monitor.
// SourceType is deliberately absent — it's fixed at creation, since
// switching it could leave an already-detected Schema (and any ingested
// data) inconsistent with the new source. When SourceConfig is present, it
// REPLACES the stored config wholesale (the edit form always sends the
// complete set of fields for the monitor's source type, same as creation),
// except for the server-controlled fields SourceConfigInput can't carry
// (PushToken, NextPullAt, LastPull*), which the handler preserves from the
// existing monitor.
type UpdateMonitorRequest struct {
	Name         *string            `json:"name,omitempty"`
	Description  *string            `json:"description,omitempty"`
	SourceConfig *SourceConfigInput `json:"sourceConfig,omitempty"`
}
