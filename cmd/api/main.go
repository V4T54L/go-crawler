package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/user/crawler-service/internal/adapter/postgres"
	redis_adapter "github.com/user/crawler-service/internal/adapter/redis"
	"github.com/user/crawler-service/internal/delivery/http/handler"
	"github.com/user/crawler-service/internal/delivery/http/router"
	"github.com/user/crawler-service/internal/usecase"
	"github.com/user/crawler-service/pkg/config"
	"github.com/user/crawler-service/pkg/logger"
	"github.com/user/crawler-service/pkg/metrics"
)

func main() {
	// --- Configuration ---
	cfg := config.Load()

	// --- Logger ---
	logLevel := slog.LevelInfo
	if cfg.LogLevel == "debug" {
		logLevel = slog.LevelDebug
	}
	logger.Init(os.Stdout, logLevel)
	slog.Info("Logger initialized", "level", logLevel.String())

	// --- Metrics ---
	metrics.Init()
	slog.Info("Metrics initialized")

	// --- Database Connections ---
	ctx := context.Background()

	// PostgreSQL
	pgConnString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.PostgresUser, cfg.PostgresPassword, cfg.PostgresHost, cfg.PostgresPort, cfg.PostgresDB)
	dbpool, err := pgxpool.New(ctx, pgConnString)
	if err != nil {
		slog.Error("Unable to connect to database", "error", err)
		os.Exit(1)
	}
	defer dbpool.Close()
	slog.Info("PostgreSQL connection pool established")

	// Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		slog.Error("Unable to connect to Redis", "error", err)
		os.Exit(1)
	}
	slog.Info("Redis connection established")

	// --- Repositories ---
	visitedRepo := redis_adapter.NewVisitedRepo(rdb)
	queueRepo := redis_adapter.NewQueueRepo(rdb)
	extractedDataRepo := postgres.NewExtractedDataRepo(dbpool)
	failedURLRepo := postgres.NewFailedURLRepo(dbpool)

	// --- Use Cases ---
	urlManager := usecase.NewURLManager(visitedRepo, queueRepo, extractedDataRepo, failedURLRepo)

	// --- HTTP Server ---
	apiHandler := handler.NewHandler(urlManager)
	httpRouter := router.New(apiHandler)

	server := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      httpRouter,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	slog.Info("Starting server", "port", cfg.ServerPort)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("Could not listen on port", "port", cfg.ServerPort, "error", err)
		os.Exit(1)
	}
}

