package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (s *Server) setupRouter() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/metrics", promhttp.Handler().(http.HandlerFunc))
	r.Get("/api/health", s.handleHealthCheck)

	r.Route("/api", func(r chi.Router) {
		r.Post("/crawl", s.handleCrawlRequest)
		r.Get("/status", s.handleStatusRequest)
	})

	return r
}
