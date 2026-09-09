package handlers

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
	"github.com/thureos/compliance/internal/services"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type MonitorHandler struct {
	monitorRepo      *repository.MonitorRepository
	ingestionService *services.IngestionService
	jobQueue         *services.JobQueue
	ruleEngine       *services.RuleEngine
	uploadLogRepo    *repository.UploadLogRepository
}

func NewMonitorHandler(
	monitorRepo *repository.MonitorRepository,
	ingestionService *services.IngestionService,
	jobQueue *services.JobQueue,
	ruleEngine *services.RuleEngine,
	uploadLogRepo *repository.UploadLogRepository,
) *MonitorHandler {
	return &MonitorHandler{
		monitorRepo:      monitorRepo,
		ingestionService: ingestionService,
		jobQueue:         jobQueue,
		ruleEngine:       ruleEngine,
		uploadLogRepo:    uploadLogRepo,
	}
}

func (h *MonitorHandler) Create(c *fiber.Ctx) error {
	var req models.CreateMonitorRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	userID, _ := primitive.ObjectIDFromHex(c.Locals("userId").(string))

	monitor := &models.Monitor{
		Name:         req.Name,
		Description:  req.Description,
		SourceType:   req.SourceType,
		SourceConfig: req.SourceConfig.ToSourceConfig(),
		Schema:       req.Schema,
		CollectionID: primitive.NewObjectID().Hex(),
		OwnerID:      userID,
	}

	if err := validateSourceConfig(monitor.SourceType, monitor.SourceConfig); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var pushToken string
	if monitor.SourceType == models.SourceAPI && monitor.SourceConfig != nil && monitor.SourceConfig.Mode == models.APIModePush {
		token, err := generatePushToken()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "generating push token: " + err.Error()})
		}
		monitor.SourceConfig.PushToken = token
		pushToken = token
	}

	if err := h.monitorRepo.Create(c.Context(), monitor); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "creating monitor: " + err.Error()})
	}

	if pushToken != "" {
		// PushToken has json:"-" on Monitor, so it never round-trips through
		// the normal response. This is the one moment it's shown — the
		// frontend must display and let the user copy it now.
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"monitor": monitor, "pushToken": pushToken})
	}
	return c.Status(fiber.StatusCreated).JSON(monitor)
}

// generatePushToken returns a 64-character hex-encoded random token
// (32 bytes of entropy) for authenticating API push ingestion requests.
func generatePushToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// validateSourceConfig rejects monitor configurations that can never
// receive data — the exact class of bug this feature exists to prevent
// (a monitor that saves successfully but has no working ingestion path).
// Non-api source types have no required SourceConfig fields, so nil/empty
// configs are always valid for them.
func validateSourceConfig(sourceType models.SourceType, cfg *models.SourceConfig) error {
	if sourceType != models.SourceAPI {
		return nil
	}
	if cfg == nil || cfg.Mode == "" {
		return fmt.Errorf("api monitors require sourceConfig.mode (\"push\" or \"pull\")")
	}
	if cfg.Mode != models.APIModePush && cfg.Mode != models.APIModePull {
		return fmt.Errorf("sourceConfig.mode must be \"push\" or \"pull\"")
	}
	if cfg.Mode == models.APIModePull {
		if strings.TrimSpace(cfg.PullURL) == "" {
			return fmt.Errorf("pull mode requires sourceConfig.pullUrl")
		}
		if cfg.PullAuthType == models.APIAuthAPIKey && strings.TrimSpace(cfg.PullAuthHeaderName) == "" {
			return fmt.Errorf("api_key_header auth requires sourceConfig.pullAuthHeaderName")
		}
		if cfg.PullMethod != "" && cfg.PullMethod != fiber.MethodGet && cfg.PullMethod != fiber.MethodPost {
			return fmt.Errorf("sourceConfig.pullMethod must be GET or POST")
		}
	}
	return nil
}

// detectSchemaRequest mirrors the shape of CreateMonitorRequest's relevant
// fields (sourceType + sourceConfig) rather than a bare SourceConfigInput,
// so the frontend can build this call the same way it builds Create's body.
type detectSchemaRequest struct {
	SourceType   models.SourceType         `json:"sourceType"`
	SourceConfig *models.SourceConfigInput `json:"sourceConfig"`
}

