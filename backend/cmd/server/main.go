package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/thureos/compliance/internal/config"
	"github.com/thureos/compliance/internal/database"
	"github.com/thureos/compliance/internal/handlers"
	"github.com/thureos/compliance/internal/repository"
	"github.com/thureos/compliance/internal/router"
	"github.com/thureos/compliance/internal/seed"
	"github.com/thureos/compliance/internal/services"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	cfg := config.Load()

	// Connect to MongoDB
	mongo, err := database.ConnectMongo(cfg.MongoURI)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer mongo.Disconnect()

	// Connect to Redis
	redisClient, err := database.ConnectRedis(cfg.RedisURL)
	if err != nil {
		log.Printf("Warning: Redis connection failed: %v (workers disabled)", err)
	}

	// Initialize repositories
	userRepo := repository.NewUserRepository(mongo)
	monitorRepo := repository.NewMonitorRepository(mongo)
	ruleRepo := repository.NewRuleRepository(mongo)
	redFlagRepo := repository.NewRedFlagRepository(mongo)
	redFlagLogRepo := repository.NewRedFlagLogRepository(mongo)
	dashboardRepo := repository.NewDashboardRepository(mongo)
	mccRepo := repository.NewMCCRepository(mongo)
	countryRepo := repository.NewCountryRepository(mongo)
	activityLogRepo := repository.NewActivityLogRepository(mongo)
	execLogRepo := repository.NewRuleExecutionLogRepository(mongo)
	systemConfigRepo := repository.NewSystemConfigRepository(mongo)

	// Seed catalogs
	if err := mccRepo.Seed(context.Background(), seed.MCCData); err != nil {
		log.Printf("Warning: MCC seed failed: %v", err)
	}
	if err := countryRepo.Seed(context.Background(), seed.CountryData); err != nil {
		log.Printf("Warning: Country seed failed: %v", err)
	}

	// Seed default AI config (uses env ANTHROPIC_API_KEY if set)
	if err := systemConfigRepo.SeedDefault(context.Background(), cfg.AnthropicKey); err != nil {
		log.Printf("Warning: system config seed failed: %v", err)
	}

	// Seed admin user (uses env ADMIN_EMAIL + ADMIN_PASSWORD)
	if cfg.AdminEmail != "" && cfg.AdminPassword != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("Warning: could not hash admin password: %v", err)
		} else if err := userRepo.SeedAdmin(context.Background(), cfg.AdminEmail, string(hashed), "Admin"); err != nil {
			log.Printf("Warning: admin seed failed: %v", err)
		} else {
			log.Printf("Admin user seed OK (email=%s)", cfg.AdminEmail)
		}
	}

	// Migrate old "analyst" role to "compliance"
	if migrated, err := userRepo.MigrateRole(context.Background(), "analyst", "compliance"); err != nil {
		log.Printf("Warning: role migration failed: %v", err)
	} else if migrated > 0 {
		log.Printf("Migrated %d users from 'analyst' to 'compliance' role", migrated)
	}

	// Initialize services
	authService := services.NewAuthService(userRepo, cfg)
	ingestionService := services.NewIngestionService(monitorRepo)
	ruleEngine := services.NewRuleEngine(ruleRepo, redFlagRepo, monitorRepo, execLogRepo)
	aiRulesService := services.NewAIRulesService(systemConfigRepo)
	dashboardService := services.NewDashboardService(dashboardRepo, monitorRepo, ruleRepo)

	// Initialize job queue and workers
	var jobQueue *services.JobQueue
	if redisClient != nil {
		jobQueue = services.NewJobQueue(redisClient)
		worker := services.NewWorker(jobQueue, ruleEngine, monitorRepo, 2)

		workerCtx, workerCancel := context.WithCancel(context.Background())
		defer workerCancel()
		go worker.Start(workerCtx)
	}

	// Clean up duplicate red flags from before deduplication was added
	if deleted, err := redFlagRepo.DeleteDuplicates(context.Background()); err != nil {
		log.Printf("Warning: duplicate cleanup failed: %v", err)
	} else if deleted > 0 {
		log.Printf("Cleaned up %d duplicate red flags", deleted)
	}

	// Initialize scheduler
	scheduler := services.NewScheduler(ruleRepo, monitorRepo, ruleEngine)
	schedulerCtx, schedulerCancel := context.WithCancel(context.Background())
	defer schedulerCancel()
	go scheduler.Start(schedulerCtx)

	// Initialize monitor puller (API pull-mode monitors)
	monitorPuller := services.NewMonitorPuller(monitorRepo, ingestionService, jobQueue)
	pullerCtx, pullerCancel := context.WithCancel(context.Background())
	defer pullerCancel()
	go monitorPuller.Start(pullerCtx)

	// Initialize handlers
	h := &router.Handlers{
		Auth:          handlers.NewAuthHandler(authService, activityLogRepo),
		Monitor:       handlers.NewMonitorHandler(monitorRepo, ingestionService, jobQueue, ruleEngine),
		Rule:          handlers.NewRuleHandler(ruleRepo, monitorRepo, aiRulesService, ruleEngine),
		Dashboard:     handlers.NewDashboardHandler(dashboardRepo, dashboardService),
		RedFlag:       handlers.NewRedFlagHandler(redFlagRepo, redFlagLogRepo, ruleRepo, monitorRepo, userRepo, activityLogRepo),
		User:          handlers.NewUserHandler(userRepo, activityLogRepo),
		MCC:           handlers.NewMCCHandler(mccRepo, activityLogRepo),
		Country:       handlers.NewCountryHandler(countryRepo),
		Activity:      handlers.NewActivityHandler(activityLogRepo),
		Scheduler:     scheduler,
		MonitorPuller: monitorPuller,
		ConfigRepo:    systemConfigRepo,
	}

	// Create Fiber app
	app := fiber.New(fiber.Config{
		BodyLimit: 50 * 1024 * 1024, // 50MB for file uploads
	})

	router.Setup(app, cfg, h)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down server...")
		app.Shutdown()
	}()

	log.Printf("Thureos Compliance API server starting on :%s", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
