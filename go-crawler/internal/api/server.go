package api

import (
	"context"
	"crawler/internal/config"
	"crawler/internal/crawler"
	"crawler/internal/monitoring"
	"crawler/internal/storage"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Server holds the dependencies for the HTTP server.
type Server struct {
	config     *config.Config
	router     http.Handler
	httpServer *http.Server
	crawler    *crawler.Crawler
	pgStore    *storage.PostgresStore
	redisStore *storage.RedisStore
	metrics    *monitoring.Metrics
	logger     *zap.Logger
}

func NewServer(cfg *config.Config, cr *crawler.Crawler, ps *storage.PostgresStore, rs *storage.RedisStore, m *monitoring.Metrics, l *zap.Logger) *Server {
	s := &Server{
		config:     cfg,
		crawler:    cr,
		pgStore:    ps,
		redisStore: rs,
		metrics:    m,
		logger:     l,
	}
	s.router = s.setupRouter()
	return s
}

func (s *Server) Start() error {
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%s", s.config.ServerPort),
		Handler:      s.router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
