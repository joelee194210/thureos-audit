# Screening de Sanciones (Watchman) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Que una red flag con contraparte identificable dispare automáticamente un screening contra las listas de sanciones vía Watchman (OFAC/ONU/UE/UK, ya desplegado y operado fuera de esta app), y que un analista pueda además buscar un nombre a mano.

**Architecture:** Cliente HTTP simple contra un servicio Watchman ya corriendo (`GET /v2/search`), una capa de clasificación pura (CLEAR/MATCH/REVIEW, fail-closed), un modelo `ScreeningResult` linkeado opcionalmente a un red flag, disparo automático desde el motor de reglas (mismo punto que la notificación y el reporte PDF), y un cron de reevaluación que reabre descartes vencidos — mismo patrón que `SLAEscalationJob`.

**Tech Stack:** Go/Fiber/MongoDB (backend, ya en el repo), Next.js/React/TanStack (frontend, ya en el repo). Sin dependencias nuevas.

**Spec:** `docs/superpowers/specs/2026-09-07-screening-sanciones-design.md`

## Global Constraints

- Fail-closed: un error del proveedor Watchman clasifica SIEMPRE como REVIEW, nunca CLEAR.
- La URL de Watchman vive en `system_config` (Configuración → APIs), no en variable de entorno.
- El screening automático corre en su propia goroutine — nunca bloquea ni puede revertir la creación del red flag.
- El disparo automático va en `EvaluateRules` y `EvaluateRuleForDateRange` (los dos caminos que persisten red flags), **nunca** en `matchRuleForDateRange`/`BacktestRule` (el camino de backtest, que no debe tener efectos secundarios).
- Búsqueda manual: no persiste nada.
- Descarte (Dismissed/FalsePositive) exige nota y vence a los 180 días (`models.ScreeningWhitelistDuration`).

---

## Task 1: Modelo de datos

**Files:**
- Create: `backend/internal/models/screening.go`
- Modify: `backend/internal/models/system_config.go`
- Modify: `backend/internal/models/rule.go`
- Modify: `backend/internal/handlers/rule_handler.go`

**Interfaces:**
- Produces: `models.ScreeningStatus` (const: `ScreeningClear`, `ScreeningMatch`, `ScreeningReview`, `ScreeningDismissed`, `ScreeningFalsePositive`), `models.NormalizedMatch{Name, EntityType, SourceList, Score, Programs}`, `models.ScreeningResult{ID, Query, RedFlagID *primitive.ObjectID, Field, Status, StrongestMatch, Matches, ReviewedBy, ReviewedAt, ReviewNotes, WhitelistExpiresAt, CreatedAt}`, `models.ScreeningWhitelistDuration` (180 días), `models.WatchmanConfig{URL}`, `models.Rule.ScreeningFields []string`.

- [ ] **Step 1: Crear `backend/internal/models/screening.go`**

```go
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
```

- [ ] **Step 2: Agregar `WatchmanConfig` a `backend/internal/models/system_config.go`**

Insertar antes de `type SystemConfig struct` (después del bloque de `WebhookConfig`/`NotificationConfig`):

```go
// WatchmanConfig es la URL del servicio de screening de sanciones (Moov
// Watchman) — ya desplegado y operado fuera de esta app (VPS + cron de
// actualización de listas); acá solo se configura contra qué URL hablarle.
type WatchmanConfig struct {
	URL string `bson:"url" json:"url"`
}
```

Y agregar el campo a `SystemConfig`:

```go
type SystemConfig struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	AI            AIConfig           `bson:"ai" json:"ai"`
	Notifications NotificationConfig `bson:"notifications" json:"notifications"`
	Watchman      WatchmanConfig     `bson:"watchman" json:"watchman"`
	UpdatedAt     time.Time          `bson:"updated_at" json:"updatedAt"`
	UpdatedBy     string             `bson:"updated_by" json:"updatedBy"`
}
```

- [ ] **Step 3: Agregar `ScreeningFields` a `Rule` y `CreateRuleRequest` en `backend/internal/models/rule.go`**

En `type Rule struct`, agregar junto a `TemplateID`:

```go
	// ScreeningFields son los nombres de columnas del schema del monitor
	// cuyo valor se screenea contra Watchman cuando la regla dispara —
	// vacío significa que esta regla no dispara screening.
	ScreeningFields        []string             `bson:"screening_fields,omitempty" json:"screeningFields,omitempty"`
```

En `type CreateRuleRequest struct`, agregar:

```go
	ScreeningFields        []string             `json:"screeningFields,omitempty"`
```

- [ ] **Step 4: Setear `ScreeningFields` en `Create` y `Update` de `backend/internal/handlers/rule_handler.go`**

En `Create`, dentro del literal `rule := &models.Rule{...}`, agregar `ScreeningFields: req.ScreeningFields,`.

En `updateRuleRequest`, agregar:

```go
	ScreeningFields     []string                    `json:"screeningFields" bson:"screening_fields,omitempty"`
```

En `Update`, junto al resto de los `if req.X != nil`:

```go
	if req.ScreeningFields != nil {
		update["screening_fields"] = req.ScreeningFields
	}
```

- [ ] **Step 5: Verificar que compila**

Run: `cd backend && go build ./...`
Expected: sin errores.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/models/screening.go backend/internal/models/system_config.go backend/internal/models/rule.go backend/internal/handlers/rule_handler.go
git commit -m "feat(screening): modelo de datos — ScreeningResult, WatchmanConfig, Rule.ScreeningFields"
```

---

## Task 2: Clasificación pura (CLEAR/MATCH/REVIEW)

**Files:**
- Create: `backend/internal/services/screening_classify.go`
- Test: `backend/internal/services/screening_classify_test.go`

**Interfaces:**
- Consumes: `models.NormalizedMatch`, `models.ScreeningStatus` (Task 1)
- Produces: `services.ScreeningOutcome{Status, StrongestMatch}`, `services.ClassifyScreeningOutcome(matches []models.NormalizedMatch, searchErr error) ScreeningOutcome`

- [ ] **Step 1: Escribir el test en rojo**

```go
package services

import (
	"errors"
	"testing"

	"github.com/thureos/compliance/internal/models"
)

func TestClassifyScreeningOutcome_ErrorDelProveedorEsReviewNuncaClear(t *testing.T) {
	out := ClassifyScreeningOutcome(nil, errors.New("watchman caído"))
	if out.Status != models.ScreeningReview {
		t.Errorf("Status = %q, want %q (fail-closed)", out.Status, models.ScreeningReview)
	}
}

func TestClassifyScreeningOutcome_SinMatchesEsClear(t *testing.T) {
	out := ClassifyScreeningOutcome(nil, nil)
	if out.Status != models.ScreeningClear {
		t.Errorf("Status = %q, want %q", out.Status, models.ScreeningClear)
	}
	if out.StrongestMatch != "" {
		t.Errorf("StrongestMatch = %q, want vacío", out.StrongestMatch)
	}
}

func TestClassifyScreeningOutcome_MatchFuerteEsMatch(t *testing.T) {
	matches := []models.NormalizedMatch{
		{Name: "Juan Perez", Score: 0.6},
		{Name: "Juan Andres Perez", Score: 0.97}, // EXACT, no es el primero de la lista
	}
	out := ClassifyScreeningOutcome(matches, nil)
	if out.Status != models.ScreeningMatch {
		t.Errorf("Status = %q, want %q", out.Status, models.ScreeningMatch)
	}
	if out.StrongestMatch != "EXACT" {
		t.Errorf("StrongestMatch = %q, want EXACT", out.StrongestMatch)
	}
}

