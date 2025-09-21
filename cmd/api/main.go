package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp" // Kept for /metrics endpoint
	"github.com/redis/go-redis/v9"                            // Changed from github.com/redis/go-redis/v9

	"github.com/user/crawler-service/internal/adapter/chromedp_crawler"
	"github.com/user/crawler-service/internal/adapter/postgres" // Changed import alias
	redis_adapter "github.com/user/crawler-service/internal/adapter/redis"
	http_delivery "github.com/user/crawler-service/internal/delivery/http/handler" // Changed import alias
	"github.com/user/crawler-service/internal/repository"                          // Added for QueueRepository in metrics collector
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

	// Create a context that is cancelled on interruption signals
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// --- Database Connections ---
	pgConnString := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.PostgresHost, cfg.PostgresPort, cfg.PostgresUser, cfg.PostgresPassword, cfg.PostgresDB)
	dbPool, err := pgxpool.New(ctx, pgConnString) // Use ctx for connection
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
	if err := redisClient.Ping(ctx).Err(); err != nil { // Use ctx for ping
		slog.Error("Unable to connect to Redis", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close() // Add defer close for redis
	slog.Info("Successfully connected to Redis")

	// --- Repositories ---
	visitedRepo := redis_adapter.NewVisitedRepo(redisClient)
	queueRepo := redis_adapter.NewQueueRepo(redisClient)
	extractedDataRepo := postgres.NewExtractedDataRepo(dbPool) // Use new postgres adapter
	failedURLRepo := postgres.NewFailedURLRepo(dbPool)         // Use new postgres adapter

	// Initialize Crawler Repository
	// For now, no proxies are configured. This can be loaded from config.
	var proxies []string
	crawlerRepo, err := chromedp_crawler.NewChromedpCrawler(cfg.MaxConcurrency, cfg.PageLoadTimeout, proxies) // Added proxies argument
	if err != nil {
		slog.Error("Failed to initialize Chromedp Crawler", "error", err)
		os.Exit(1)
	}
	slog.Info("Chromedp crawler initialized")

	// --- Use Cases ---
	urlManager := usecase.NewURLManager(visitedRepo, queueRepo, extractedDataRepo, failedURLRepo)
	// The crawler use case would be run by background workers.
	// For the API, we only need the URL manager.
	_ = usecase.NewCrawlerUseCase(queueRepo, crawlerRepo, extractedDataRepo, failedURLRepo) // Commented out as per attempted content
	slog.Info("URL Manager use case initialized")                                           // Updated log message

	// --- Start Background Services ---
	go startQueueMetricsCollector(ctx, queueRepo) // Added from attempted content

	// --- HTTP Server ---
	apiHandler := http_delivery.NewHandler(urlManager) // Use http_delivery
	// httpRouter := http_delivery.New(apiHandler)        // Use http_delivery

	// Add Prometheus metrics handler
	http.Handle("/metrics", promhttp.Handler())
	http.Handle("/", httpRouter) // Use the new router

	server := &http.Server{
		Addr:         net.JoinHostPort("", cfg.ServerPort), // Use net.JoinHostPort
		Handler:      http.DefaultServeMux,                 // Use DefaultServeMux to handle both router and metrics
		ReadTimeout:  10 * time.Second,                     // Adopted from attempted content
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second, // Kept from original
	}

	go func() {
		slog.Info("Server is starting", "port", cfg.ServerPort)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) { // Adopted error check
			slog.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	// --- Graceful Shutdown ---
	<-ctx.Done() // Use the context from signal.NotifyContext
	slog.Info("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Kept original timeout duration
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown failed", "error", err)
	} else {
		slog.Info("Server gracefully stopped")
	}
}

// startQueueMetricsCollector periodically polls the queue for its size and updates the Prometheus gauge.
func startQueueMetricsCollector(ctx context.Context, queueRepo repository.QueueRepository) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	slog.Info("Starting queue metrics collector")

	for {
		select {
		case <-ticker.C:
			size, err := queueRepo.Size(context.Background()) // Use background context for short-lived operation
			if err != nil {
				slog.Error("Failed to get queue size for metrics", "error", err)
				continue
			}
			metrics.URLsInQueue.Set(float64(size))
		case <-ctx.Done():
			slog.Info("Stopping queue metrics collector")
			return
		}
	}
}
