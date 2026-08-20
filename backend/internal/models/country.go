package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CountryRiskLevel string

const (
	CountryRiskHigh   CountryRiskLevel = "high"
	CountryRiskMedium CountryRiskLevel = "medium"
	CountryRiskLow    CountryRiskLevel = "low"
)

type Country struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Code        string             `bson:"code" json:"code"`               // ISO 3166-1 alpha-2 (PA, US, RU)
	Code3       string             `bson:"code3" json:"code3"`             // ISO 3166-1 alpha-3 (PAN, USA, RUS)
	Name        string             `bson:"name" json:"name"`               // Nombre en espanol
	NameEN      string             `bson:"name_en" json:"nameEn"`          // Nombre en ingles
	Region      string             `bson:"region" json:"region"`           // Region geografica
	RiskLevel   CountryRiskLevel   `bson:"risk_level" json:"riskLevel"`    // high, medium, low
	RiskSources []string           `bson:"risk_sources" json:"riskSources"` // FATF, Basel, OFAC, EU, ONU
	Active      bool               `bson:"active" json:"active"`
	Notes       string             `bson:"notes" json:"notes"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updatedAt"`
	UpdatedBy   string             `bson:"updated_by" json:"updatedBy"`
	CreatedAt   time.Time          `bson:"created_at" json:"createdAt"`
}
