package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all bot configuration parameters
type Config struct {
	BotToken     string
	Debug        bool
	PollTimeout  time.Duration
	LogLevel     string
	StartTime    time.Time
}

// Load loads configuration from .env file and environment variables
func Load() *Config {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}

	config := &Config{
		BotToken:    getEnv("TELEGRAM_BOT_TOKEN", ""),
		Debug:       getEnvBool("DEBUG", false),
		PollTimeout: time.Duration(getEnvInt("POLL_TIMEOUT", 10)) * time.Second,
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		StartTime:   time.Now(),
	}

	// Validate required parameters
	if config.BotToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is required")
	}

	return config
}

// getEnv gets environment variable or returns default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvBool gets boolean environment variable
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// getEnvInt gets integer environment variable
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}