func TestClassifyScreeningOutcome_MatchDebilEsReview(t *testing.T) {
	matches := []models.NormalizedMatch{{Name: "Juan Perez", Score: 0.72}} // MEDIUM
	out := ClassifyScreeningOutcome(matches, nil)
	if out.Status != models.ScreeningReview {
		t.Errorf("Status = %q, want %q", out.Status, models.ScreeningReview)
	}
	if out.StrongestMatch != "MEDIUM" {
		t.Errorf("StrongestMatch = %q, want MEDIUM", out.StrongestMatch)
	}
}

func TestClassifyScreeningOutcome_UmbralesDeFuerza(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{0.95, "EXACT"},
		{0.85, "STRONG"},
		{0.7, "MEDIUM"},
		{0.1, "WEAK"},
	}
	for _, tc := range cases {
		out := ClassifyScreeningOutcome([]models.NormalizedMatch{{Score: tc.score}}, nil)
		if out.StrongestMatch != tc.want {
			t.Errorf("score %.2f: StrongestMatch = %q, want %q", tc.score, out.StrongestMatch, tc.want)
		}
	}
}
```

- [ ] **Step 2: Correr el test, verificar que falla**

Run: `cd backend && go test ./internal/services/... -run TestClassifyScreeningOutcome -v`
Expected: FAIL — `undefined: ClassifyScreeningOutcome`

- [ ] **Step 3: Implementar `backend/internal/services/screening_classify.go`**

```go
package services

import "github.com/thureos/compliance/internal/models"

// ScreeningOutcome es la clasificación pura de un resultado de screening.
type ScreeningOutcome struct {
	Status         models.ScreeningStatus
	StrongestMatch string // EXACT | STRONG | MEDIUM | WEAK | "" si no hay matches
}

// matchStrength convierte un score 0.0-1.0 de Watchman a una categoría —
// mismos umbrales que la referencia de thureos-main.
func matchStrength(score float64) string {
	switch {
	case score >= 0.95:
		return "EXACT"
	case score >= 0.85:
		return "STRONG"
	case score >= 0.7:
		return "MEDIUM"
	default:
		return "WEAK"
	}
}

// ClassifyScreeningOutcome clasifica un resultado de screening:
//   - error del proveedor → REVIEW (fail-closed, nunca CLEAR)
//   - sin matches → CLEAR
//   - el match más fuerte es EXACT/STRONG → MATCH
//   - el match más fuerte es MEDIUM/WEAK → REVIEW
func ClassifyScreeningOutcome(matches []models.NormalizedMatch, searchErr error) ScreeningOutcome {
	if searchErr != nil {
		return ScreeningOutcome{Status: models.ScreeningReview}
	}
	if len(matches) == 0 {
		return ScreeningOutcome{Status: models.ScreeningClear}
	}

	strongest := matches[0]
	for _, m := range matches[1:] {
		if m.Score > strongest.Score {
			strongest = m
		}
	}
	strength := matchStrength(strongest.Score)
	status := models.ScreeningReview
	if strength == "EXACT" || strength == "STRONG" {
		status = models.ScreeningMatch
	}
	return ScreeningOutcome{Status: status, StrongestMatch: strength}
}
```

- [ ] **Step 4: Correr el test, verificar que pasa**

Run: `cd backend && go test ./internal/services/... -run TestClassifyScreeningOutcome -v`
Expected: PASS (5 subtests)

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/screening_classify.go backend/internal/services/screening_classify_test.go
git commit -m "feat(screening): clasificación pura CLEAR/MATCH/REVIEW, fail-closed"
```

---

## Task 3: Cliente Watchman

**Files:**
- Create: `backend/internal/services/watchman_client.go`
- Test: `backend/internal/services/watchman_client_test.go`

**Interfaces:**
- Consumes: `models.NormalizedMatch` (Task 1), `repository.SystemConfigRepository.Get(ctx)` (ya existe)
- Produces: `services.WatchmanClient{httpClient, configRepo}`, `services.NewWatchmanClient(configRepo) *WatchmanClient`, `(*WatchmanClient).Search(ctx, name string, opts WatchmanSearchOptions) ([]models.NormalizedMatch, error)`, `services.WatchmanSearchOptions{Limit, MinMatch, DateOfBirth}`

- [ ] **Step 1: Escribir el test en rojo**

```go
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
```

- [ ] **Step 2: Correr el test, verificar que falla**

Run: `cd backend && go test ./internal/services/... -run TestWatchmanClient -v`
Expected: FAIL — `undefined: WatchmanClient`

- [ ] **Step 3: Implementar `backend/internal/services/watchman_client.go`**

```go
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
		return nil, fmt.Errorf("Watchman: URL no configurada (Configuración → APIs)")
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
		return nil, fmt.Errorf("Watchman respondió HTTP %d: %s", resp.StatusCode, string(body))
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
```

- [ ] **Step 4: Correr el test, verificar que pasa**

Run: `cd backend && go test ./internal/services/... -run TestWatchmanClient -v`
Expected: PASS (3 subtests)

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/watchman_client.go backend/internal/services/watchman_client_test.go
git commit -m "feat(screening): cliente HTTP de Watchman (API v2 /v2/search)"
```

---

## Task 4: Repositorio de screening

**Files:**
- Create: `backend/internal/repository/screening_repo.go`

**Interfaces:**
- Consumes: `models.ScreeningResult`, `models.ScreeningStatus`, `models.ScreeningDismissed`, `models.ScreeningFalsePositive` (Task 1)
- Produces: `repository.ScreeningRepository{col}`, `repository.NewScreeningRepository(db) *ScreeningRepository`, `.Create(ctx, *models.ScreeningResult) error`, `.FindByRedFlagID(ctx, redFlagID primitive.ObjectID) ([]models.ScreeningResult, error)`, `.FindByID(ctx, id primitive.ObjectID) (*models.ScreeningResult, error)`, `.Dismiss(ctx, id primitive.ObjectID, status models.ScreeningStatus, reviewedBy primitive.ObjectID, notes string) error`, `.FindExpiredWhitelist(ctx) ([]models.ScreeningResult, error)`, `.UpdateAfterReeval(ctx, id primitive.ObjectID, status models.ScreeningStatus, strongestMatch string, matches []models.NormalizedMatch) error`

No hay test unitario en esta tarea — es capa de Mongo pura, mismo criterio que el resto de los repositorios del proyecto (`red_flag_repo.go`, `rule_repo.go`, ninguno tiene test unitario porque requieren una instancia real de Mongo). Se verifica en la Tarea 9 contra Mongo real.

- [ ] **Step 1: Implementar `backend/internal/repository/screening_repo.go`**

```go
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/thureos/compliance/internal/database"
	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ScreeningRepository struct {
	col *mongo.Collection
}

func NewScreeningRepository(db *database.MongoDB) *ScreeningRepository {
	r := &ScreeningRepository{col: db.Collection("screening_results")}
	r.ensureIndexes()
	return r
}

func (r *ScreeningRepository) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _ = r.col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "red_flag_id", Value: 1}},
	})
	_, _ = r.col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "status", Value: 1}, {Key: "whitelist_expires_at", Value: 1}},
	})
}

func (r *ScreeningRepository) Create(ctx context.Context, result *models.ScreeningResult) error {
	result.CreatedAt = time.Now()
	res, err := r.col.InsertOne(ctx, result)
	if err != nil {
		return fmt.Errorf("creando screening result: %w", err)
	}
	result.ID = res.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *ScreeningRepository) FindByRedFlagID(ctx context.Context, redFlagID primitive.ObjectID) ([]models.ScreeningResult, error) {
	cursor, err := r.col.Find(ctx, bson.M{"red_flag_id": redFlagID},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("buscando screening results: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var results []models.ScreeningResult
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("decodificando screening results: %w", err)
	}
	return results, nil
}

func (r *ScreeningRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.ScreeningResult, error) {
	var result models.ScreeningResult
	if err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&result); err != nil {
		return nil, fmt.Errorf("screening result %s no encontrado: %w", id.Hex(), err)
	}
	return &result, nil
}