// DetectSchema makes a live test request to a not-yet-created api+pull
// monitor's configured endpoint and returns the inferred schema, without
// persisting anything — lets the create-monitor form show the user a
// schema to confirm instead of it being detected blindly on first ingest.
func (h *MonitorHandler) DetectSchema(c *fiber.Ctx) error {
	var req detectSchemaRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.SourceType != "" && req.SourceType != models.SourceAPI {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "la detección de schema solo está disponible para sourceType \"api\""})
	}

	cfg := req.SourceConfig.ToSourceConfig()
	if err := validateSourceConfig(models.SourceAPI, cfg); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	schema, sampleCount, err := h.ingestionService.DetectAPISchema(c.Context(), *cfg)
	if err != nil {
		if errors.Is(err, services.ErrPushModeSchemaDetection) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		// A timed-out request to the external API is distinguished from
		// other network/response failures (DNS, refused, non-2xx, bad JSON)
		// so the frontend can tell "it's slow" apart from "it's broken" —
		// everything else collapses to 502 since this service isn't the
		// origin of the failure either way.
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return c.Status(fiber.StatusGatewayTimeout).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"schema": schema, "sampleCount": sampleCount})
}

func (h *MonitorHandler) List(c *fiber.Ctx) error {
	monitors, err := h.monitorRepo.FindAll(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if monitors == nil {
		monitors = []models.Monitor{}
	}
	return c.JSON(monitors)
}

func (h *MonitorHandler) Get(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}

	monitor, err := h.monitorRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
	}

	return c.JSON(monitor)
}

func (h *MonitorHandler) Update(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}

	var req models.UpdateMonitorRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	update := bson.M{}
	if req.Name != nil {
		update["name"] = *req.Name
	}
	if req.Description != nil {
		update["description"] = *req.Description
	}

	if req.SourceConfig != nil {
		monitor, err := h.monitorRepo.FindByID(c.Context(), id)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
		}

		newConfig := req.SourceConfig.ToSourceConfig()
		if err := validateSourceConfig(monitor.SourceType, newConfig); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		// Server-controlled fields aren't in SourceConfigInput at all, so
		// carry them over from the existing config — otherwise every edit
		// would silently wipe the push token or pull history.
		if monitor.SourceConfig != nil {
			newConfig.PushToken = monitor.SourceConfig.PushToken
			newConfig.NextPullAt = monitor.SourceConfig.NextPullAt
			newConfig.LastPullAt = monitor.SourceConfig.LastPullAt
			newConfig.LastPullStatus = monitor.SourceConfig.LastPullStatus
			newConfig.LastPullError = monitor.SourceConfig.LastPullError

			// PullAuthValue is json:"-" on the response shape (it's a
			// secret, so it's never sent back to the client), which means
			// the edit form can never echo it — an empty value here isn't
			// "the user cleared it", it's "the client never had it to send
			// in the first place". Treat blank as "leave unchanged", same
			// as the AI-config API key field.
			if newConfig.PullAuthValue == "" {
				newConfig.PullAuthValue = monitor.SourceConfig.PullAuthValue
			}
		}
		update["source_config"] = newConfig
	}

	if len(update) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no valid fields to update"})
	}

	if err := h.monitorRepo.Update(c.Context(), id, update); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "monitor updated"})
}

// RotatePushToken issues a new PushToken for an api+push monitor,
// immediately invalidating the old one. Same one-time-visibility rule as
// creation: PushToken has json:"-" on Monitor, so this response is the
// only place the new value is ever shown.
func (h *MonitorHandler) RotatePushToken(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}

	monitor, err := h.monitorRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
	}
	if monitor.SourceType != models.SourceAPI || monitor.SourceConfig == nil || monitor.SourceConfig.Mode != models.APIModePush {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "monitor is not configured for API push"})
	}

	token, err := generatePushToken()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "generating push token: " + err.Error()})
	}

	if err := h.monitorRepo.Update(c.Context(), id, bson.M{"source_config.push_token": token}); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"pushToken": token})
}

