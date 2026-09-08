package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thureos/compliance/internal/models"
)

func TestDetectSchemaFromJSON_DetectsDatesMixedWithStringsAndNumbers(t *testing.T) {
	records := []map[string]interface{}{
		{
			"amount":       1200.50,
			"createdAt":    "2024-01-15",
			"updatedAt":    "2024-01-15T10:30:00Z",
			"description":  "wire transfer",
			"isReconciled": true,
		},
	}

	schema := detectSchemaFromJSON(records)

	types := make(map[string]models.FieldType, len(schema))
	for _, f := range schema {
		types[f.Name] = f.Type
	}

	cases := map[string]models.FieldType{
		"amount":       models.FieldNumber,
		"createdAt":    models.FieldDate,
		"updatedAt":    models.FieldDate,
		"description":  models.FieldString,
		"isReconciled": models.FieldBoolean,
	}
	for field, want := range cases {
		got, ok := types[field]
		if !ok {
			t.Fatalf("field %q missing from detected schema: %+v", field, schema)
		}
		if got != want {
			t.Errorf("field %q: got type %q, want %q", field, got, want)
		}
	}
	if len(schema) != len(cases) {
		t.Errorf("schema has %d fields, want %d: %+v", len(schema), len(cases), schema)
	}
}

func TestDetectSchemaFromJSON_ReturnsStableAlphabeticalOrder(t *testing.T) {
	records := []map[string]interface{}{
		{"id": float64(1), "amount": float64(10), "createdAt": "2024-01-01"},
		{"id": float64(2), "amount": float64(20), "createdAt": "2024-01-02"},
	}

	// This is now a user-facing preview the user confirms, so the same
	// input must yield the same field order on every call — not whatever
	// order Go's randomized map iteration happens to produce this time.
	want := []string{"amount", "createdAt", "id"}

	for i := 0; i < 20; i++ {
		schema := detectSchemaFromJSON(records)
		if len(schema) != len(want) {
			t.Fatalf("run %d: schema has %d fields, want %d: %+v", i, len(schema), len(want), schema)
		}
		for j, f := range schema {
			if f.Name != want[j] {
				t.Fatalf("run %d: schema order = %v, want %v", i, fieldNames(schema), want)
			}
		}
	}
}

func fieldNames(schema []models.SchemaField) []string {
	names := make([]string, len(schema))
	for i, f := range schema {
		names[i] = f.Name
	}
	return names
}

// newIngestionServiceForTest builds an IngestionService with an unrestricted
// http.Client — DetectAPISchema's default client refuses loopback addresses
// (SSRF guard), which would also refuse httptest's own server, so tests that
// need a real HTTP round trip must swap it out.
func newIngestionServiceForTest() *IngestionService {
	return &IngestionService{httpClient: http.DefaultClient}
}

func TestDetectAPISchema_PushModeReturnsError(t *testing.T) {
	s := newIngestionServiceForTest()

	_, _, err := s.DetectAPISchema(context.Background(), models.SourceConfig{
		Mode:    models.APIModePush,
		PullURL: "https://example.com/data",
	})

	if !errors.Is(err, ErrPushModeSchemaDetection) {
		t.Fatalf("expected ErrPushModeSchemaDetection, got %v", err)
	}
}