// Dismiss descarta un match/review con nota obligatoria y fija el
// vencimiento del whitelist a models.ScreeningWhitelistDuration — el
// screening_reeval_job lo vuelve a evaluar cuando venza.
func (r *ScreeningRepository) Dismiss(ctx context.Context, id primitive.ObjectID, status models.ScreeningStatus, reviewedBy primitive.ObjectID, notes string) error {
	now := time.Now()
	expiresAt := now.Add(models.ScreeningWhitelistDuration)
	_, err := r.col.UpdateByID(ctx, id, bson.M{"$set": bson.M{
		"status":               status,
		"reviewed_by":          reviewedBy,
		"reviewed_at":          now,
		"review_notes":         notes,
		"whitelist_expires_at": expiresAt,
	}})
	if err != nil {
		return fmt.Errorf("descartando screening result %s: %w", id.Hex(), err)
	}
	return nil
}

// FindExpiredWhitelist devuelve los descartes vencidos — lo que
// screening_reeval_job necesita reevaluar en cada tick.
func (r *ScreeningRepository) FindExpiredWhitelist(ctx context.Context) ([]models.ScreeningResult, error) {
	filter := bson.M{
		"status":               bson.M{"$in": bson.A{models.ScreeningDismissed, models.ScreeningFalsePositive}},
		"whitelist_expires_at": bson.M{"$lt": time.Now()},
	}
	cursor, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("buscando whitelist vencido: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var results []models.ScreeningResult
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("decodificando whitelist vencido: %w", err)
	}
	return results, nil
}

// UpdateAfterReeval reemplaza status/matches tras reevaluar un descarte
// vencido — limpia whitelist_expires_at (ya no es un descarte activo,
// vuelve a ser lo que Watchman diga hoy).
func (r *ScreeningRepository) UpdateAfterReeval(ctx context.Context, id primitive.ObjectID, status models.ScreeningStatus, strongestMatch string, matches []models.NormalizedMatch) error {
	_, err := r.col.UpdateByID(ctx, id, bson.M{
		"$set": bson.M{
			"status":          status,
			"strongest_match": strongestMatch,
			"matches":         matches,
		},
		"$unset": bson.M{"whitelist_expires_at": ""},
	})
	if err != nil {
		return fmt.Errorf("actualizando screening result %s tras reevaluación: %w", id.Hex(), err)
	}
	return nil
}
```

- [ ] **Step 2: Verificar que compila**

Run: `cd backend && go build ./...`
Expected: sin errores.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/repository/screening_repo.go
git commit -m "feat(screening): repositorio Mongo — create, por caso, descarte, whitelist vencido"
```

---

## Task 5: Servicio de orquestación

**Files:**
- Create: `backend/internal/services/screening_service.go`
- Test: `backend/internal/services/screening_service_test.go`

**Interfaces:**
- Consumes: `WatchmanClient.Search` (Task 3), `ClassifyScreeningOutcome` (Task 2), `repository.ScreeningRepository.Create` (Task 4), `models.RedFlag.MatchedData` (ya existe)
- Produces: `services.ScreeningService{client, repo}`, `services.NewScreeningService(client *WatchmanClient, repo *repository.ScreeningRepository) *ScreeningService`, `(*ScreeningService).ScreenAndPersist(ctx, redFlagID *primitive.ObjectID, field, query string) (*models.ScreeningResult, error)`, `extractScreeningValues(matchedData map[string]interface{}, fields []string) map[string]string` (pura, testeada)

- [ ] **Step 1: Escribir el test de `extractScreeningValues` en rojo**

```go
package services

import (
	"reflect"
	"testing"
)

func TestExtractScreeningValues_TomaSoloLosCamposConfigurados(t *testing.T) {
	data := map[string]interface{}{
		"account":     "ACC-1",
		"counterpart": "Juan Perez",
		"amount":      500.0,
	}
	got := extractScreeningValues(data, []string{"counterpart"})
	want := map[string]string{"counterpart": "Juan Perez"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestExtractScreeningValues_CampoAusenteOVacioSeOmite(t *testing.T) {
	data := map[string]interface{}{"counterpart": "", "other": "x"}
	got := extractScreeningValues(data, []string{"counterpart", "no_existe"})
	if len(got) != 0 {
		t.Errorf("got %v, want vacío (campo ausente o vacío no cuenta)", got)
	}
}

func TestExtractScreeningValues_ValorNoStringSeOmite(t *testing.T) {
	data := map[string]interface{}{"amount": 500.0}
	got := extractScreeningValues(data, []string{"amount"})
	if len(got) != 0 {
		t.Errorf("got %v, want vacío (un monto no es un nombre para screenear)", got)
	}
}

func TestExtractScreeningValues_VariosCampos(t *testing.T) {
	data := map[string]interface{}{
		"ordenante":   "Ana Gomez",
		"beneficiario": "Juan Perez",
	}
	got := extractScreeningValues(data, []string{"ordenante", "beneficiario"})
	want := map[string]string{"ordenante": "Ana Gomez", "beneficiario": "Juan Perez"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
```

- [ ] **Step 2: Correr el test, verificar que falla**

Run: `cd backend && go test ./internal/services/... -run TestExtractScreeningValues -v`
Expected: FAIL — `undefined: extractScreeningValues`

- [ ] **Step 3: Implementar `backend/internal/services/screening_service.go`**

```go
package services

import (
	"context"
	"fmt"

	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ScreeningService orquesta un screening: busca en Watchman, clasifica,
// persiste. No decide CUÁNDO screenear (eso lo decide el caller — el
// motor de reglas para el automático, el handler para el manual).
type ScreeningService struct {
	client *WatchmanClient
	repo   *repository.ScreeningRepository
}

func NewScreeningService(client *WatchmanClient, repo *repository.ScreeningRepository) *ScreeningService {
	return &ScreeningService{client: client, repo: repo}
}

// extractScreeningValues toma, de los campos configurados en una regla,
// los que existen en matchedData, no están vacíos y son string — un
// monto o una fecha no es un nombre para screenear.
func extractScreeningValues(matchedData map[string]interface{}, fields []string) map[string]string {
	values := make(map[string]string)
	for _, field := range fields {
		raw, ok := matchedData[field]
		if !ok {
			continue
		}
		str, ok := raw.(string)
		if !ok || str == "" {
			continue
		}
		values[field] = str
	}
	return values
}

// ScreenAndPersist busca `query` en Watchman, clasifica el resultado y
// persiste un ScreeningResult. redFlagID es nulo para búsquedas manuales.
func (s *ScreeningService) ScreenAndPersist(ctx context.Context, redFlagID *primitive.ObjectID, field, query string) (*models.ScreeningResult, error) {
	matches, searchErr := s.client.Search(ctx, query, WatchmanSearchOptions{Limit: 20, MinMatch: 0.5})
	outcome := ClassifyScreeningOutcome(matches, searchErr)

	result := &models.ScreeningResult{
		Query:          query,
		RedFlagID:      redFlagID,
		Field:          field,
		Status:         outcome.Status,
		StrongestMatch: outcome.StrongestMatch,
		Matches:        matches,
	}
	if err := s.repo.Create(ctx, result); err != nil {
		return nil, fmt.Errorf("persistiendo screening result: %w", err)
	}
	return result, nil
}

// ScreenRuleFields dispara un ScreenAndPersist por cada campo configurado
// en la regla que tenga un valor screenable en matchedData — llamado por
// el motor de reglas en su propia goroutine (ver Tarea 6), nunca bloquea
// la creación del red flag.
func (s *ScreeningService) ScreenRuleFields(ctx context.Context, redFlagID primitive.ObjectID, screeningFields []string, matchedData map[string]interface{}) {
	values := extractScreeningValues(matchedData, screeningFields)
	for field, query := range values {
		if _, err := s.ScreenAndPersist(ctx, &redFlagID, field, query); err != nil {
			// No hay caller esperando esto (corre en su propia goroutine) —
			// se loguea y se sigue con el resto de los campos.
			logScreeningError(redFlagID, field, err)
		}
	}
}
```