func (h *MonitorHandler) Delete(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}

	if err := h.monitorRepo.Delete(c.Context(), id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "monitor deleted"})
}

func (h *MonitorHandler) UploadData(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}

	monitor, err := h.monitorRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "file is required"})
	}

	f, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "opening file"})
	}
	defer func() { _ = f.Close() }()

	// El archivo se lee una sola vez desde el multipart; si el intento se
	// rechaza por estructura, se sube a GridFS desde estos mismos bytes
	// (leídos antes de que Ingest* consuma el reader).
	rawBytes, err := io.ReadAll(f)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "reading file"})
	}
	reReadable := &memFile{Reader: bytes.NewReader(rawBytes)}

	var outcome *services.IngestOutcome
	switch monitor.SourceType {
	case models.SourceCSV:
		outcome, err = h.ingestionService.IngestCSV(c.Context(), monitor, reReadable, false)
	case models.SourceJSON:
		count, jerr := h.ingestionService.IngestJSON(c.Context(), monitor, bytes.NewReader(rawBytes))
		if jerr == nil {
			outcome = &services.IngestOutcome{Ingested: true, TotalRows: count, RowsAccepted: count}
		}
		err = jerr
	case models.SourceExcel:
		outcome, err = h.ingestionService.IngestExcel(c.Context(), monitor, reReadable, false)
	case models.SourceTXT:
		outcome, err = h.ingestionService.IngestTXT(c.Context(), monitor, reReadable, false)
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unsupported source type"})
	}

	if err != nil {
		if errors.Is(err, services.ErrMalformedFile) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	userID, _ := primitive.ObjectIDFromHex(c.Locals("userId").(string))
	userEmail, _ := c.Locals("email").(string)

	entry := &models.UploadLogEntry{
		MonitorID:       id,
		MonitorName:     monitor.Name,
		FileName:        fileHeader.Filename,
		SourceType:      monitor.SourceType,
		UploadedBy:      userID,
		UploadedByEmail: userEmail,
		UploadedAt:      time.Now(),
		TotalRows:       outcome.TotalRows,
	}

	if !outcome.Ingested {
		entry.Status = models.UploadStatusRejectedStructure
		entry.SchemaDiff = &outcome.SchemaDiff
		entry.RowsRejected = outcome.TotalRows
		if saveErr := h.uploadLogRepo.Save(c.Context(), entry, rawBytes); saveErr != nil {
			log.Printf("bitacora: guardando entrada rechazada: %v", saveErr)
		}
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error":      "el archivo no coincide con la estructura del monitor",
			"schemaDiff": outcome.SchemaDiff,
		})
	}

	entry.RowsAccepted = outcome.RowsAccepted
	entry.RowsRejected = len(outcome.RowRejections)
	entry.RowRejections = outcome.RowRejections
	if entry.RowsRejected > 0 {
		entry.Status = models.UploadStatusPartial
	} else {
		entry.Status = models.UploadStatusAccepted
	}
	if saveErr := h.uploadLogRepo.Save(c.Context(), entry, nil); saveErr != nil {
		log.Printf("bitacora: guardando entrada aceptada: %v", saveErr)
	}

	// Enqueue rule evaluation (async via worker)
	queued := false
	if h.jobQueue != nil {
		qerr := h.jobQueue.Enqueue(c.Context(), services.EvalJob{MonitorID: id.Hex()})
		queued = qerr == nil
	}

	return c.JSON(fiber.Map{
		"recordsIngested":  outcome.RowsAccepted,
		"rowsRejected":     entry.RowsRejected,
		"schema":           monitor.Schema,
		"evaluationQueued": queued,
	})
}

