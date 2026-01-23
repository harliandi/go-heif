package config

import (
	"os"
	"strconv"
)

// Config holds application configuration
type Config struct {
	Port               int
	MaxUploadMB        int
	TargetSizeKB       int
	MaxConcurrent      int
	RateLimitPerSec    int
	RateLimitBurst     int
	WorkerCount        int
}

// Load loads configuration from environment variables with defaults
func Load() *Config {
	cfg := &Config{
		Port:            getEnvInt("PORT", 8080),
		MaxUploadMB:     getEnvInt("MAX_UPLOAD_MB", 10),
		TargetSizeKB:    getEnvInt("TARGET_SIZE_KB", 500),
		MaxConcurrent:   getEnvInt("MAX_CONCURRENT", 50),
		RateLimitPerSec: getEnvInt("RATE_LIMIT", 10),
		RateLimitBurst:  getEnvInt("RATE_LIMIT_BURST", 20),
		WorkerCount:     getEnvInt("WORKER_COUNT", 10),
	}
	return cfg
}

func getEnvInt(key string, defaultValue int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultValue
}