Nota: `logScreeningError` se implementa en el Step 4 de esta misma tarea, más abajo — si `go build` falla en este paso por la función faltante, es esperado, se resuelve en el siguiente Step.

- [ ] **Step 4: Agregar el helper de logging al final de `screening_service.go`**

```go
func logScreeningError(redFlagID primitive.ObjectID, field string, err error) {
	log.Printf("WARNING: screening: campo %q del red flag %s: %v", field, redFlagID.Hex(), err)
}
```

Y agregar `"log"` al bloque de imports de `screening_service.go`.

- [ ] **Step 5: Correr el test, verificar que pasa**

Run: `cd backend && go test ./internal/services/... -run TestExtractScreeningValues -v`
Expected: PASS (4 subtests)

- [ ] **Step 6: Verificar que compila todo el paquete**

Run: `cd backend && go build ./... && go vet ./...`
Expected: sin errores.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/services/screening_service.go backend/internal/services/screening_service_test.go
git commit -m "feat(screening): servicio de orquestación — extracción de campos + screen-and-persist"
```

---

## Task 6: Disparo automático desde el motor de reglas

**Files:**
- Modify: `backend/internal/services/rule_engine.go`

**Interfaces:**
- Consumes: `ScreeningService.ScreenRuleFields` (Task 5), `models.Rule.ScreeningFields` (Task 1)
- Produces: `(*RuleEngine).SetScreeningService(s *ScreeningService)`, `(*RuleEngine).triggerScreening(rf models.RedFlag, rule models.Rule, isNew bool)`

- [ ] **Step 1: Agregar el campo y el setter, junto a `SetNotifier`**

En `type RuleEngine struct`, agregar junto a `notifier`:

```go
	screening   *ScreeningService
```

Después de `SetNotifier`, agregar:

```go
// SetScreeningService conecta el servicio de screening (opcional, para no
// romper los constructores existentes ni los tests).
func (e *RuleEngine) SetScreeningService(s *ScreeningService) {
	e.screening = s
}

// triggerScreening dispara el screening de los campos configurados en la
// regla, solo para red flags NUEVOS — mismo criterio que triggerNotification
// (evita re-screenear en cada re-trigger del mismo patrón). Corre en su
// propia goroutine, igual que el reporte y la notificación.
func (e *RuleEngine) triggerScreening(rf models.RedFlag, rule models.Rule, isNew bool) {
	if e.screening == nil || !isNew || len(rule.ScreeningFields) == 0 {
		return
	}
	go e.screening.ScreenRuleFields(context.Background(), rf.ID, rule.ScreeningFields, rf.MatchedData)
}
```

- [ ] **Step 2: Llamar `triggerScreening` en `EvaluateRules` — condiciones de fila**

En el bloque `if isNew { ... e.triggerReportGeneration(redFlag); e.triggerNotification(redFlag, isNew) }` de la sección "Evaluate standard row-level conditions", agregar la línea siguiente después de `e.triggerNotification(redFlag, isNew)`:

```go
					e.triggerScreening(redFlag, rule, isNew)
```

- [ ] **Step 3: Llamar `triggerScreening` en `EvaluateRules` — condiciones agregadas**

En el bloque equivalente de "Evaluate aggregate conditions", después de `e.triggerNotification(aggRedFlags[i], isNew)`:

```go
					e.triggerScreening(aggRedFlags[i], rule, isNew)
```

- [ ] **Step 4: Llamar `triggerScreening` en `EvaluateRuleForDateRange`**

Este es el camino que evalúa el scheduler — la Tarea 5 del roadmap P1 ya encontró y corrigió que a este camino le faltaba `triggerNotification`; no se debe repetir el mismo gap con screening. Justo después de la línea `e.triggerNotification(candidate, isNew)` (agregada en esa corrección, dentro del `for i := range candidates` de `EvaluateRuleForDateRange`):

```go
			e.triggerScreening(candidate, rule, isNew)
```

**No** se agrega a `matchRuleForDateRange` ni a `BacktestRule` — esos son el camino compartido con el backtest (Tarea 4 del roadmap P1), que no debe tener efectos secundarios.

- [ ] **Step 5: Verificar que compila**

Run: `cd backend && go build ./... && go vet ./...`
Expected: sin errores.

- [ ] **Step 6: Correr toda la suite — nada debe romperse**

Run: `cd backend && go test ./...`
Expected: PASS en todos los paquetes (esta tarea no agrega tests propios — es wiring; se verifica manualmente contra Mongo+Watchman real en la Tarea 9, igual que se hizo con la Tarea 6 del roadmap P1 para el filtro de agregados).

- [ ] **Step 7: Commit**

```bash
git add backend/internal/services/rule_engine.go
git commit -m "feat(screening): disparo automático desde el motor de reglas (upload + scheduler)"
```

---

## Task 7: Notificación de reevaluación (whitelist vencido)

**Files:**
- Modify: `backend/internal/services/notification_service.go`

**Interfaces:**
- Consumes: `NotificationService.enqueueNotification` (ya existe, Tarea 5 del roadmap P1)
- Produces: `services.NotifyTypeScreeningStale` (const), `(*NotificationService).TriggerScreeningStale(result models.ScreeningResult) error`

- [ ] **Step 1: Agregar el nuevo tipo de notificación**

Junto a las consts existentes:

```go
const (
	NotifyTypeRedFlagCreated = "red_flag.created"
	NotifyTypeSLABreach      = "sla_breach"
	NotifyTypeScreeningStale = "screening_stale"
)
```

- [ ] **Step 2: Agregar `TriggerScreeningStale`, después de `TriggerSLABreach`**

```go
// TriggerScreeningStale encola el aviso de que un descarte de screening
// vencido (whitelist expirado) volvió a dar match/review al reevaluarse
// — alguien lo cerró hace 180+ días y nadie está mirando ese caso hoy,
// a diferencia de un red flag recién creado. Devuelve error como
// TriggerSLABreach: el caller (screening_reeval_job) decide qué hacer
// según si el aviso salió de verdad.
func (s *NotificationService) TriggerScreeningStale(result models.ScreeningResult) error {
	redFlagRef := "búsqueda manual"
	if result.RedFlagID != nil {
		redFlagRef = result.RedFlagID.Hex()
	}
	return s.enqueueNotification(&NotifyPayload{
		Type:        NotifyTypeScreeningStale,
		RedFlagID:   redFlagRef,
		MonitorName: "",
		RuleName:    "",
		Severity:    "high",
		Message:     fmt.Sprintf("Screening reevaluado de %q vuelve a dar %s tras vencer el whitelist", result.Query, result.Status),
		MatchCount:  len(result.Matches),
	})
}
```

- [ ] **Step 3: Verificar que compila**

Run: `cd backend && go build ./... && go vet ./...`
Expected: sin errores.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/services/notification_service.go
git commit -m "feat(screening): tipo de notificación screening_stale + TriggerScreeningStale"
```

---

## Task 8: Job de reevaluación de whitelist vencido

**Files:**
- Create: `backend/internal/services/screening_reeval_job.go`

**Interfaces:**
- Consumes: `ScreeningRepository.FindExpiredWhitelist`, `.UpdateAfterReeval` (Task 4), `WatchmanClient.Search` (Task 3), `ClassifyScreeningOutcome` (Task 2), `NotificationService.TriggerScreeningStale` (Task 7)
- Produces: `services.ScreeningReevalJob{cron, repo, client, notifier}`, `services.NewScreeningReevalJob(repo, client, notifier) *ScreeningReevalJob`, `(*ScreeningReevalJob).Start(ctx)`, `.Status() map[string]interface{}`

