package services

import (
	"testing"
	"time"

	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson"
)

func baseTimestampConfig() models.DerivedTimestampConfig {
	return models.DerivedTimestampConfig{
		DateField:  "fechapoliza",
		DateFormat: "YYYYMMDD",
		TimeField:  "horapoliza",
		TimeFormat: "HHMMSS",
		TargetName: "timestamp",
	}
}

// El caso que motiva todo: horapoliza se guarda como número, así que
// "021517" llega como 21517. Sin relleno de ceros se leería 21:51:07 —
// 19 horas de diferencia sobre el valor real, en silencio.
func TestBuildDerivedTimestamp_RellenaCerosALaIzquierda(t *testing.T) {
	doc := bson.M{"fechapoliza": int32(20260907), "horapoliza": int32(21517)}
	got, ok := BuildDerivedTimestamp(baseTimestampConfig(), doc)
	if !ok {
		t.Fatal("esperaba poder construir el timestamp")
	}
	want := time.Date(2026, 9, 7, 2, 15, 17, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestBuildDerivedTimestamp_MedianocheEsCero(t *testing.T) {
	doc := bson.M{"fechapoliza": int32(20260907), "horapoliza": int32(0)}
	got, ok := BuildDerivedTimestamp(baseTimestampConfig(), doc)
	if !ok {
		t.Fatal("esperaba poder construir el timestamp")
	}
	want := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v (medianoche)", got, want)
	}
}

func TestBuildDerivedTimestamp_FormatoHHMM(t *testing.T) {
	cfg := baseTimestampConfig()
	cfg.TimeFormat = "HHMM"
	doc := bson.M{"fechapoliza": int32(20260907), "horapoliza": int32(215)}
	got, ok := BuildDerivedTimestamp(cfg, doc)
	if !ok {
		t.Fatal("esperaba poder construir el timestamp")
	}
	want := time.Date(2026, 9, 7, 2, 15, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestBuildDerivedTimestamp_ValoresFloatYString(t *testing.T) {
	// La ingesta puede dejar números como float64 (JSON) o el valor crudo
	// como string si el campo no se tipó como number.
	casos := []bson.M{
		{"fechapoliza": float64(20260907), "horapoliza": float64(21517)},
		{"fechapoliza": "20260907", "horapoliza": "021517"},
	}
	want := time.Date(2026, 9, 7, 2, 15, 17, 0, time.UTC)
	for i, doc := range casos {
		got, ok := BuildDerivedTimestamp(baseTimestampConfig(), doc)
		if !ok {
			t.Fatalf("caso %d: esperaba poder construir el timestamp", i)
		}
		if !got.Equal(want) {
			t.Errorf("caso %d: got %v, want %v", i, got, want)
		}
	}
}

func TestBuildDerivedTimestamp_CampoFaltanteNoConstruye(t *testing.T) {
	doc := bson.M{"fechapoliza": int32(20260907)}
	if _, ok := BuildDerivedTimestamp(baseTimestampConfig(), doc); ok {
		t.Error("sin el campo de hora no debería construir un timestamp")
	}
}

func TestBuildDerivedTimestamp_ValorNoParseableNoConstruye(t *testing.T) {
	doc := bson.M{"fechapoliza": "no-es-fecha", "horapoliza": int32(21517)}
	if _, ok := BuildDerivedTimestamp(baseTimestampConfig(), doc); ok {
		t.Error("un valor no parseable no debería construir un timestamp")
	}
}

func TestBuildDerivedTimestamp_FechaInvalidaNoConstruye(t *testing.T) {
	doc := bson.M{"fechapoliza": int32(20261345), "horapoliza": int32(21517)}
	if _, ok := BuildDerivedTimestamp(baseTimestampConfig(), doc); ok {
		t.Error("mes 13 / día 45 no debería construir un timestamp")
	}
}
