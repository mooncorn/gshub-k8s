package main

import (
	"context"
	"log"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"github.com/mooncorn/gshub/api/config"
	"github.com/mooncorn/gshub/api/internal/api"
	"github.com/mooncorn/gshub/api/internal/database"
	"github.com/mooncorn/gshub/api/internal/services/broadcast"
	"github.com/mooncorn/gshub/api/internal/services/cleanup"
	"github.com/mooncorn/gshub/api/internal/services/k8s"
	"github.com/mooncorn/gshub/api/internal/services/nodesync"
	"github.com/mooncorn/gshub/api/internal/services/portalloc"
	"github.com/mooncorn/gshub/api/internal/services/reconciler"
	"github.com/mooncorn/gshub/api/internal/services/watcher"
	"go.uber.org/zap"
)

func main() {
	// Load .env file (ignore error in production)
	// TODO: Move to config and log this ^
	_ = godotenv.Load()

	// Register custom validators
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterValidation("dns", validateDNS)
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// Connect to database
	database, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer database.Close()

	log.Println("Connected to database successfully")

	// Run database migrations
	ctx := context.Background()
	if err := database.Migrate(ctx, cfg.MigrationsDir); err != nil {
		log.Fatal("Failed to run migrations:", err)
	}

	// Initialize Kubernetes client
	k8sClient, err := k8s.NewClient()
	if err != nil {
		log.Fatal("Failed to initialize K8s client:", err)
	}

	log.Println("Kubernetes client initialized successfully")

	// Test K8s connectivity
	if err := k8sClient.Health(ctx); err != nil {
		log.Fatal("K8s health check failed:", err)
	}

	log.Println("Connected to Kubernetes API successfully")

	// Initialize logger for services
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatal("Failed to create logger:", err)
	}
	defer logger.Sync()

	// Initialize port allocation service
	portAllocService := portalloc.NewService(database, logger)
	log.Println("Port allocation service initialized")

	// Initialize broadcast hub for real-time SSE updates
	hub := broadcast.NewHub(logger)
	log.Println("Broadcast hub initialized")

	// Initialize and start GameServer watcher for real-time K8s state updates
	watcherService := watcher.NewService(database, k8sClient.AgonesClientset(), hub, logger, cfg.K8sNamespace)
	watcherService.Start(ctx)
	defer watcherService.Stop()
	log.Println("GameServer watcher started")

	// Initialize and start node sync service
	nodeSyncConfig := nodesync.Config{
		PortRangeMin:  cfg.PortRangeMin,
		PortRangeMax:  cfg.PortRangeMax,
		SyncInterval:  nodesync.DefaultConfig().SyncInterval,
		NodeRoleLabel: nodesync.DefaultConfig().NodeRoleLabel,
		PublicIPLabel: nodesync.DefaultConfig().PublicIPLabel,
	}
	nodeSyncService := nodesync.NewService(database, k8sClient, nodeSyncConfig, logger)
	nodeSyncService.Start(ctx)
	defer nodeSyncService.Stop()
	log.Println("Node sync service started")

	// Initialize and start the server reconciler
	serverReconciler := reconciler.NewServerReconciler(database, k8sClient, portAllocService, logger, cfg.K8sNamespace, cfg.K8sGameCatalogName)
	serverReconciler.Start(ctx)
	defer serverReconciler.Stop()

	log.Println("Server reconciler started")

	// Initialize and start the cleanup service
	cleanupConfig := cleanup.Config{
		Interval:  cleanup.DefaultConfig().Interval,
		Namespace: cfg.K8sNamespace,
	}
	cleanupService := cleanup.NewService(database, k8sClient, cleanupConfig, logger)
	cleanupService.Start(ctx)
	defer cleanupService.Stop()

	log.Println("Cleanup service started")

	handlers := api.NewHandlers(database, cfg, k8sClient, portAllocService, hub)
	r := gin.Default()
	handlers.RegisterRoutes(r)

	// Start server
	log.Printf("Starting server on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

// validateDNS validates that a string is a valid DNS subdomain
// DNS subdomains can contain letters, numbers, and hyphens
// They cannot start or end with a hyphen
func validateDNS(fl validator.FieldLevel) bool {
	dns := fl.Field().String()
	// Valid DNS subdomain pattern: alphanumeric and hyphens, cannot start or end with hyphen
	pattern := `^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`
	regex := regexp.MustCompile(pattern)
	return regex.MatchString(dns)
}
