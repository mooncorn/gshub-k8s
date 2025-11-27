package main

import (
	"context"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/mooncorn/gshub/api/config"
	"github.com/mooncorn/gshub/api/internal/api"
	"github.com/mooncorn/gshub/api/internal/database"
	"github.com/mooncorn/gshub/api/internal/services/k8s"
)

func main() {
	// Load .env file (ignore error in production)
	// TODO: Move to config and log this ^
	_ = godotenv.Load()

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

	// Initialize Kubernetes client
	ctx := context.Background()
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

	handlers := api.NewHandlers(database, cfg, k8sClient)
	r := gin.Default()
	handlers.RegisterRoutes(r)

	// Start server
	log.Printf("Starting server on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
