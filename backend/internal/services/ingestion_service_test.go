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
