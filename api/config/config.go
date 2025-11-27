package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	// Environment
	Environment string

	// Server
	Port           string
	GinMode        string
	AllowedOrigins []string

	// Database
	DatabaseURL string

	// JWT
	JWTSecret        string
	JWTAccessExpiry  time.Duration
	JWTRefreshExpiry time.Duration

	// MailerSend
	MailerSendAPIKey    string
	MailerSendFromEmail string
	MailerSendFromName  string

	// Stripe
	StripeSecretKey         string
	StripeWebhookSecret     string
	StripeHobbyPriceID      string
	StripeProPriceID        string
	StripeEnterprisePriceID string

	FrontendURL string
}

func Load() (*Config, error) {
	// Build DATABASE_URL from components
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "gshub")
	dbPassword := getEnv("DB_PASSWORD", "")
	dbName := getEnv("DB_NAME", "gshub")
	dbSSLMode := getEnv("DB_SSLMODE", "disable")

	databaseURL := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		dbUser, dbPassword, dbHost, dbPort, dbName, dbSSLMode,
	)

	cfg := &Config{
		Environment: getEnv("ENVIRONMENT", "development"),

		Port:           getEnv("PORT", "8080"),
		GinMode:        getEnv("GIN_MODE", "debug"),
		AllowedOrigins: getEnvSlice("ALLOWED_ORIGINS", []string{"http://localhost:3000"}),

		DatabaseURL: databaseURL,

		JWTSecret:        getEnv("JWT_SECRET", "your-super-secret-jwt-key"),
		JWTAccessExpiry:  parseDuration(getEnv("JWT_ACCESS_EXPIRY", "15m"), 15*time.Minute),
		JWTRefreshExpiry: parseDuration(getEnv("JWT_REFRESH_EXPIRY", "168h"), 168*time.Hour),

		MailerSendAPIKey:    getEnv("MAILERSEND_API_KEY", ""),
		MailerSendFromEmail: getEnv("MAILERSEND_FROM_EMAIL", "noreply@gshub.pro"),
		MailerSendFromName:  getEnv("MAILERSEND_FROM_NAME", "GSHUB.PRO"),

		StripeSecretKey:         getEnv("STRIPE_SECRET_KEY", ""),
		StripeWebhookSecret:     getEnv("STRIPE_WEBHOOK_SECRET", ""),
		StripeHobbyPriceID:      getEnv("STRIPE_HOBBY_PRICE_ID", ""),
		StripeProPriceID:        getEnv("STRIPE_PRO_PRICE_ID", ""),
		StripeEnterprisePriceID: getEnv("STRIPE_ENTERPRISE_PRICE_ID", ""),

		FrontendURL: getEnv("FRONTEND_URL", "http://localhost:3000"),
	}

	// Validate required fields
	if dbPassword == "" {
		return nil, fmt.Errorf("DB_PASSWORD is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}

func parseDuration(value string, defaultValue time.Duration) time.Duration {
	duration, err := time.ParseDuration(value)
	if err != nil {
		return defaultValue
	}
	return duration
}
