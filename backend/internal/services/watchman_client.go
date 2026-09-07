package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
)

// WatchmanSearchOptions ajusta la búsqueda contra Watchman.
type WatchmanSearchOptions struct {
	Limit       int
	MinMatch    float64
	DateOfBirth string // ISO o año — desambigua homónimos si el dato está disponible
}

// watchmanEntity es la forma cruda de la API v2 de Watchman (moov/watchman).
type watchmanEntity struct {
	Name          string  `json:"name"`
	EntityType    string  `json:"entityType"`
	SourceList    string  `json:"sourceList"`
	Match         float64 `json:"match"`
	SanctionsInfo *struct {
		Programs []string `json:"programs"`
	} `json:"sanctionsInfo"`
}

type watchmanSearchResponse struct {
	Entities []watchmanEntity `json:"entities"`
}

// WatchmanClient habla con un servicio Watchman ya desplegado (VPS
// compartido, misma red Docker que este proyecto en producción) — no
// gestiona listas ni las actualiza, eso lo hace el cron del VPS.
type WatchmanClient struct {
	httpClient  *http.Client
	configRepo  *repository.SystemConfigRepository
	watchmanURL string // inyectable en tests; en producción sale de configRepo
}

func NewWatchmanClient(configRepo *repository.SystemConfigRepository) *WatchmanClient {
	return &WatchmanClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		configRepo: configRepo,
	}
}

// Search busca un nombre contra Watchman y normaliza la respuesta. La URL
// configurada un admin de confianza (Configuración → APIs), no un cliente
// HTTP con guard anti-SSRF como el de PullURL — no es la misma superficie.
func (c *WatchmanClient) Search(ctx context.Context, name string, opts WatchmanSearchOptions) ([]models.NormalizedMatch, error) {
	baseURL := c.watchmanURL
	if baseURL == "" && c.configRepo != nil {
		cfg, err := c.configRepo.Get(ctx)
		if err != nil {
			return nil, fmt.Errorf("leyendo config de Watchman: %w", err)
		}
		baseURL = cfg.Watchman.URL
	}
	if baseURL == "" {
		return nil, fmt.Errorf("watchman: URL no configurada (Configuración → APIs)")
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}
	minMatch := opts.MinMatch
	if minMatch <= 0 {
		minMatch = 0.5
	}

	params := url.Values{}
	params.Set("name", name)
	params.Set("limit", strconv.Itoa(limit))
	params.Set("minMatch", strconv.FormatFloat(minMatch, 'f', -1, 64))
	if opts.DateOfBirth != "" {
		params.Set("dateOfBirth", opts.DateOfBirth)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v2/search?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("armando la petición a Watchman: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("no se pudo contactar Watchman: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("watchman respondió HTTP %d: %s", resp.StatusCode, string(body))
	}

	var parsed watchmanSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("la respuesta de Watchman no es JSON válido: %w", err)
	}

	matches := make([]models.NormalizedMatch, 0, len(parsed.Entities))
	for _, e := range parsed.Entities {
		m := models.NormalizedMatch{
			Name:       e.Name,
			EntityType: e.EntityType,
			SourceList: e.SourceList,
			Score:      e.Match,
		}
		if e.SanctionsInfo != nil {
			m.Programs = e.SanctionsInfo.Programs
		}
		matches = append(matches, m)
	}
	return matches, nil
}
