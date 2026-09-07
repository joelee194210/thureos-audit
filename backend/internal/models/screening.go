package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ScreeningStatus es el resultado de clasificar un screening contra
// Watchman, o la decisión de un analista sobre un match/review.
type ScreeningStatus string

const (
	ScreeningClear         ScreeningStatus = "clear"
	ScreeningMatch         ScreeningStatus = "match"
	ScreeningReview        ScreeningStatus = "review"
	ScreeningDismissed     ScreeningStatus = "dismissed"
	ScreeningFalsePositive ScreeningStatus = "false_positive"
)

// ScreeningWhitelistDuration: cuánto dura un descarte (Dismissed/
// FalsePositive) antes de que screening_reeval_job lo vuelva a correr
// contra Watchman. Un descarte no es "nunca más" — vence.
const ScreeningWhitelistDuration = 180 * 24 * time.Hour

// NormalizedMatch es un resultado de Watchman normalizado — mismo shape
// en la respuesta en vivo del cliente y en lo que persiste
// ScreeningResult.Matches.
type NormalizedMatch struct {
	Name       string   `bson:"name" json:"name"`
	EntityType string   `bson:"entity_type" json:"entityType"`
	SourceList string   `bson:"source_list" json:"sourceList"`
	Score      float64  `bson:"score" json:"score"`
	Programs   []string `bson:"programs,omitempty" json:"programs,omitempty"`
}

// ScreeningResult es un screening contra Watchman: de una búsqueda manual
// (RedFlagID nulo) o disparado por una regla (linkeado al caso vía
// RedFlagID + qué campo de la regla lo generó).
type ScreeningResult struct {
	ID                 primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	Query              string              `bson:"query" json:"query"`
	RedFlagID          *primitive.ObjectID `bson:"red_flag_id,omitempty" json:"redFlagId,omitempty"`
	Field              string              `bson:"field,omitempty" json:"field,omitempty"`
	Status             ScreeningStatus     `bson:"status" json:"status"`
	StrongestMatch     string              `bson:"strongest_match,omitempty" json:"strongestMatch,omitempty"`
	Matches            []NormalizedMatch   `bson:"matches,omitempty" json:"matches,omitempty"`
	ReviewedBy         *primitive.ObjectID `bson:"reviewed_by,omitempty" json:"reviewedBy,omitempty"`
	ReviewedAt         *time.Time          `bson:"reviewed_at,omitempty" json:"reviewedAt,omitempty"`
	ReviewNotes        string              `bson:"review_notes,omitempty" json:"reviewNotes,omitempty"`
	WhitelistExpiresAt *time.Time          `bson:"whitelist_expires_at,omitempty" json:"whitelistExpiresAt,omitempty"`
	CreatedAt          time.Time           `bson:"created_at" json:"createdAt"`
}
