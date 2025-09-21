package handler

import (
	"encoding/json"
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
		if err == usecase.ErrURLRecentlyCrawled {
			h.writeJSONError(w, err.Error(), http.StatusConflict)
			return
		}
		slog.Error("Failed to submit URL", "url", req.URL, "error", err)
		h.writeJSONError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	resp := response.SubmitCrawlResponse{
		Status:         "success",
		Message:        "URL submitted for crawling.",
		CrawlRequestID: crawlID,
	}
	h.writeJSON(w, http.StatusAccepted, resp)
}

func (h *Handler) HandleGetCrawlStatus(w http.ResponseWriter, r *http.Request) {
	urlParam := r.URL.Query().Get("url")
	if urlParam == "" {
		h.writeJSONError(w, "URL query parameter is required", http.StatusBadRequest)
		return
	}

	status, err := h.urlManager.GetStatus(r.Context(), urlParam)
	if err != nil {
		slog.Error("Failed to get crawl status", "url", urlParam, "error", err)
		h.writeJSONError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if status.CurrentStatus == "not_found" {
		h.writeJSONError(w, "Crawl status not found for the given URL", http.StatusNotFound)
		return
	}

	h.writeJSON(w, http.StatusOK, status)
}

func (h *Handler) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	resp := map[string]string{"status": "ok"}
	h.writeJSON(w, http.StatusOK, resp)
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