// UploadCheck corre la misma validación que UploadData (comparación de
// estructura + validación por fila) SIN escribir nada en Mongo ni en la
// bitácora — un "dry run" para que el frontend alerte al usuario antes
// de comprometer la subida real.
func (h *MonitorHandler) UploadCheck(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}

	monitor, err := h.monitorRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "file is required"})
	}

	f, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "opening file"})
	}
	defer func() { _ = f.Close() }()

	rawBytes, err := io.ReadAll(f)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "reading file"})
	}

	var outcome *services.IngestOutcome
	switch monitor.SourceType {
	case models.SourceCSV:
		outcome, err = h.ingestionService.IngestCSV(c.Context(), monitor, &memFile{Reader: bytes.NewReader(rawBytes)}, true)
	case models.SourceExcel:
		outcome, err = h.ingestionService.IngestExcel(c.Context(), monitor, &memFile{Reader: bytes.NewReader(rawBytes)}, true)
	case models.SourceTXT:
		outcome, err = h.ingestionService.IngestTXT(c.Context(), monitor, &memFile{Reader: bytes.NewReader(rawBytes)}, true)
	default:
		// JSON no tiene validación por fila en esta iteración (ver spec)
		// ni necesita dry-run — se sube directo.
		return c.JSON(fiber.Map{"match": true, "totalRows": 0, "rowsThatWouldPass": 0, "rowsThatWouldFail": 0})
	}

	if err != nil {
		if errors.Is(err, services.ErrMalformedFile) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"match":             outcome.SchemaDiff.Match,
		"missingFields":     outcome.SchemaDiff.MissingFields,
		"extraFields":       outcome.SchemaDiff.ExtraFields,
		"typeMismatches":    outcome.SchemaDiff.TypeMismatches,
		"totalRows":         outcome.TotalRows,
		"rowsThatWouldPass": outcome.RowsAccepted,
		"rowsThatWouldFail": len(outcome.RowRejections),
	})
}

// memFile adapta un *bytes.Reader a multipart.File (io.Reader +
// io.ReaderAt + io.Seeker + io.Closer) para poder re-parsear el mismo
// archivo dos veces (una para detectar, otra si Ingest* lo necesita) sin
// volver a leer del stream multipart original.
type memFile struct {
	*bytes.Reader
}

func (memFile) Close() error { return nil }

func (h *MonitorHandler) IngestionHistory(c *fiber.Ctx) error {
	entries, err := h.monitorRepo.GetIngestionHistory(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(entries)
}

// UploadLogList devuelve la bitácora completa, opcionalmente filtrada
// por monitor vía ?monitorId=. Mismo criterio de acceso que
// IngestionHistory (cualquier usuario autenticado puede ver — no hay
// restricción de rol para lectura, solo para aprobar).
func (h *MonitorHandler) UploadLogList(c *fiber.Ctx) error {
	var monitorID *primitive.ObjectID
	if q := c.Query("monitorId"); q != "" {
		id, err := primitive.ObjectIDFromHex(q)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitorId"})
		}
		monitorID = &id
	}

	entries, err := h.uploadLogRepo.List(c.Context(), monitorID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(entries)
}

// UploadLogApprove re-ingiere el archivo guardado de una entrada
// rejected_structure, tratando su estructura como la nueva base del
// monitor. Se limpia monitor.Schema antes de re-ingerir: compareSchema
// trata un schema vacío como "primer upload" (siempre match), así el
// mismo camino de Ingest* que ya existe detecta y fija la nueva
// estructura sin necesitar un parámetro "forzar" aparte.
func (h *MonitorHandler) UploadLogApprove(c *fiber.Ctx) error {
	monitorID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}
	logID, err := primitive.ObjectIDFromHex(c.Params("logId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid log ID"})
	}

	entry, err := h.uploadLogRepo.Get(c.Context(), logID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "upload log entry not found"})
	}
	if entry.Status != models.UploadStatusRejectedStructure {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "solo se pueden aprobar entradas rechazadas por estructura"})
	}
	if entry.MonitorID != monitorID {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "la entrada de bitácora no pertenece a este monitor"})
	}

	monitor, err := h.monitorRepo.FindByID(c.Context(), monitorID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
	}

	fileBytes, err := h.uploadLogRepo.DownloadRejectedFile(c.Context(), entry)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}

	monitor.Schema = nil // fuerza a compareSchema a tratar esto como primer upload

	var outcome *services.IngestOutcome
	switch entry.SourceType {
	case models.SourceCSV:
		outcome, err = h.ingestionService.IngestCSV(c.Context(), monitor, &memFile{Reader: bytes.NewReader(fileBytes)}, false)
	case models.SourceExcel:
		outcome, err = h.ingestionService.IngestExcel(c.Context(), monitor, &memFile{Reader: bytes.NewReader(fileBytes)}, false)
	case models.SourceTXT:
		outcome, err = h.ingestionService.IngestTXT(c.Context(), monitor, &memFile{Reader: bytes.NewReader(fileBytes)}, false)
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "tipo de fuente no soportado para aprobación"})
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if approveErr := h.uploadLogRepo.MarkApproved(c.Context(), logID, outcome.TotalRows, outcome.RowsAccepted, len(outcome.RowRejections), outcome.RowRejections); approveErr != nil {
		log.Printf("bitacora: marcando entrada aprobada: %v", approveErr)
	}

	queued := false
	if h.jobQueue != nil {
		qerr := h.jobQueue.Enqueue(c.Context(), services.EvalJob{MonitorID: monitorID.Hex()})
		queued = qerr == nil
	}

	return c.JSON(fiber.Map{
		"recordsIngested":  outcome.RowsAccepted,
		"rowsRejected":     len(outcome.RowRejections),
		"schema":           monitor.Schema,
		"evaluationQueued": queued,
	})
}

