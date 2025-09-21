package config

import (
	"log"
	"os"
	"strconv"
	"time"
)

// Config holds the application configuration.
type Config struct {
	ServerPort string
	LogLevel   string

	PostgresHost     string
	PostgresPort     string
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	MaxConcurrency int
	PageLoadTimeout time.Duration
}

// Load loads configuration from environment variables.
func Load() *Config {
	return &Config{
		ServerPort:       getEnv("SERVER_PORT", "8080"),
		LogLevel:         getEnv("LOG_LEVEL", "info"),
		PostgresHost:     getEnv("POSTGRES_HOST", "localhost"),
		PostgresPort:     getEnv("POSTGRES_PORT", "5432"),
		PostgresUser:     getEnv("POSTGRES_USER", "user"),
		PostgresPassword: getEnv("POSTGRES_PASSWORD", "password"),
		PostgresDB:       getEnv("POSTGRES_DB", "crawler"),
		RedisAddr:        getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:    getEnv("REDIS_PASSWORD", ""),
		RedisDB:          getEnvAsInt("REDIS_DB", 0),
		MaxConcurrency:   getEnvAsInt("MAX_CONCURRENCY", 10),
		PageLoadTimeout:  getEnvAsDuration("PAGE_LOAD_TIMEOUT_SECONDS", 60) * time.Second,
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	valueStr := getEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return fallback
}

func getEnvAsDuration(key string, fallback int) time.Duration {
	return time.Duration(getEnvAsInt(key, fallback))
}