No hay test unitario — mismo criterio que `sla_escalation.go` (Tarea 5 del roadmap P1): la lógica de filtro/cron toca Mongo, se verifica manualmente en la Tarea 9.

- [ ] **Step 1: Implementar `backend/internal/services/screening_reeval_job.go`**

```go
package services

import (
	"context"
	"log"
	"sync"

	"github.com/robfig/cron/v3"
	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
)

// ScreeningReevalJob reabre descartes de screening vencidos (whitelist
// expirado): los vuelve a correr contra Watchman y, si siguen dando
// match/review, avisa — un descarte de hace 180+ días no debe quedar
// congelado en falso positivo para siempre. Mismo patrón de polling que
// SLAEscalationJob (Tarea 5 del roadmap P1).
type ScreeningReevalJob struct {
	cron     *cron.Cron
	repo     *repository.ScreeningRepository
	client   *WatchmanClient
	notifier *NotificationService
	mu       sync.RWMutex
	running  bool
}

func NewScreeningReevalJob(repo *repository.ScreeningRepository, client *WatchmanClient, notifier *NotificationService) *ScreeningReevalJob {
	return &ScreeningReevalJob{repo: repo, client: client, notifier: notifier}
}

// Start launches the job. Blocks until ctx is cancelled — call with `go`.
func (j *ScreeningReevalJob) Start(ctx context.Context) {
	j.cron = cron.New()
	// Diario alcanza: un vencimiento de 180 días no necesita chequeo por
	// minuto como el de SLA (que vence en horas).
	if _, err := j.cron.AddFunc("0 4 * * *", func() {
		j.checkAndReeval(ctx)
	}); err != nil {
		log.Printf("screening reeval: registrando chequeo: %v", err)
	}

	j.mu.Lock()
	j.running = true
	j.mu.Unlock()

	log.Println("Screening reeval job started — checking expired whitelist daily at 04:00")
	j.cron.Start()

	<-ctx.Done()

	log.Println("Screening reeval job stopping...")
	stopCtx := j.cron.Stop()
	<-stopCtx.Done()

	j.mu.Lock()
	j.running = false
	j.mu.Unlock()
	log.Println("Screening reeval job stopped")
}

func (j *ScreeningReevalJob) checkAndReeval(ctx context.Context) {
	expired, err := j.repo.FindExpiredWhitelist(ctx)
	if err != nil {
		log.Printf("screening reeval: buscando whitelist vencido: %v", err)
		return
	}
	for _, result := range expired {
		matches, searchErr := j.client.Search(ctx, result.Query, WatchmanSearchOptions{Limit: 20, MinMatch: 0.5})
		outcome := ClassifyScreeningOutcome(matches, searchErr)

		if err := j.repo.UpdateAfterReeval(ctx, result.ID, outcome.Status, outcome.StrongestMatch, matches); err != nil {
			log.Printf("screening reeval: actualizando %s: %v", result.ID.Hex(), err)
			continue
		}

		stillFlagged := outcome.Status == models.ScreeningMatch || outcome.Status == models.ScreeningReview
		if stillFlagged && j.notifier != nil {
			result.Status = outcome.Status
			result.Matches = matches
			if err := j.notifier.TriggerScreeningStale(result); err != nil {
				log.Printf("screening reeval: avisando %s: %v", result.ID.Hex(), err)
			}
		}
	}
}

func (j *ScreeningReevalJob) Status() map[string]interface{} {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return map[string]interface{}{"running": j.running}
}
```

- [ ] **Step 2: Verificar que compila**

Run: `cd backend && go build ./... && go vet ./...`
Expected: sin errores.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/services/screening_reeval_job.go
git commit -m "feat(screening): job diario de reevaluación de whitelist vencido"
```

---

## Task 9: Handlers, rutas, config y wiring

**Files:**
- Create: `backend/internal/handlers/screening_handler.go`
- Modify: `backend/internal/models/activity_log.go`
- Modify: `backend/internal/services/screening_service.go` (agrega `ScreenAndPersistEphemeral`, Step 3)
- Modify: `backend/internal/router/router.go`
- Modify: `backend/cmd/server/main.go`

**Interfaces:**
- Consumes: `ScreeningService.ScreenAndPersist` (Task 5), `ScreeningRepository.FindByRedFlagID/FindByID/Dismiss` (Task 4), `models.ScreeningDismissed/ScreeningFalsePositive` (Task 1)
- Produces: `handlers.ScreeningHandler{screeningService, screeningRepo, activityRepo}`, `handlers.NewScreeningHandler(...)`, `.Search`, `.GetByRedFlag`, `.Dismiss` — endpoints montados en el router

- [ ] **Step 1: Agregar el tipo de actividad, en `backend/internal/models/activity_log.go`**

```go
	ActivityScreeningDismiss ActivityType = "screening_dismiss"
```

- [ ] **Step 2: Implementar `backend/internal/handlers/screening_handler.go`**

```go
package handlers

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
	"github.com/thureos/compliance/internal/services"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ScreeningHandler struct {
	screeningService *services.ScreeningService
	screeningRepo    *repository.ScreeningRepository
	activityRepo     *repository.ActivityLogRepository
}

func NewScreeningHandler(
	screeningService *services.ScreeningService,
	screeningRepo *repository.ScreeningRepository,
	activityRepo *repository.ActivityLogRepository,
) *ScreeningHandler {
	return &ScreeningHandler{
		screeningService: screeningService,
		screeningRepo:    screeningRepo,
		activityRepo:     activityRepo,
	}
}

type searchScreeningRequest struct {
	Name        string `json:"name"`
	DateOfBirth string `json:"dateOfBirth,omitempty"`
}

// Search es una búsqueda manual ad-hoc: no persiste nada, misma lógica
// que el disparo automático (mismo clasificador) pero sin caso asociado.
func (h *ScreeningHandler) Search(c *fiber.Ctx) error {
	var req searchScreeningRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name es obligatorio"})
	}

	result, err := h.screeningService.ScreenAndPersistEphemeral(c.Context(), req.Name, req.DateOfBirth)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

// GetByRedFlag devuelve los screenings linkeados a un caso.
func (h *ScreeningHandler) GetByRedFlag(c *fiber.Ctx) error {
	redFlagID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid red flag ID"})
	}
	results, err := h.screeningRepo.FindByRedFlagID(c.Context(), redFlagID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if results == nil {
		results = []models.ScreeningResult{}
	}
	return c.JSON(results)
}

type dismissScreeningRequest struct {
	Status models.ScreeningStatus `json:"status"` // dismissed | false_positive
	Notes  string                 `json:"notes"`
}

