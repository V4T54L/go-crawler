package main

import (
	"context"
	"crawler/internal/api"
	"crawler/internal/config"
	"crawler/internal/crawler"
	"crawler/internal/monitoring"
	"crawler/internal/proxy"
	"crawler/internal/storage"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
)

func main() {
	// Initialize structured logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("could not load config", zap.Error(err))
	}

	// Initialize Storage Layer
	pgStore, err := storage.NewPostgresStore(cfg.PostgresURL)
	if err != nil {
		logger.Fatal("failed to connect to postgres", zap.Error(err))
	}
	redisStore := storage.NewRedisStore(cfg.RedisAddr)

	// Initialize Monitoring, Proxies
	metrics := monitoring.NewMetrics()
	proxyManager := proxy.NewManager()

	// Initialize Core Crawler
	coreCrawler := crawler.NewCrawler(cfg, redisStore, pgStore, proxyManager, metrics, logger)
	coreCrawler.Start()

	// Initialize API Server
	server := api.NewServer(cfg, coreCrawler, pgStore, redisStore, metrics, logger)

	// Graceful Shutdown
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("could not start server", zap.Error(err))
		}
	}()

	logger.Info("server started", zap.String("port", cfg.ServerPort))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	coreCrawler.Stop()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("server forced to shutdown", zap.Error(err))
	}

	logger.Info("server exiting")
}