func TestDetectAPISchema_HappyPathWithRootPathAndSampleLimit(t *testing.T) {
	records := make([]map[string]interface{}, 0, 60)
	for i := 0; i < 60; i++ {
		records = append(records, map[string]interface{}{
			"id":        float64(i),
			"amount":    float64(i) * 10.5,
			"createdAt": "2024-03-01T00:00:00Z",
			"active":    i%2 == 0,
		})
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"records": records,
			},
		})
	}))
	defer server.Close()

	s := newIngestionServiceForTest()

	schema, sampleCount, err := s.DetectAPISchema(context.Background(), models.SourceConfig{
		Mode:               models.APIModePull,
		PullURL:            server.URL,
		PullMethod:         http.MethodGet,
		PullAuthType:       models.APIAuthAPIKey,
		PullAuthHeaderName: "X-Api-Key",
		PullAuthValue:      "secret",
		RootPath:           "data.records",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sampleCount != maxSchemaDetectSampleRows {
		t.Errorf("sampleCount = %d, want %d (capped)", sampleCount, maxSchemaDetectSampleRows)
	}

	types := make(map[string]models.FieldType, len(schema))
	for _, f := range schema {
		types[f.Name] = f.Type
	}
	cases := map[string]models.FieldType{
		"id":        models.FieldNumber,
		"amount":    models.FieldNumber,
		"createdAt": models.FieldDate,
		"active":    models.FieldBoolean,
	}
	for field, want := range cases {
		got, ok := types[field]
		if !ok {
			t.Fatalf("field %q missing from detected schema: %+v", field, schema)
		}
		if got != want {
			t.Errorf("field %q: got type %q, want %q", field, got, want)
		}
	}
}

func TestDetectAPISchema_EmptyRecordsReturnsClearError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]interface{}{})
	}))
	defer server.Close()

	s := newIngestionServiceForTest()

	_, _, err := s.DetectAPISchema(context.Background(), models.SourceConfig{
		Mode:    models.APIModePull,
		PullURL: server.URL,
	})
	if err == nil {
		t.Fatal("expected an error for a response with no records, got nil")
	}
}

func TestDetectAPISchema_NonOKStatusReturnsClearError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("invalid credentials"))
	}))
	defer server.Close()

	s := newIngestionServiceForTest()

	_, _, err := s.DetectAPISchema(context.Background(), models.SourceConfig{
		Mode:    models.APIModePull,
		PullURL: server.URL,
	})
	if err == nil {
		t.Fatal("expected an error for a non-2xx response, got nil")
	}
}