// Dismiss descarta un match/review con nota obligatoria — mismo principio
// que cerrar un caso exige disposition (Tarea 8 del roadmap P0).
func (h *ScreeningHandler) Dismiss(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid screening result ID"})
	}
	var req dismissScreeningRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Status != models.ScreeningDismissed && req.Status != models.ScreeningFalsePositive {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "status debe ser dismissed o false_positive"})
	}
	if req.Notes == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "notes es obligatorio para descartar un screening"})
	}

	userID, err := caseUserID(c)
	if err != nil {
		return err
	}

	if err := h.screeningRepo.Dismiss(c.Context(), id, req.Status, userID, req.Notes); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if h.activityRepo != nil {
		userName, _ := c.Locals("email").(string)
		_ = h.activityRepo.Create(c.Context(), &models.ActivityLog{
			UserID:    userID,
			UserName:  userName,
			UserEmail: userName,
			Action:    models.ActivityScreeningDismiss,
			Detail:    fmt.Sprintf("Screening %s: descartado como %s — %s", id.Hex(), req.Status, req.Notes),
			Resource:  "screening_result",
			IP:        c.IP(),
		})
	}

	return c.JSON(fiber.Map{"message": "screening descartado"})
}
```

- [ ] **Step 3: Agregar `ScreenAndPersistEphemeral` a `screening_service.go`**

La búsqueda manual no persiste (spec, sección C) — agregar al final de `backend/internal/services/screening_service.go`:

```go
// ScreenAndPersistEphemeral es la búsqueda manual: corre Search()+clasifica
// pero NO persiste — es una herramienta de investigación ad-hoc, no un
// registro de caso. Nombre distinto a ScreenAndPersist a propósito: que
// quede claro en el call site que este camino no deja rastro en Mongo.
func (s *ScreeningService) ScreenAndPersistEphemeral(ctx context.Context, query, dateOfBirth string) (*models.ScreeningResult, error) {
	matches, searchErr := s.client.Search(ctx, query, WatchmanSearchOptions{Limit: 20, MinMatch: 0.5, DateOfBirth: dateOfBirth})
	outcome := ClassifyScreeningOutcome(matches, searchErr)
	return &models.ScreeningResult{
		Query:          query,
		Status:         outcome.Status,
		StrongestMatch: outcome.StrongestMatch,
		Matches:        matches,
	}, nil
}
```

- [ ] **Step 4: Agregar las rutas en `backend/internal/router/router.go`**

Agregar `Screening *handlers.ScreeningHandler` a `type Handlers struct`.

Después del bloque de rutas de `red-flags` (junto a `redFlags.Get("/:id/notes", ...)`), agregar:

```go
	redFlags.Get("/:id/screening", h.Screening.GetByRedFlag)
```

Nuevo grupo, después del grupo `templateAPI`:

```go
	// Screening de sanciones: buscar es compliance+ (misma vara que crear
	// reglas); descartar un match exige nota, igual que cerrar un caso.
	screening := protected.Group("/screening")
	screening.Post("/search", middleware.RequireComplianceOrAbove(), h.Screening.Search)
	screening.Post("/:id/dismiss", middleware.RequireComplianceOrAbove(), h.Screening.Dismiss)
