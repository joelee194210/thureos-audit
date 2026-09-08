package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type UploadStatus string

const (
	UploadStatusAccepted          UploadStatus = "accepted"
	UploadStatusPartial           UploadStatus = "partial"
	UploadStatusRejectedStructure UploadStatus = "rejected_structure"
	UploadStatusApproved          UploadStatus = "approved"
)

// FieldTypeMismatch describes one field whose type in the uploaded file
// doesn't match the monitor's established schema.
type FieldTypeMismatch struct {
	Field        string    `bson:"field" json:"field"`
	ExpectedType FieldType `bson:"expected_type" json:"expectedType"`
	ActualType   FieldType `bson:"actual_type" json:"actualType"`
}

// SchemaDiff is the result of comparing a file's detected structure
// against a monitor's established schema. Match is true only when
// MissingFields, ExtraFields and TypeMismatches are all empty.
type SchemaDiff struct {
	Match          bool                `bson:"match" json:"match"`
	MissingFields  []string            `bson:"missing_fields,omitempty" json:"missingFields,omitempty"`
	ExtraFields    []string            `bson:"extra_fields,omitempty" json:"extraFields,omitempty"`
	TypeMismatches []FieldTypeMismatch `bson:"type_mismatches,omitempty" json:"typeMismatches,omitempty"`
}

// RowRejection records one row that was skipped during ingestion because
// a field's value didn't parse as its schema type. RowIndex is 1-based,
// counting only data rows (the header row, if any, is not row 1).
type RowRejection struct {
	RowIndex int    `bson:"row_index" json:"rowIndex"`
	Field    string `bson:"field" json:"field"`
	Reason   string `bson:"reason" json:"reason"`
}

// UploadLogEntry is one row of the upload audit log — one per real
// upload attempt (not per dry-run check, those are never persisted).
type UploadLogEntry struct {
	ID              primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	MonitorID       primitive.ObjectID  `bson:"monitor_id" json:"monitorId"`
	MonitorName     string              `bson:"monitor_name" json:"monitorName"`
	FileName        string              `bson:"file_name" json:"fileName"`
	SourceType      SourceType          `bson:"source_type" json:"sourceType"`
	UploadedBy      primitive.ObjectID  `bson:"uploaded_by" json:"uploadedBy"`
	UploadedByEmail string              `bson:"uploaded_by_email" json:"uploadedByEmail"`
	UploadedAt      time.Time           `bson:"uploaded_at" json:"uploadedAt"`
	Status          UploadStatus        `bson:"status" json:"status"`
	TotalRows       int                 `bson:"total_rows" json:"totalRows"`
	RowsAccepted    int                 `bson:"rows_accepted" json:"rowsAccepted"`
	RowsRejected    int                 `bson:"rows_rejected" json:"rowsRejected"`
	SchemaDiff      *SchemaDiff         `bson:"schema_diff,omitempty" json:"schemaDiff,omitempty"`
	RowRejections   []RowRejection      `bson:"row_rejections,omitempty" json:"rowRejections,omitempty"`
	SampleRows      []map[string]any    `bson:"sample_rows,omitempty" json:"sampleRows,omitempty"`
	GridFSFileID    *primitive.ObjectID `bson:"gridfs_file_id,omitempty" json:"-"`
}
