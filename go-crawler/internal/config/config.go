package config

import (
	"github.com/spf13/viper"
)

// Config stores all configuration for the application.
type Config struct {
	PostgresURL       string `mapstructure:"POSTGRES_URL"`
	RedisAddr         string `mapstructure:"REDIS_ADDR"`
	ServerPort        string `mapstructure:"SERVER_PORT"`
	MaxRetries        int    `mapstructure:"MAX_RETRIES"`
	CrawlWorkers      int    `mapstructure:"CRAWL_WORKERS"`
	CrawlTimeout      int    `mapstructure:"CRAWL_TIMEOUT"`
	DeduplicationDays int    `mapstructure:"DEDUPLICATION_DAYS"`
}

// Load reads configuration from file or environment variables.
func Load() (*Config, error) {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	// Attempt to read the .env file, but don't fail if it's not present
	// This allows configuration purely through environment variables in production
	_ = viper.ReadInConfig()

	// Set default values
	viper.SetDefault("SERVER_PORT", "8080")
	viper.SetDefault("MAX_RETRIES", 2)
	viper.SetDefault("CRAWL_WORKERS", 10)
	viper.SetDefault("CRAWL_TIMEOUT", 30) // in seconds
	viper.SetDefault("DEDUPLICATION_DAYS", 2)

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