```

Config de Watchman (admin only), junto al bloque de `/settings/notifications`:

```go
	settings.Get("/screening", func(c *fiber.Ctx) error {
		cfg, err := h.ConfigRepo.Get(c.Context())
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load config"})
		}
		return c.JSON(fiber.Map{"watchmanUrl": cfg.Watchman.URL})
	})

	settings.Put("/screening", func(c *fiber.Ctx) error {
		var body struct {
			WatchmanURL string `json:"watchmanUrl"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		current, err := h.ConfigRepo.Get(c.Context())
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load current config"})
		}
		current.Watchman.URL = body.WatchmanURL
		current.UpdatedBy, _ = c.Locals("email").(string)
		if err := h.ConfigRepo.Upsert(c.Context(), current); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"message": "screening actualizado"})
	})
```

- [ ] **Step 5: Wiring en `backend/cmd/server/main.go`**

Junto a la construcción de `notificationService` (después de `ruleEngine.SetNotifier(notificationService)`):

```go
	watchmanClient := services.NewWatchmanClient(systemConfigRepo)
	screeningRepo := repository.NewScreeningRepository(mongo)
	screeningService := services.NewScreeningService(watchmanClient, screeningRepo)
	ruleEngine.SetScreeningService(screeningService)
```

Junto a la construcción de `slaEscalationJob`:

```go
	// Initialize screening reeval job
	screeningReevalJob := services.NewScreeningReevalJob(screeningRepo, watchmanClient, notificationService)
	screeningReevalCtx, screeningReevalCancel := context.WithCancel(context.Background())
	defer screeningReevalCancel()
	go screeningReevalJob.Start(screeningReevalCtx)
```

En el literal `h := &router.Handlers{...}`, agregar:

```go
		Screening:     handlers.NewScreeningHandler(screeningService, screeningRepo, activityLogRepo),
```

- [ ] **Step 6: Verificar que compila, formatea y lintea**

Run: `cd backend && go build ./... && go vet ./... && gofmt -l . && golangci-lint run ./...`
Expected: sin errores, `gofmt -l .` sin salida, `golangci-lint` en 0 issues.

- [ ] **Step 7: Correr toda la suite**

Run: `cd backend && go test ./...`
Expected: PASS en todos los paquetes.

- [ ] **Step 8: Verificación manual contra Mongo+Watchman real**

Mismo criterio que el resto del roadmap P1 (Tareas 4, 5, 6): levantar el backend contra Mongo/Redis reales, y contra un Watchman real o un `httptest`-style servidor que imite `/v2/search` si el VPS no es alcanzable desde la sesión de desarrollo. Verificar:
- `POST /api/v1/screening/search` con un nombre conocido de una lista de prueba → clasifica correctamente, no persiste (confirmar que `screening_results` no creció).
- Una regla con `ScreeningFields` configurado dispara al crear un red flag nuevo → aparece un `ScreeningResult` con `redFlagId` seteado.
- `GET /api/v1/red-flags/:id/screening` devuelve ese resultado.
- `POST /api/v1/screening/:id/dismiss` sin `notes` → 400. Con `notes` → 200, `whitelistExpiresAt` queda ~180 días adelante.
- Apagar el proveedor (URL inválida) → el resultado clasifica REVIEW, nunca CLEAR.

- [ ] **Step 9: Commit**

```bash
git add backend/internal/handlers/screening_handler.go backend/internal/services/screening_service.go backend/internal/models/activity_log.go backend/internal/router/router.go backend/cmd/server/main.go
git commit -m "feat(screening): endpoints, config de Watchman en Settings, wiring completo"
```

---

## Task 10: Frontend — cliente API y página de búsqueda manual

**Files:**
- Create: `frontend/src/lib/api/screening.ts`
- Create: `frontend/src/app/(dashboard)/screening/page.tsx`
- Modify: `frontend/src/components/layout/top-nav.tsx`

**Interfaces:**
- Consumes: endpoints de la Tarea 9 (`POST /screening/search`, `GET/PUT /settings/screening`)
- Produces: `screeningApi.search`, `screeningApi.getByRedFlag`, `screeningApi.dismiss`, `screeningApi.getConfig`, `screeningApi.updateConfig`; tipos `ScreeningResult`, `NormalizedMatch`

- [ ] **Step 1: Crear `frontend/src/lib/api/screening.ts`**

```typescript
import { api } from "./client";

export interface NormalizedMatch {
  name: string;
  entityType: string;
  sourceList: string;
  score: number;
  programs?: string[];
}

export type ScreeningStatus =
  | "clear"
  | "match"
  | "review"
  | "dismissed"
  | "false_positive";

export interface ScreeningResult {
  id: string;
  query: string;
  redFlagId?: string;
  field?: string;
  status: ScreeningStatus;
  strongestMatch?: string;
  matches?: NormalizedMatch[];
  reviewedBy?: string;
  reviewedAt?: string;
  reviewNotes?: string;
  whitelistExpiresAt?: string;
  createdAt: string;
}

export const screeningApi = {
  search: (name: string, dateOfBirth?: string) =>
    api.post<ScreeningResult>("/screening/search", { name, dateOfBirth }),

  getByRedFlag: (redFlagId: string) =>
    api.get<ScreeningResult[]>(`/red-flags/${redFlagId}/screening`),

  dismiss: (
    id: string,
    data: { status: "dismissed" | "false_positive"; notes: string },
  ) => api.post<{ message: string }>(`/screening/${id}/dismiss`, data),

  getConfig: () => api.get<{ watchmanUrl: string }>("/settings/screening"),

  updateConfig: (watchmanUrl: string) =>
    api.put<{ message: string }>("/settings/screening", { watchmanUrl }),
};
```

- [ ] **Step 2: Crear `frontend/src/app/(dashboard)/screening/page.tsx`**

```tsx
"use client";

import { useState } from "react";
import { Header } from "@/components/layout/header";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { useToast } from "@/lib/use-toast";
import { screeningApi, type ScreeningResult } from "@/lib/api/screening";

const STATUS_LABEL: Record<string, string> = {
  clear: "Limpio",
  match: "Coincidencia",
  review: "Revisar",
};

const STATUS_VARIANT: Record<string, "success" | "destructive" | "warning"> = {
  clear: "success",
  match: "destructive",
  review: "warning",
};

export default function ScreeningPage() {
  const { toastError } = useToast();
  const [name, setName] = useState("");
  const [dateOfBirth, setDateOfBirth] = useState("");
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<ScreeningResult | null>(null);

  async function search(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    setLoading(true);
    setResult(null);
    try {
      const res = await screeningApi.search(name, dateOfBirth || undefined);
      setResult(res);
    } catch (err) {
      toastError(
        err instanceof Error ? err.message : "Error al buscar en Watchman",
      );
    } finally {
      setLoading(false);
    }
  }

  return (
    <>
      <Header title="Screening de sanciones" />
      <div className="p-6 space-y-4 max-w-2xl">
        <p className="text-sm text-muted-foreground">
          Búsqueda ad-hoc contra OFAC/ONU/UE/UK (Watchman). No queda
          registrada — para screening con caso asociado, ver el detalle de
          la bandera roja correspondiente.
        </p>
        <form onSubmit={search} className="flex gap-2 items-end">
          <div className="flex-1 space-y-1.5">
            <Label>Nombre</Label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Nombre completo"
              required
            />
          </div>
          <div className="w-40 space-y-1.5">
            <Label>Fecha de nac. (opcional)</Label>
            <Input
              type="date"
              value={dateOfBirth}
              onChange={(e) => setDateOfBirth(e.target.value)}
            />
          </div>
          <Button type="submit" disabled={loading}>
            {loading ? "Buscando…" : "Buscar"}
          </Button>
        </form>

        {result && (
          <Card>
            <CardContent className="pt-6 space-y-3">
              <div className="flex items-center gap-2">
                <Badge variant={STATUS_VARIANT[result.status]}>
                  {STATUS_LABEL[result.status] || result.status}
                </Badge>
                <span className="text-sm text-muted-foreground">
                  {result.matches?.length || 0} coincidencia(s)
                </span>
              </div>
              {(result.matches || []).map((m, i) => (
                <div key={i} className="rounded-md border p-3 text-sm">
                  <p className="font-medium">{m.name}</p>
                  <p className="text-xs text-muted-foreground">
                    {m.sourceList} · score {m.score.toFixed(2)}
                    {m.programs && m.programs.length > 0
                      ? ` · ${m.programs.join(", ")}`
                      : ""}
                  </p>
                </div>
              ))}
            </CardContent>
          </Card>
        )}
      </div>
    </>
  );
}
```

- [ ] **Step 3: Agregar la entrada de nav en `frontend/src/components/layout/top-nav.tsx`**

Agregar `ScanSearch` al import de `lucide-react`.

En el área `"Monitoreo"`, agregar después de `"Reglas"`:

```typescript
      { name: "Screening", href: "/screening", icon: ScanSearch },
```

- [ ] **Step 4: Lint y build**

Run: `cd frontend && npm run lint`
Expected: 0 errores (warnings preexistentes sin cambios).

Run: `cd frontend && npm run build`
Expected: compila, `/screening` aparece en la lista de rutas.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/api/screening.ts "frontend/src/app/(dashboard)/screening/page.tsx" frontend/src/components/layout/top-nav.tsx
git commit -m "feat(screening): página de búsqueda manual + entrada de nav"
```

---

## Task 11: Frontend — detalle de caso y configuración

**Files:**
- Modify: `frontend/src/app/(dashboard)/red-flags/[id]/page.tsx`
- Create: `frontend/src/components/settings/screening-card.tsx`
- Modify: `frontend/src/app/(dashboard)/settings/page.tsx`

**Interfaces:**
- Consumes: `screeningApi.getByRedFlag`, `.dismiss`, `.getConfig`, `.updateConfig` (Task 10)

- [ ] **Step 1: Agregar la sección de screening en `red-flags/[id]/page.tsx`**

Agregar el import: `import { screeningApi, type ScreeningResult } from "@/lib/api/screening";`

Agregar estado, junto a `const [notes, setNotes] = useState<CaseNote[]>([]);`:

```typescript
  const [screenings, setScreenings] = useState<ScreeningResult[]>([]);
```

En la función `load` (el `useCallback` existente), agregar `screeningApi.getByRedFlag(id)` al `Promise.all` y guardarlo:

```typescript
  const load = useCallback(async () => {
    try {
      const [flag, caseNotes, screeningResults] = await Promise.all([
        redFlagsApi.get(id),
        redFlagsApi.listNotes(id),
        screeningApi.getByRedFlag(id),
      ]);
      setRf(flag);
      setNotes(caseNotes);
      setScreenings(screeningResults);
    } catch {
      toastError("Error al cargar el caso");
    }
  }, [id]);
```

Agregar estado para el control de descarte inline, junto al resto de `useState` del componente:

```typescript
  const [dismissingId, setDismissingId] = useState<string | null>(null);
  const [dismissNotes, setDismissNotes] = useState("");
  const [dismissing, setDismissing] = useState(false);

  async function dismissScreening(
    screeningId: string,
    status: "dismissed" | "false_positive",
  ) {
    if (!dismissNotes.trim()) {
      toastError("La nota es obligatoria para descartar un screening");
      return;
    }
    setDismissing(true);
    try {
      await screeningApi.dismiss(screeningId, { status, notes: dismissNotes });
      toastSuccess("Screening descartado");
      setDismissingId(null);
      setDismissNotes("");
      await load();
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Error al descartar");
    } finally {
      setDismissing(false);
    }
  }
```

Agregar la sección visual, dentro del panel "Investigación" (junto al timeline de notas), antes del cierre de esa tarjeta:

```tsx
          {screenings.length > 0 && (
            <div className="mt-4 space-y-2">
              <h3 className="text-sm font-medium">Screening de sanciones</h3>
              {screenings.map((s) => {
                const open = s.status === "match" || s.status === "review";
                return (
                  <div key={s.id} className="rounded-md border p-3 text-xs space-y-2">
                    <div className="flex items-center justify-between">
                      <span className="font-medium">{s.field}: {s.query}</span>
                      <span
                        className={cn(
                          "rounded px-2 py-0.5 text-[10px] font-medium",
                          s.status === "clear" && "bg-success-bg text-success-fg",
                          open && "bg-danger-bg text-danger-fg",
                          (s.status === "dismissed" || s.status === "false_positive") &&
                            "bg-muted text-muted-foreground",
                        )}
                      >
                        {s.status}
                      </span>
                    </div>
                    {(s.matches || []).slice(0, 3).map((m, i) => (
                      <p key={i} className="text-muted-foreground">
                        {m.name} — {m.sourceList} ({m.score.toFixed(2)})
                      </p>
                    ))}

                    {open && dismissingId !== s.id && canAct && (
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-6 text-[11px]"
                        onClick={() => {
                          setDismissingId(s.id);
                          setDismissNotes("");
                        }}
                      >
                        Descartar
                      </Button>
                    )}

                    {dismissingId === s.id && (
                      <div className="space-y-2 pt-1">
                        <Input
                          value={dismissNotes}
                          onChange={(e) => setDismissNotes(e.target.value)}
                          placeholder="Nota obligatoria: por qué se descarta"
                          className="text-xs h-7"
                        />
                        <div className="flex gap-2">
                          <Button
                            size="sm"
                            className="h-6 text-[11px]"
                            disabled={dismissing}
                            onClick={() => dismissScreening(s.id, "false_positive")}
                          >
                            Falso positivo
                          </Button>
                          <Button
                            size="sm"
                            variant="outline"
                            className="h-6 text-[11px]"
                            disabled={dismissing}
                            onClick={() => dismissScreening(s.id, "dismissed")}
                          >
                            Descartar
                          </Button>
                          <Button
                            size="sm"
                            variant="ghost"
                            className="h-6 text-[11px]"
                            onClick={() => setDismissingId(null)}
                          >
                            Cancelar
                          </Button>
                        </div>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          )}
```

`Input` y `Button` ya están importados en este archivo; `canAct` también ya existe (`user.role !== "viewer"`) — reusados tal cual, no hay que agregar imports nuevos para este bloque salvo `screeningApi`/`ScreeningResult` (Step 1, más arriba) y `cn` (ya importado, se usa en otras partes del archivo).

- [ ] **Step 2: Crear `frontend/src/components/settings/screening-card.tsx`**

```tsx
"use client";

import { useEffect, useState } from "react";
import { ScanSearch } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useToast } from "@/lib/use-toast";
import { screeningApi } from "@/lib/api/screening";

export function ScreeningCard() {
  const { toastError, toastSuccess } = useToast();
  const [watchmanUrl, setWatchmanUrl] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    screeningApi
      .getConfig()
      .then((cfg) => setWatchmanUrl(cfg.watchmanUrl))
      .catch(() => toastError("Error al cargar configuración de screening"));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function save() {
    setSaving(true);
    try {
      await screeningApi.updateConfig(watchmanUrl);
      toastSuccess("Configuración de screening guardada");
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Error al guardar");
    } finally {
      setSaving(false);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-base">
          <ScanSearch className="h-4 w-4" /> Screening de sanciones
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-1.5">
          <Label>URL de Watchman</Label>
          <Input
            value={watchmanUrl}
            onChange={(e) => setWatchmanUrl(e.target.value)}
            placeholder="http://marion-watchman:8084"
          />
          <p className="text-xs text-muted-foreground">
            Servicio de screening contra OFAC/ONU/UE/UK — se opera fuera de
            esta app; acá solo se configura la URL.
          </p>
        </div>
        <div className="flex justify-end">
          <Button onClick={save} disabled={saving}>
            {saving ? "Guardando…" : "Guardar"}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
```

- [ ] **Step 3: Montar `ScreeningCard` en `frontend/src/app/(dashboard)/settings/page.tsx`**

Agregar el import: `import { ScreeningCard } from "@/components/settings/screening-card";`

Agregar `<ScreeningCard />` junto a `<NotificationsCard />` (misma sección admin-only).

- [ ] **Step 4: Lint y build**

Run: `cd frontend && npm run lint`
Expected: 0 errores.

Run: `cd frontend && npm run build`
Expected: compila.

- [ ] **Step 5: Commit**

```bash
git add "frontend/src/app/(dashboard)/red-flags/[id]/page.tsx" frontend/src/components/settings/screening-card.tsx "frontend/src/app/(dashboard)/settings/page.tsx"
git commit -m "feat(screening): sección en detalle de caso + config de Watchman en Settings"
```

---

## Task 12: Frontend — selector de campos a screenear en el formulario de regla

**Files:**
- Modify: `frontend/src/app/(dashboard)/rules/page.tsx`
- Modify: `frontend/src/lib/api/rules.ts`

**Interfaces:**
- Consumes: `Rule.screeningFields`, `CreateRuleRequest.screeningFields` (Task 1, ya expuesto por JSON)

- [ ] **Step 1: Agregar `screeningFields` a los tipos en `frontend/src/lib/api/rules.ts`**

En el objeto que recibe `create`, agregar `screeningFields?: string[];` junto a `regulatoryBasis?: string;`.

- [ ] **Step 2: Agregar el campo a `createForm` y `editForm` en `rules/page.tsx`**

En ambos `useState({...})` (create y edit), agregar `screeningFields: [] as string[],`.

- [ ] **Step 3: Agregar el selector en el formulario de creación**

Después del bloque de campos regulatorios (`fatfTypology`/`thresholdJustification`/`regulatoryBasis`), agregar:

```tsx
                  <div className="space-y-2">
                    <Label className="text-xs text-muted-foreground">
                      Campos a screenear contra sanciones (opcional)
                    </Label>
                    <div className="flex flex-wrap gap-2">
                      {(monitors.find((m) => m.id === createForm.monitorId)?.schema || []).map(
                        (f) => {
                          const active = createForm.screeningFields.includes(f.name);
                          return (
                            <button
                              key={f.name}
                              type="button"
                              onClick={() =>
                                setCreateForm((prev) => ({
                                  ...prev,
                                  screeningFields: active
                                    ? prev.screeningFields.filter((n) => n !== f.name)
                                    : [...prev.screeningFields, f.name],
                                }))
                              }
                              className={`rounded-md border px-2 py-1 text-xs ${
                                active ? "border-primary bg-primary/10" : ""
                              }`}
                            >
                              {f.name}
                            </button>
                          );
                        },
                      )}
                    </div>
                  </div>
```

Y en `createRule`, agregar `screeningFields: createForm.screeningFields.length > 0 ? createForm.screeningFields : undefined,` al body de `rulesApi.create(...)`.

- [ ] **Step 4: Agregar el selector en el formulario de edición**

Después del bloque equivalente de campos regulatorios en el diálogo de edición, agregar:

```tsx
                  <div className="space-y-2">
                    <Label className="text-xs text-muted-foreground">
                      Campos a screenear contra sanciones (opcional)
                    </Label>
                    <div className="flex flex-wrap gap-2">
                      {(editMonitorSchema || []).map((f) => {
                        const active = editForm.screeningFields.includes(f.name);
                        return (
                          <button
                            key={f.name}
                            type="button"
                            onClick={() =>
                              setEditForm((prev) => ({
                                ...prev,
                                screeningFields: active
                                  ? prev.screeningFields.filter((n) => n !== f.name)
                                  : [...prev.screeningFields, f.name],
                              }))
                            }
                            className={`rounded-md border px-2 py-1 text-xs ${
                              active ? "border-primary bg-primary/10" : ""
                            }`}
                          >
                            {f.name}
                          </button>
                        );
                      })}
                    </div>
                  </div>
```

`editMonitorSchema` ya existe en el archivo (`const editMonitorSchema = editingRule ? monitors.find((m) => m.id === editingRule.monitorId)?.schema : undefined;`).

En `saveEditRule`, agregar `screeningFields: editForm.screeningFields.length > 0 ? editForm.screeningFields : undefined,` al body de `rulesApi.update(editingRule.id, {...})`.

- [ ] **Step 5: Lint y build**

Run: `cd frontend && npm run lint`
Expected: 0 errores.

Run: `cd frontend && npm run build`
Expected: compila.

- [ ] **Step 6: Commit**

```bash
git add "frontend/src/app/(dashboard)/rules/page.tsx" frontend/src/lib/api/rules.ts
git commit -m "feat(screening): selector de campos a screenear en el formulario de regla"
```

---

## Criterio de cierre del lote

- Gate (`go build`, `go vet`, `gofmt -l .`, `golangci-lint run ./...`, `go test ./...`, `npm run lint`, `npm run build`) sin bloqueantes en las 12 tareas.
- Recorrido manual: crear una regla con `ScreeningFields` → subir datos que la disparen → el red flag resultante tiene un `ScreeningResult` linkeado, visible en su detalle → descartar con nota → forzar el vencimiento del whitelist (mongo directo, como se hizo con el SLA en la Tarea 5 del roadmap P1) → el job de reevaluación lo vuelve a correr y avisa si sigue dando match.
- Búsqueda manual desde `/screening` funciona sin dejar rastro en `screening_results`.
- Apagar/desconfigurar la URL de Watchman → toda búsqueda clasifica REVIEW, nunca CLEAR (fail-closed verificado en vivo, no solo por unit test).
