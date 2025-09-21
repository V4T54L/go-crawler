package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/user/crawler-service/internal/delivery/http/request"
	"github.com/user/crawler-service/internal/delivery/http/response"
	"github.com/user/crawler-service/internal/usecase"
)

type Handler struct {
	urlManager usecase.URLManager
}

func NewHandler(urlManager usecase.URLManager) *Handler {
	return &Handler{
		urlManager: urlManager,
	}
}

func (h *Handler) HandleSubmitCrawl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req request.SubmitCrawlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if _, err := url.ParseRequestURI(req.URL); err != nil {
		h.writeJSONError(w, "Invalid URL format", http.StatusBadRequest)
		return
	}

	crawlID, err := h.urlManager.Submit(r.Context(), req.URL, req.ForceCrawl)
	if err != nil {
		if errors.Is(err, usecase.ErrURLRecentlyCrawled) {
			h.writeJSONError(w, err.Error(), http.StatusConflict)
			return
		}
		slog.Error("Failed to submit URL", "url", req.URL, "error", err)
		h.writeJSONError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	resp := response.SubmitCrawlResponse{
		Status:         "success",
		Message:        "URL submitted for crawling",
		CrawlRequestID: crawlID,
	}
	h.writeJSON(w, http.StatusAccepted, resp)
}

func (h *Handler) HandleGetCrawlStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rawURL := r.URL.Query().Get("url")
	if rawURL == "" {
		h.writeJSONError(w, "URL query parameter is required", http.StatusBadRequest)
		return
	}

	if _, err := url.ParseRequestURI(rawURL); err != nil {
		h.writeJSONError(w, "Invalid URL format in query parameter", http.StatusBadRequest)
		return
	}

	status, err := h.urlManager.GetStatus(r.Context(), rawURL)
	if err != nil {
		slog.Error("Failed to get crawl status", "url", rawURL, "error", err)
		h.writeJSONError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if status.CurrentStatus == "not_found" {
		h.writeJSONError(w, "Crawl status not found for the given URL", http.StatusNotFound)
		return
	}

	resp := response.CrawlStatusResponse{
		URL:                status.URL,
		CurrentStatus:      status.CurrentStatus,
		LastCrawlTimestamp: status.LastCrawlTimestamp,
		NextRetryAt:        status.NextRetryAt,
		FailureReason:      status.FailureReason,
	}

	h.writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("Failed to write JSON response", "error", err)
	}
}

func (h *Handler) writeJSONError(w http.ResponseWriter, message string, status int) {
	h.writeJSON(w, status, map[string]string{"error": message})
}

