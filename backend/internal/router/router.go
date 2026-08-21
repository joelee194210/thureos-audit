package router

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/thureos/compliance/internal/config"
	"github.com/thureos/compliance/internal/handlers"
	"github.com/thureos/compliance/internal/middleware"
	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
)

type Handlers struct {
	Auth       *handlers.AuthHandler
	Monitor    *handlers.MonitorHandler
	Rule       *handlers.RuleHandler
	Dashboard  *handlers.DashboardHandler
	RedFlag    *handlers.RedFlagHandler
	User       *handlers.UserHandler
	MCC        *handlers.MCCHandler
	Country    *handlers.CountryHandler
	Activity   *handlers.ActivityHandler
	Scheduler  interface{ Status() map[string]interface{} }
	ConfigRepo *repository.SystemConfigRepository
}

func Setup(app *fiber.App, cfg *config.Config, h *Handlers) {
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "${time} | ${status} | ${latency} | ${method} ${path}\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: cfg.AllowedOrigins,
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, PUT, PATCH, DELETE, OPTIONS",
	}))

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	api := app.Group("/api/v1")

	// Public routes
	// Endpoints públicos: limitados por IP para que la fuerza bruta contra
	// credenciales y la creación masiva de cuentas tengan coste.
	auth := api.Group("/auth")
	auth.Post("/register",
		middleware.RegisterRateLimiter(),
		h.Auth.Register)
	auth.Post("/login",
		middleware.LoginRateLimiter(),
		h.Auth.Login)

	// Public ingest endpoint for API+push monitors — authenticated by a
	// per-monitor secret token (X-Ingest-Token header), not JWT. An
	// external system pushing data has no user session.
	api.Post("/ingest/:monitorId",
		middleware.IngestRateLimiter(),
		middleware.IngestBodySizeLimit(10*1024*1024),
		h.Monitor.IngestPush)

	// Protected routes
	protected := api.Group("", middleware.AuthRequired(cfg))

	protected.Get("/auth/me", h.Auth.Me)

	// Monitors
	monitors := protected.Group("/monitors")
	monitors.Get("/", h.Monitor.List)
	monitors.Get("/:id", h.Monitor.Get)
	monitors.Post("/", middleware.RequireComplianceOrAbove(), h.Monitor.Create)
	monitors.Put("/:id", middleware.RequireComplianceOrAbove(), h.Monitor.Update)
	monitors.Delete("/:id", middleware.RequireComplianceOrAbove(), h.Monitor.Delete)
	monitors.Post("/:id/upload", middleware.RequireComplianceOrAbove(), h.Monitor.UploadData)
	monitors.Post("/:id/evaluate", middleware.RequireComplianceOrAbove(), h.Monitor.Evaluate)
	monitors.Get("/:id/data", h.Monitor.GetData)

	// Ingestion history
	protected.Get("/ingestion-history", h.Monitor.IngestionHistory)

	// Rules
	rules := protected.Group("/rules")
	rules.Get("/", h.Rule.List)
	rules.Get("/:id", h.Rule.Get)
	rules.Post("/", middleware.RequireComplianceOrAbove(), h.Rule.Create)
	rules.Put("/:id", middleware.RequireComplianceOrAbove(), h.Rule.Update)
	rules.Delete("/:id", middleware.RequireComplianceOrAbove(), h.Rule.Delete)
	rules.Post("/:id/execute", middleware.RequireComplianceOrAbove(), h.Rule.Execute)
	rules.Post("/ai-generate", middleware.RequireComplianceOrAbove(), h.Rule.GenerateAIRules)

	// Dashboards
	dashboards := protected.Group("/dashboards")
	dashboards.Get("/", h.Dashboard.List)
	dashboards.Get("/:id", h.Dashboard.Get)
	dashboards.Get("/:id/data", h.Dashboard.GetData)
	dashboards.Post("/", middleware.RequireComplianceOrAbove(), h.Dashboard.Create)
	dashboards.Post("/:id/widgets", middleware.RequireComplianceOrAbove(), h.Dashboard.AddWidget)
	dashboards.Post("/:id/widgets/:widgetId/drilldown", h.Dashboard.DrillDown)
	dashboards.Delete("/:id/widgets/:widgetId", middleware.RequireComplianceOrAbove(), h.Dashboard.DeleteWidget)
	dashboards.Delete("/:id", middleware.RequireComplianceOrAbove(), h.Dashboard.Delete)

	// Red flags
	redFlags := protected.Group("/red-flags")
	redFlags.Get("/", h.RedFlag.List)
	redFlags.Get("/stats", h.RedFlag.Stats)
	redFlags.Get("/calendar", h.RedFlag.Calendar)
	redFlags.Get("/date/:date", h.RedFlag.ByDate)
	redFlags.Get("/:id", h.RedFlag.Get)
	redFlags.Get("/:id/records", h.RedFlag.GetRecords)
	redFlags.Get("/:id/logs", h.RedFlag.GetLogs)
	redFlags.Patch("/:id/status", middleware.RequireComplianceOrAbove(), h.RedFlag.UpdateStatus)

	// MCCs (catalog)
	mccs := protected.Group("/mccs")
	mccs.Get("/", h.MCC.List)
	mccs.Get("/search", h.MCC.Search)
	mccs.Get("/categories", h.MCC.Categories)
	mccs.Get("/category/:category", h.MCC.ByCategory)
	mccs.Get("/risk/:level", h.MCC.ByRiskLevel)
	mccs.Put("/:id", middleware.RequireComplianceOrAbove(), h.MCC.Update)

	// Countries (risk catalog)
	countries := protected.Group("/countries")
	countries.Get("/", h.Country.List)
	countries.Get("/active", h.Country.Active)
	countries.Get("/search", h.Country.Search)
	countries.Get("/regions", h.Country.Regions)
	countries.Get("/risk/:level", h.Country.ByRiskLevel)
	countries.Get("/region/:region", h.Country.ByRegion)
	countries.Get("/:id", h.Country.Get)
	countries.Post("/", middleware.RequireComplianceOrAbove(), h.Country.Create)
	countries.Put("/:id", middleware.RequireComplianceOrAbove(), h.Country.Update)
	countries.Delete("/:id", middleware.RequireRole(models.RoleAdmin), h.Country.Delete)

	// Activity logs (admin only)
	protected.Get("/activity-logs", middleware.RequireRole(models.RoleAdmin), h.Activity.List)

	// Users (admin only)
	users := protected.Group("/users", middleware.RequireRole(models.RoleAdmin))
	users.Get("/", h.User.List)
	users.Get("/:id", h.User.Get)
	users.Patch("/:id/role", h.User.UpdateRole)
	users.Patch("/:id/toggle-active", h.User.ToggleActive)
	users.Delete("/:id", h.User.Delete)

	// Settings (admin only)
	settings := protected.Group("/settings", middleware.RequireRole(models.RoleAdmin))

	settings.Get("/", func(c *fiber.Ctx) error {
		aiCfg, _ := h.ConfigRepo.Get(c.Context())
		hasAI := aiCfg != nil && aiCfg.AI.APIKey != ""
		aiProvider := "anthropic"
		aiModel := "claude-sonnet-4-20250514"
		if aiCfg != nil {
			aiProvider = string(aiCfg.AI.Provider)
			if aiCfg.AI.Model != "" {
				aiModel = aiCfg.AI.Model
			}
		}

		schedulerStatus := map[string]interface{}{"running": false, "rulesActive": 0}
		if h.Scheduler != nil {
			schedulerStatus = h.Scheduler.Status()
		}
		return c.JSON(fiber.Map{
			"port":           cfg.Port,
			"mongoConnected": true,
			"redisConnected": true,
			"aiEnabled":      hasAI,
			"aiProvider":     aiProvider,
			"aiModel":        aiModel,
			"allowedOrigins": cfg.AllowedOrigins,
			"workers":        2,
			"maxRetries":     3,
			"scheduler":      schedulerStatus,
		})
	})

	// AI config — GET returns masked key, PUT updates provider/model/key
	settings.Get("/ai", func(c *fiber.Ctx) error {
		aiCfg, err := h.ConfigRepo.Get(c.Context())
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load AI config"})
		}
		return c.JSON(fiber.Map{
			"provider":        string(aiCfg.AI.Provider),
			"model":           aiCfg.AI.Model,
			"apiKeyMasked":    models.MaskAPIKey(aiCfg.AI.APIKey),
			"apiKeySet":       aiCfg.AI.APIKey != "",
			"baseUrl":         aiCfg.AI.BaseURL,
			"availableModels": models.AIProviderModels,
		})
	})

	settings.Put("/ai", func(c *fiber.Ctx) error {
		var body struct {
			Provider string `json:"provider"`
			Model    string `json:"model"`
			APIKey   string `json:"apiKey"`
			BaseURL  string `json:"baseUrl"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}

		provider := models.AIProvider(body.Provider)
		if provider != models.AIProviderAnthropic && provider != models.AIProviderDeepSeek {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid provider, must be 'anthropic' or 'deepseek'"})
		}

		// Validate model belongs to provider
		validModels := models.AIProviderModels[provider]
		modelValid := false
		for _, m := range validModels {
			if m == body.Model {
				modelValid = true
				break
			}
		}
		if !modelValid {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid model for provider"})
		}

		// Get current config to preserve API key if not provided
		current, err := h.ConfigRepo.Get(c.Context())
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load current config"})
		}

		apiKey := body.APIKey
		if apiKey == "" {
			apiKey = current.AI.APIKey // keep existing key
		}

		userEmail, _ := c.Locals("email").(string)

		updated := &models.SystemConfig{
			AI: models.AIConfig{
				Provider: provider,
				Model:    body.Model,
				APIKey:   apiKey,
				BaseURL:  body.BaseURL,
			},
			UpdatedBy: userEmail,
		}

		if err := h.ConfigRepo.Upsert(c.Context(), updated); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save AI config"})
		}

		return c.JSON(fiber.Map{"message": "AI configuration updated"})
	})
}
