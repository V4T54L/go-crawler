package api

import (
	"context"
	"crawler/internal/domain"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"
)

func (s *Server) handleCrawlRequest(w http.ResponseWriter, r *http.Request) {
	var req domain.CrawlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.URLs) == 0 {
		s.respondWithError(w, http.StatusBadRequest, "URLs list cannot be empty")
		return
	}

	for _, u := range req.URLs {
		if _, err := url.ParseRequestURI(u); err != nil {
			s.respondWithError(w, http.StatusBadRequest, "Invalid URL in list: "+u)
			return
		}
		task := domain.URLTask{URL: u, ForceCrawl: req.ForceCrawl}
		s.crawler.Submit(task)
	}

	s.respondWithJSON(w, http.StatusAccepted, map[string]string{"message": "URLs accepted for crawling"})
}

func (s *Server) handleStatusRequest(w http.ResponseWriter, r *http.Request) {
	urlParam := r.URL.Query().Get("url")
	if urlParam == "" {
		s.respondWithError(w, http.StatusBadRequest, "URL query parameter is required")
		return
	}

	status, err := s.pgStore.GetCrawlStatus(r.Context(), urlParam)
	if err != nil {
		if err.Error() == "not_found" {
			s.respondWithError(w, http.StatusNotFound, "URL status not found")
			return
		}
		s.logger.Error("failed to get crawl status", zap.Error(err))
		s.respondWithError(w, http.StatusInternalServerError, "Could not retrieve status")
		return
	}

	s.respondWithJSON(w, http.StatusOK, status)
}

func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	healthStatus := make(map[string]string)

	// Check Postgres
	if err := s.pgStore.Ping(ctx); err != nil {
		healthStatus["postgres"] = "unhealthy"
		s.logger.Error("health check failed for postgres", zap.Error(err))
	} else {
		healthStatus["postgres"] = "healthy"
	}

	// Check Redis
	if err := s.redisStore.Ping(ctx); err != nil {
		healthStatus["redis"] = "unhealthy"
		s.logger.Error("health check failed for redis", zap.Error(err))
	} else {
		healthStatus["redis"] = "healthy"
	}

	isHealthy := healthStatus["postgres"] == "healthy" && healthStatus["redis"] == "healthy"
	if !isHealthy {
		s.respondWithJSON(w, http.StatusServiceUnavailable, healthStatus)
		return
	}

	s.respondWithJSON(w, http.StatusOK, healthStatus)
}

// --- Helper Functions ---

func (s *Server) respondWithError(w http.ResponseWriter, code int, message string) {
	s.respondWithJSON(w, code, map[string]string{"error": message})
}

func (s *Server) respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
