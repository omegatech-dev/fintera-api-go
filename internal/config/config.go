package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all application configuration
type Config struct {
	// Server
	Port        string
	Environment string

	// Database
	DatabaseURL string

	// JWT
	JWTSecret          string
	JWTExpirationHours int

	// Storage
	StoragePath string

	// IdentityDocumentsRequired enforces that a contract is created only when
	// the applicant has a DNI and an RTN on file. It is off during the
	// rollout, while the two frontends are still being deployed.
	IdentityDocumentsRequired bool

	// Request limits
	// MaxJSONBodyBytes caps the JSON request bodies accepted by the API.
	// Multipart uploads are exempt; they enforce their own per-endpoint size
	// limits in the handlers.
	MaxJSONBodyBytes int64

	// Background Workers
	WorkerCount int

	// CORS
	AllowedOrigins []string

	// General
	AppURL    string
	PortalURL string

	// Email (Resend)
	ResendAPIKey             string
	FromEmail                string
	EmailOverrideTo          string
	EnableEmailNotifications bool

	// Rollbar
	RollbarToken       string
	RollbarEnvironment string
	RollbarCodeVersion string
	RollbarServerRoot  string

	// Default Login (Development only)
	DefaultEmail    string
	DefaultPassword string
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	environment := getEnv("ENVIRONMENT", "development")

	cfg := &Config{
		Port:                      getEnv("PORT", "8080"),
		Environment:               environment,
		DatabaseURL:               getEnv("DATABASE_URL", ""),
		JWTSecret:                 getEnv("JWT_SECRET", ""),
		JWTExpirationHours:        getEnvAsInt("JWT_EXPIRATION_HOURS", 24),
		StoragePath:               getEnv("STORAGE_PATH", "./storage"),
		IdentityDocumentsRequired: getEnvAsBool("IDENTITY_DOCUMENTS_REQUIRED", false),
		MaxJSONBodyBytes:          getEnvAsInt64("MAX_JSON_BODY_BYTES", 2097152), // 2 MiB
		WorkerCount:               getEnvAsInt("WORKER_COUNT", 5),
		AllowedOrigins:            getEnvAsSlice("ALLOWED_ORIGINS", []string{"*"}),
		AppURL:                    getEnv("APP_URL", "https://fintera.securexapp.com"),
		PortalURL:                 getEnv("PORTAL_URL", "https://portal.fintera.app"),
		ResendAPIKey:              getEnv("RESEND_API_KEY", ""),
		FromEmail:                 getEnv("FROM_EMAIL", "noreply@fintera.app"),
		EmailOverrideTo:           getEnv("EMAIL_OVERRIDE_TO", ""),
		EnableEmailNotifications:  getEnvAsBool("ENABLE_EMAIL_NOTIFICATIONS", false),
		RollbarToken:              getEnv("ROLLBAR_TOKEN", ""),
		RollbarEnvironment:        getEnv("ROLLBAR_ENVIRONMENT", environment),
		RollbarCodeVersion:        getEnv("ROLLBAR_CODE_VERSION", ""),
		RollbarServerRoot:         getEnv("ROLLBAR_SERVER_ROOT", "github.com/sjperalta/fintera-api"),
		DefaultEmail:              getEnv("DEFAULT_EMAIL", ""),
		DefaultPassword:           getEnv("DEFAULT_PASSWORD", ""),
	}

	// Validate required configuration
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	if cfg.JWTSecret == "" && cfg.Environment == "production" {
		return nil, fmt.Errorf("JWT_SECRET is required in production")
	}

	// Set default JWT secret for development
	if cfg.JWTSecret == "" {
		cfg.JWTSecret = "dev-secret-change-in-production"
	}

	return cfg, nil
}

// getEnv reads an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// getEnvAsInt reads an environment variable as integer
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

// getEnvAsInt64 reads an environment variable as a 64-bit integer
func getEnvAsInt64(key string, defaultValue int64) int64 {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseInt(valueStr, 10, 64)
	if err != nil {
		return defaultValue
	}
	return value
}

// getEnvAsBool reads an environment variable as boolean
func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

// getEnvAsSlice reads an environment variable as comma-separated slice
func getEnvAsSlice(key string, defaultValue []string) []string {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}
	return strings.Split(valueStr, ",")
}
