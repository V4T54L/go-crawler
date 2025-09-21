package router

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/user/crawler-service/internal/delivery/http/handler"
	"github.com/user/crawler-service/internal/delivery/http/middleware"
)

func New(h *handler.Handler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", h.HandleHealthCheck)
	mux.HandleFunc("POST /api/crawl", h.HandleSubmitCrawl)
	mux.HandleFunc("GET /api/status", h.HandleGetCrawlStatus)

	// Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Apply middlewares
	var chainedHandler http.Handler = mux
	chainedHandler = middleware.Metrics(chainedHandler)
	chainedHandler = middleware.Logging(chainedHandler)

	return chainedHandler
}