func (h *MonitorHandler) Evaluate(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}

	monitor, err := h.monitorRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
	}

	if monitor.RecordCount == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "monitor has no data to evaluate"})
	}

	redFlags, err := h.ruleEngine.EvaluateRules(c.Context(), monitor)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "rule evaluation failed: " + err.Error()})
	}

	return c.JSON(fiber.Map{
		"redFlagsGenerated": len(redFlags),
		"redFlags":          redFlags,
	})
}

func (h *MonitorHandler) GetData(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}

	monitor, err := h.monitorRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
	}

	data, err := h.monitorRepo.QueryData(c.Context(), monitor.CollectionID, nil, 100)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"data":   data,
		"total":  monitor.RecordCount,
		"schema": monitor.Schema,
	})
}

// IngestPush receives data pushed by an external system for an API+push
// monitor. Public route (no JWT) — authenticated by a per-monitor secret
// token instead, since an external system has no user session.
func (h *MonitorHandler) IngestPush(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("monitorId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}

	monitor, err := h.monitorRepo.FindByID(c.Context(), id)
	if err != nil || monitor.SourceType != models.SourceAPI || monitor.SourceConfig == nil || monitor.SourceConfig.Mode != models.APIModePush {
		// Same response whether the monitor doesn't exist or isn't
		// configured for API push: an unauthenticated caller must not be
		// able to distinguish "no such monitor" from "wrong kind of
		// monitor" — that would be an existence/config oracle probeable
		// without any credentials.
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
	}

	token := c.Get("X-Ingest-Token")
	if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(monitor.SourceConfig.PushToken)) != 1 {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or missing ingest token"})
	}

	count, err := h.ingestionService.IngestJSON(c.Context(), monitor, bytes.NewReader(c.Body()))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	queued := false
	if h.jobQueue != nil {
		err = h.jobQueue.Enqueue(c.Context(), services.EvalJob{MonitorID: id.Hex()})
		queued = err == nil
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"recordsIngested":  count,
		"evaluationQueued": queued,
	})
}

