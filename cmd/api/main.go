package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"github.com/user/crawler-service/internal/adapter/chromedp_crawler"
	postgres_adapter "github.com/user/crawler-service/internal/adapter/postgres"
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
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.PostgresUser, cfg.PostgresPassword, cfg.PostgresHost, cfg.PostgresPort, cfg.PostgresDB)
	dbPool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		slog.Error("Unable to connect to database", "error", err)
		os.Exit(1)
	}
	defer dbPool.Close()
	slog.Info("Successfully connected to PostgreSQL")

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if _, err := redisClient.Ping(context.Background()).Result(); err != nil {
		slog.Error("Unable to connect to Redis", "error", err)
		os.Exit(1)
	}
	slog.Info("Successfully connected to Redis")

	// --- Repositories ---
	visitedRepo := redis_adapter.NewVisitedRepo(redisClient)
	queueRepo := redis_adapter.NewQueueRepo(redisClient)
	extractedDataRepo := postgres_adapter.NewExtractedDataRepo(dbPool)
	failedURLRepo := postgres_adapter.NewFailedURLRepo(dbPool)

	// Initialize Crawler Repository
	crawlerRepo, err := chromedp_crawler.NewChromedpCrawler(cfg.MaxConcurrency, cfg.PageLoadTimeout)
	if err != nil {
		slog.Error("Failed to initialize Chromedp Crawler", "error", err)
		os.Exit(1)
	}
	slog.Info("Chromedp crawler initialized")

	// --- Use Cases ---
	urlManager := usecase.NewURLManager(visitedRepo, queueRepo, extractedDataRepo, failedURLRepo)

	// Initialize Crawler Use Case
	// Note: We are not starting the crawler worker here. This is just setting up the components.
	// A future step will introduce a worker that calls crawlerUseCase.ProcessURLFromQueue in a loop.
	_ = usecase.NewCrawlerUseCase(queueRepo, crawlerRepo, extractedDataRepo, failedURLRepo)
	slog.Info("Crawler use case initialized")

	// --- HTTP Server ---
	apiHandler := handler.NewHandler(urlManager)
	httpRouter := router.New(apiHandler)

	// Add Prometheus metrics handler
	http.Handle("/metrics", promhttp.Handler())
	http.Handle("/", httpRouter)

	server := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      http.DefaultServeMux, // Use DefaultServeMux to handle both router and metrics
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// --- Graceful Shutdown ---
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info("Server is starting", "port", cfg.ServerPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	<-stop
	slog.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Server shutdown failed", "error", err)
	} else {
		slog.Info("Server gracefully stopped")
	}
}

