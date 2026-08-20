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
	Name     string    `bson:"name" json:"name"`
	Type     FieldType `bson:"type" json:"type"`
	Required bool      `bson:"required" json:"required"`
	Sample   string    `bson:"sample" json:"sample"`
}

type Monitor struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name         string             `bson:"name" json:"name"`
	Description  string             `bson:"description" json:"description"`
	SourceType   SourceType         `bson:"source_type" json:"sourceType"`
	Schema       []SchemaField      `bson:"schema" json:"schema"`
	CollectionID string             `bson:"collection_id" json:"collectionId"`
	APIEndpoint  string             `bson:"api_endpoint,omitempty" json:"apiEndpoint,omitempty"`
	Schedule     string             `bson:"schedule,omitempty" json:"schedule,omitempty"`
	OwnerID      primitive.ObjectID `bson:"owner_id" json:"ownerId"`
	RecordCount  int64              `bson:"record_count" json:"recordCount"`
	LastIngested *time.Time         `bson:"last_ingested,omitempty" json:"lastIngested,omitempty"`
	CreatedAt    time.Time          `bson:"created_at" json:"createdAt"`
	UpdatedAt    time.Time          `bson:"updated_at" json:"updatedAt"`
}

type CreateMonitorRequest struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	SourceType  SourceType `json:"sourceType"`
	APIEndpoint string     `json:"apiEndpoint,omitempty"`
	Schedule    string     `json:"schedule,omitempty"`
}
