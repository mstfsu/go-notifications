package config

import (
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL       string
	RedisURL          string
	WebhookSiteURL    string
	ServerPort        string
	WorkerConcurrency int
	LogLevel          string
	MaxRetries        int
}

func Load() *Config {
	return &Config{
		DatabaseURL:       getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/notifications?sslmode=disable"),
		RedisURL:          getEnv("REDIS_URL", "redis://localhost:6379"),
		WebhookSiteURL:    getEnv("WEBHOOK_SITE_URL", "https://webhook.site/your-uuid-here"),
		ServerPort:        getEnv("SERVER_PORT", "8080"),
		WorkerConcurrency: getEnvInt("WORKER_CONCURRENCY", 10),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
		MaxRetries:        getEnvInt("MAX_RETRIES", 5),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