// UpdateSchema reemplaza el schema del monitor, permitiendo configurar
// por campo (hoy ImpliedDecimals y DateFormat) sin cambiar el conjunto
// de campos — el schema recibido debe tener exactamente los mismos
// nombres que el actual, o se rechaza. Esto evita que un bug de
// frontend agregue/quite campos o cambie Name/Type por esta vía.
func (h *MonitorHandler) UpdateSchema(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}

	monitor, err := h.monitorRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
	}

	var body struct {
		Schema []models.SchemaField `json:"schema"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	currentNames := make(map[string]bool, len(monitor.Schema))
	for _, f := range monitor.Schema {
		currentNames[f.Name] = true
	}
	bodyNames := make(map[string]bool, len(body.Schema))
	for _, f := range body.Schema {
		bodyNames[f.Name] = true
	}
	if len(bodyNames) != len(currentNames) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "el schema enviado debe tener los mismos campos que el actual"})
	}
	for name := range currentNames {
		if !bodyNames[name] {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "el schema enviado debe tener los mismos campos que el actual"})
		}
	}

	for _, f := range body.Schema {
		if !services.IsValidDateFormatPreset(f.DateFormat) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("formato de fecha inválido: %s", f.DateFormat)})
		}
	}

	monitor.Schema = body.Schema
	if err := h.monitorRepo.Update(c.Context(), monitor.ID, bson.M{"schema": monitor.Schema}); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"schema": monitor.Schema})
}

// UpdateDerivedTimestamp configura (o limpia, mandando null) el timestamp
// derivado del monitor. Al guardarlo, el campo derivado se agrega al schema
// como tipo date para que aparezca en los selectores de campo de reglas,
// chat y dashboards sin que ninguno tenga que saber que es derivado.
func (h *MonitorHandler) UpdateDerivedTimestamp(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}

	monitor, err := h.monitorRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
	}

	// json.RawMessage preserva la diferencia entre "la clave no vino" (nil,
	// slice vacío) y "la clave vino con valor null" (slice "null"): un
	// *models.DerivedTimestampConfig directo en BodyParser no puede
	// distinguirlas, y confundirlas es un borrado silencioso de la
	// configuración del usuario ante un body incompleto o malformado.
	var body struct {
		DerivedTimestamp json.RawMessage `json:"derivedTimestamp"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if len(body.DerivedTimestamp) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "falta el campo derivedTimestamp (mandá null explícito para limpiar la configuración)",
		})
	}

	var cfg *models.DerivedTimestampConfig
	if err := json.Unmarshal(body.DerivedTimestamp, &cfg); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid derivedTimestamp"})
	}

	update := bson.M{}
	if cfg == nil {
		// Limpiar la configuración: se quita también el campo del schema.
		schema := make([]models.SchemaField, 0, len(monitor.Schema))
		for _, f := range monitor.Schema {
			if monitor.DerivedTimestamp != nil && f.Name == monitor.DerivedTimestamp.TargetName {
				continue
			}
			schema = append(schema, f)
		}
		update["derived_timestamp"] = nil
		update["schema"] = schema
	} else {
		// El schema contra el que se valida excluye el campo derivado
		// anterior: de lo contrario reconfigurar con el mismo TargetName
		// chocaría consigo mismo.
		base := make([]models.SchemaField, 0, len(monitor.Schema))
		for _, f := range monitor.Schema {
			if monitor.DerivedTimestamp != nil && f.Name == monitor.DerivedTimestamp.TargetName {
				continue
			}
			base = append(base, f)
		}
		if err := services.ValidateDerivedTimestamp(*cfg, base); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		update["derived_timestamp"] = cfg
		update["schema"] = append(base, models.SchemaField{
			Name:     cfg.TargetName,
			Type:     models.FieldDate,
			Required: false,
			Sample:   "",
		})
	}

	if err := h.monitorRepo.Update(c.Context(), monitor.ID, update); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	updated, err := h.monitorRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(updated)
}

// BackfillDerivedTimestamp calcula el timestamp derivado para los documentos
// ya ingeridos. Es idempotente: los documentos que ya lo tienen se saltean,
// así que correrlo dos veces no cambia nada la segunda vez.
func (h *MonitorHandler) BackfillDerivedTimestamp(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}

	monitor, err := h.monitorRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
	}
	if monitor.DerivedTimestamp == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "el monitor no tiene timestamp derivado configurado"})
	}

	cfg := *monitor.DerivedTimestamp
	updated, skipped := 0, 0
	err = h.monitorRepo.IterateData(c.Context(), monitor.CollectionID, func(doc bson.M) error {
		if _, ya := doc[cfg.TargetName]; ya {
			return nil
		}
		ts, ok := services.BuildDerivedTimestamp(cfg, doc)
		if !ok {
			skipped++
			return nil
		}
		if err := h.monitorRepo.SetDataField(c.Context(), monitor.CollectionID, doc["_id"], cfg.TargetName, ts); err != nil {
			return err
		}
		updated++
		return nil
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"updated": updated, "skipped": skipped})
}
