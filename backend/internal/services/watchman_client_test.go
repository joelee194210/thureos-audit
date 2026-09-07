package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWatchmanClient_Search_ParseaYNormalizaLaRespuesta(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path + "?" + r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"entities": []map[string]interface{}{
				{
					"name":       "Juan Perez",
					"entityType": "person",
					"sourceList": "us_ofac",
					"sourceID":   "12345",
					"match":      0.91,
					"sanctionsInfo": map[string]interface{}{
						"programs": []string{"SDGT"},
					},
				},
			},
		})
	}))
	defer srv.Close()

	client := &WatchmanClient{httpClient: srv.Client(), watchmanURL: srv.URL}
	matches, err := client.Search(context.Background(), "Juan Perez", WatchmanSearchOptions{Limit: 10, MinMatch: 0.5})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !strings.HasPrefix(gotPath, "/v2/search?") {
		t.Errorf("path = %q, want prefix /v2/search?", gotPath)
	}
	if !strings.Contains(gotPath, "name=Juan+Perez") {
		t.Errorf("path = %q, esperaba el nombre en la query", gotPath)
	}
	if len(matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(matches))
	}
	m := matches[0]
	if m.Name != "Juan Perez" || m.SourceList != "us_ofac" || m.Score != 0.91 {
		t.Errorf("match = %+v", m)
	}
	if len(m.Programs) != 1 || m.Programs[0] != "SDGT" {
		t.Errorf("programs = %v", m.Programs)
	}
}

func TestWatchmanClient_Search_ErrorHTTPSube(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	client := &WatchmanClient{httpClient: srv.Client(), watchmanURL: srv.URL}
	_, err := client.Search(context.Background(), "test", WatchmanSearchOptions{})
	if err == nil {
		t.Error("un 500 de Watchman debe devolver error")
	}
}

func TestWatchmanClient_Search_SinURLConfiguradaEsError(t *testing.T) {
	client := &WatchmanClient{httpClient: http.DefaultClient, watchmanURL: ""}
	_, err := client.Search(context.Background(), "test", WatchmanSearchOptions{})
	if err == nil {
		t.Error("sin watchmanURL configurada debe devolver error, no silencio")
	}
}
