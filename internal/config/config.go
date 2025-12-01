package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
	
	"provisioning/internal/db"
)

// Config holds all application configuration
// Loaded from environment variables
type Config struct {
	Database db.Config
	App      AppConfig
	AWS      AWSConfig
}

// AppConfig holds application-level configuration
type AppConfig struct {
	Environment string // development, staging, production
	LogLevel    string // debug, info, warn, error
	Port        int    // HTTP server port
}

// AWSConfig holds AWS-specific configuration
type AWSConfig struct {
	Region                 string // AWS region (e.g., "us-east-1")
	KMSKeyARN              string // KMS key ARN for encryption
	ProvisioningLambdaName string // Lambda function name for tenant provisioning
}

// LoadFromEnv loads configuration from environment variables
// Returns error if required variables are missing
func LoadFromEnv() (*Config, error) {
	cfg := &Config{
		Database: db.Config{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnvAsInt("DB_PORT", 5432),
			User:            getEnv("DB_USER", "postgres"),
			Password:        getEnv("DB_PASSWORD", ""),
			Database:        getEnv("DB_NAME", "provisioning"),
			SSLMode:         getEnv("DB_SSLMODE", "disable"),
			MaxConns:        int32(getEnvAsInt("DB_MAX_CONNS", 25)),
			MinConns:        int32(getEnvAsInt("DB_MIN_CONNS", 5)),
			MaxConnLifetime: time.Duration(getEnvAsInt("DB_MAX_CONN_LIFETIME_MINUTES", 30)) * time.Minute,
			MaxConnIdleTime: time.Duration(getEnvAsInt("DB_MAX_CONN_IDLE_TIME_MINUTES", 5)) * time.Minute,
		},
		App: AppConfig{
			Environment: getEnv("ENVIRONMENT", "development"),
			LogLevel:    getEnv("LOG_LEVEL", "info"),
			Port:        getEnvAsInt("PORT", 8080),
		},
		AWS: AWSConfig{
			Region:                 getEnv("AWS_REGION", "us-east-1"),
			KMSKeyARN:              getEnv("KMS_KEY_ARN", ""),
			ProvisioningLambdaName: getEnv("AWS_PROVISIONING_LAMBDA_NAME", "tenant-provisioning"),
		},
	}

	// Validate required fields
	if cfg.Database.Password == "" {
		return nil, fmt.Errorf("DB_PASSWORD is required")
	}

	// KMS key ARN is optional for development but required for production
	if cfg.App.Environment == "production" && cfg.AWS.KMSKeyARN == "" {
		return nil, fmt.Errorf("KMS_KEY_ARN is required in production")
	}

	return cfg, nil
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt retrieves an environment variable as an integer or returns a default value
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}