func TestDetectAPISchema_OversizedResponseReturnsSizeErrorNotJSONError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[`))
		// A body larger than maxPullResponseBytes must be reported as
		// oversized, not misdiagnosed as invalid JSON just because the read
		// was cut off mid-array.
		filler := make([]byte, maxPullResponseBytes+1024)
		for i := range filler {
			filler[i] = '0'
		}
		_, _ = w.Write(filler)
	}))
	defer server.Close()

	s := newIngestionServiceForTest()

	_, _, err := s.DetectAPISchema(context.Background(), models.SourceConfig{
		Mode:    models.APIModePull,
		PullURL: server.URL,
	})
	if err == nil {
		t.Fatal("expected an error for an oversized response, got nil")
	}
	if !strings.Contains(err.Error(), "supera el límite") {
		t.Fatalf("expected a size-limit error, got: %v", err)
	}
}

func schemaField(name string, t models.FieldType) models.SchemaField {
	return models.SchemaField{Name: name, Type: t}
}

func TestCompareSchema_ExactMatchSameOrder(t *testing.T) {
	expected := []models.SchemaField{schemaField("monto", models.FieldNumber), schemaField("fecha", models.FieldDate)}
	actual := []models.SchemaField{schemaField("monto", models.FieldNumber), schemaField("fecha", models.FieldDate)}

	diff := compareSchema(expected, actual)
	if !diff.Match {
		t.Errorf("esperaba match, obtuve %+v", diff)
	}
}

func TestCompareSchema_OrderIrrelevant(t *testing.T) {
	expected := []models.SchemaField{schemaField("monto", models.FieldNumber), schemaField("fecha", models.FieldDate)}
	actual := []models.SchemaField{schemaField("fecha", models.FieldDate), schemaField("monto", models.FieldNumber)}

	diff := compareSchema(expected, actual)
	if !diff.Match {
		t.Errorf("el orden de las columnas no debería importar, obtuve %+v", diff)
	}
}

func TestCompareSchema_MissingField(t *testing.T) {
	expected := []models.SchemaField{schemaField("monto", models.FieldNumber), schemaField("clavepais", models.FieldString)}
	actual := []models.SchemaField{schemaField("monto", models.FieldNumber)}

	diff := compareSchema(expected, actual)
	if diff.Match {
		t.Fatal("esperaba Match=false por campo faltante")
	}
	if len(diff.MissingFields) != 1 || diff.MissingFields[0] != "clavepais" {
		t.Errorf("MissingFields = %v, esperaba [clavepais]", diff.MissingFields)
	}
}

func TestCompareSchema_ExtraField(t *testing.T) {
	expected := []models.SchemaField{schemaField("monto", models.FieldNumber)}
	actual := []models.SchemaField{schemaField("monto", models.FieldNumber), schemaField("ticket", models.FieldString)}

	diff := compareSchema(expected, actual)
	if diff.Match {
		t.Fatal("esperaba Match=false por campo de más")
	}
	if len(diff.ExtraFields) != 1 || diff.ExtraFields[0] != "ticket" {
		t.Errorf("ExtraFields = %v, esperaba [ticket]", diff.ExtraFields)
	}
}

func TestCompareSchema_TypeMismatch(t *testing.T) {
	expected := []models.SchemaField{schemaField("monto", models.FieldNumber)}
	actual := []models.SchemaField{schemaField("monto", models.FieldString)}

	diff := compareSchema(expected, actual)
	if diff.Match {
		t.Fatal("esperaba Match=false por tipo distinto")
	}
	if len(diff.TypeMismatches) != 1 || diff.TypeMismatches[0].Field != "monto" {
		t.Errorf("TypeMismatches = %+v, esperaba un mismatch en 'monto'", diff.TypeMismatches)
	}
	if diff.TypeMismatches[0].ExpectedType != models.FieldNumber || diff.TypeMismatches[0].ActualType != models.FieldString {
		t.Errorf("tipos del mismatch incorrectos: %+v", diff.TypeMismatches[0])
	}
}

func TestCompareSchema_EmptyExpectedAlwaysMatches(t *testing.T) {
	actual := []models.SchemaField{schemaField("cualquier_cosa", models.FieldString)}

	diff := compareSchema(nil, actual)
	if !diff.Match {
		t.Errorf("un monitor sin base establecida (primer upload) siempre debería matchear, obtuve %+v", diff)
	}
}

func TestValidateFieldValue_NumberValid(t *testing.T) {
	if !validateFieldValue("1234.56", models.FieldNumber) {
		t.Error("esperaba que '1234.56' sea válido como number")
	}
}

func TestValidateFieldValue_NumberInvalid(t *testing.T) {
	if validateFieldValue("n/a", models.FieldNumber) {
		t.Error("esperaba que 'n/a' NO sea válido como number")
	}
}

func TestValidateFieldValue_DateValid(t *testing.T) {
	if !validateFieldValue("2026-09-08", models.FieldDate) {
		t.Error("esperaba que '2026-09-08' sea válido como date")
	}
}

func TestValidateFieldValue_DateInvalid(t *testing.T) {
	if validateFieldValue("no es una fecha", models.FieldDate) {
		t.Error("esperaba que 'no es una fecha' NO sea válido como date")
	}
}

func TestValidateFieldValue_BooleanValid(t *testing.T) {
	if !validateFieldValue("true", models.FieldBoolean) {
		t.Error("esperaba que 'true' sea válido como boolean")
	}
}

func TestValidateFieldValue_BooleanInvalid(t *testing.T) {
	if validateFieldValue("tal vez", models.FieldBoolean) {
		t.Error("esperaba que 'tal vez' NO sea válido como boolean")
	}
}

func TestValidateFieldValue_StringAlwaysValid(t *testing.T) {
	if !validateFieldValue("cualquier texto 123", models.FieldString) {
		t.Error("un campo de tipo string siempre debería aceptar cualquier valor")
	}
}

func TestValidateFieldValue_EmptyAlwaysValid(t *testing.T) {
	if !validateFieldValue("", models.FieldNumber) {
		t.Error("una celda vacía no debería rechazarse por tipo — es un campo opcional sin valor, no un valor inválido")
	}
}

func TestFirstInvalidField_CleanRowReturnsEmpty(t *testing.T) {
	schema := []models.SchemaField{schemaField("monto", models.FieldNumber), schemaField("cliente", models.FieldString)}
	field, reason := firstInvalidField([]string{"100.50", "Juan"}, []string{"monto", "cliente"}, schema)
	if field != "" || reason != "" {
		t.Errorf("fila válida no debería rechazar nada, obtuve field=%q reason=%q", field, reason)
	}
}

func TestFirstInvalidField_BadTypeReturnsField(t *testing.T) {
	schema := []models.SchemaField{schemaField("monto", models.FieldNumber), schemaField("cliente", models.FieldString)}
	field, reason := firstInvalidField([]string{"no-es-numero", "Juan"}, []string{"monto", "cliente"}, schema)
	if field != "monto" {
		t.Errorf("field = %q, esperaba 'monto'", field)
	}
	if reason == "" {
		t.Error("esperaba una razón no vacía")
	}
}

func TestFirstInvalidField_MissingTrailingColumnsNotRejected(t *testing.T) {
	schema := []models.SchemaField{schemaField("monto", models.FieldNumber), schemaField("cliente", models.FieldString)}
	// Fila con menos columnas que headers — comportamiento ya tolerado hoy
	// por el resto del código (ver doc-building loop), no es un motivo de
	// rechazo por sí solo.
	field, reason := firstInvalidField([]string{"100.50"}, []string{"monto", "cliente"}, schema)
	if field != "" || reason != "" {
		t.Errorf("columnas faltantes al final no deberían rechazar la fila, obtuve field=%q reason=%q", field, reason)
	}
}

func TestParseValue_ImpliedDecimalsDividesCorrectly(t *testing.T) {
	schema := []models.SchemaField{{Name: "monto", Type: models.FieldNumber, ImpliedDecimals: 2}}
	got := parseValue("500000", schema, "monto")
	f, ok := got.(float64)
	if !ok {
		t.Fatalf("esperaba float64, obtuve %T", got)
	}
	if f != 5000.0 {
		t.Errorf("got %v, want 5000.0", f)
	}
}

func TestParseValue_ImpliedDecimalsZeroNoChange(t *testing.T) {
	schema := []models.SchemaField{{Name: "monto", Type: models.FieldNumber, ImpliedDecimals: 0}}
	got := parseValue("500000", schema, "monto")
	f, ok := got.(float64)
	if !ok {
		t.Fatalf("esperaba float64, obtuve %T", got)
	}
	if f != 500000.0 {
		t.Errorf("got %v, want 500000.0 (sin cambios cuando ImpliedDecimals=0)", f)
	}
}

func TestParseValue_ImpliedDecimalsSmallValue(t *testing.T) {
	schema := []models.SchemaField{{Name: "monto", Type: models.FieldNumber, ImpliedDecimals: 2}}
	got := parseValue("5", schema, "monto")
	f, ok := got.(float64)
	if !ok {
		t.Fatalf("esperaba float64, obtuve %T", got)
	}
	if f != 0.05 {
		t.Errorf("got %v, want 0.05", f)
	}
}

func TestParseValue_InvalidNumberFallsBackToRawString(t *testing.T) {
	schema := []models.SchemaField{{Name: "monto", Type: models.FieldNumber, ImpliedDecimals: 2}}
	got := parseValue("no-es-numero", schema, "monto")
	if got != "no-es-numero" {
		t.Errorf("got %v, want el string crudo sin parsear", got)
	}
}
