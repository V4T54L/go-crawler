package usecase

import (
	"context"
	"errors"
	"fmt" // Added from attempted content
	"log/slog"
	"net/url" // Added from attempted content
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/user/crawler-service/internal/entity"
	"github.com/user/crawler-service/internal/repository"
	"github.com/user/crawler-service/pkg/metrics" // Added from attempted content
)

const (
	// These constants are now primarily used by the FailedURLRepoImpl for initial backoff,
	// but kept here for consistency if the use case needs to reference them.
	initialBackoff = 5 * time.Second
	maxRetries     = 5
	jitterFactor   = 0.2 // +/- 20%
)

// Crawler defines the interface for the core crawling process.
type Crawler interface {
	ProcessURLFromQueue(ctx context.Context) error
}

type crawlerUseCase struct {
	queueRepo         repository.QueueRepository
	crawlerRepo       repository.CrawlerRepository
	extractedDataRepo repository.ExtractedDataRepository
	failedURLRepo     repository.FailedURLRepository
}

// NewCrawlerUseCase creates a new instance of the crawler use case.
func NewCrawlerUseCase(
	queueRepo repository.QueueRepository,
	crawlerRepo repository.CrawlerRepository,
	extractedDataRepo repository.ExtractedDataRepository,
	failedURLRepo repository.FailedURLRepository,
) Crawler {
	return &crawlerUseCase{
		queueRepo:         queueRepo,
		crawlerRepo:       crawlerRepo,
		extractedDataRepo: extractedDataRepo,
		failedURLRepo:     failedURLRepo,
	}
}

// ProcessURLFromQueue fetches a single URL from the queue and processes it.
// It handles success by saving data and failure by scheduling a retry.
func (uc *crawlerUseCase) ProcessURLFromQueue(ctx context.Context) error {
	urlToCrawl, err := uc.queueRepo.Pop(ctx) // Renamed variable from 'url'
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// Queue is empty, which is a normal state.
			return nil
		}
		return fmt.Errorf("failed to pop URL from queue: %w", err) // Adopted improved error wrapping
	}

	slog.Info("Processing URL from queue", "url", urlToCrawl)

	startTime := time.Now()
	parsedURL, _ := url.Parse(urlToCrawl) // Adopted from attempted content
	domain := "unknown"
	if parsedURL != nil {
		domain = parsedURL.Hostname()
	}

	// For now, we default to "sneaky" mode for robustness. This could be configurable per URL.
	const useSneakyMode = true // Adopted from attempted content
	extractedData, crawlErr := uc.crawlerRepo.Crawl(ctx, urlToCrawl, useSneakyMode)

	duration := time.Since(startTime)
	metrics.CrawlDuration.WithLabelValues(domain).Observe(duration.Seconds()) // Adopted from attempted content

	if crawlErr != nil {
		slog.Error("Crawling failed for URL, scheduling retry", "url", urlToCrawl, "error", crawlErr) // Changed log level to Error
		return uc.handleCrawlFailure(ctx, urlToCrawl, crawlErr)
	}

	slog.Info("Crawling successful for URL, saving data", "url", urlToCrawl, "duration_ms", duration.Milliseconds()) // Adopted from attempted content
	return uc.handleCrawlSuccess(ctx, extractedData)
}

func (uc *crawlerUseCase) handleCrawlSuccess(ctx context.Context, data *entity.ExtractedData) error {
	metrics.CrawlsTotal.WithLabelValues("success", "").Inc() // Adopted from attempted content

	if err := uc.extractedDataRepo.Save(ctx, data); err != nil {
		return fmt.Errorf("failed to save extracted data for %s: %w", data.URL, err) // Adopted improved error wrapping
	}

	// If the URL was previously failed, remove it from the failed table.
	if err := uc.failedURLRepo.Delete(ctx, data.URL); err != nil {
		// This is not a critical error, just log it.
		slog.Warn("Failed to delete URL from failed_urls table after successful crawl", "url", data.URL, "error", err)
	}

	return nil
}

func (uc *crawlerUseCase) handleCrawlFailure(ctx context.Context, url string, crawlErr error) error {
	errorType := "unknown" // Adopted from attempted content
	var httpStatusCode int // Adopted from attempted content
	switch {
	case errors.Is(crawlErr, repository.ErrCrawlTimeout):
		errorType = "timeout"
	case errors.Is(crawlErr, repository.ErrNavigationFailed):
		errorType = "navigation"
	case errors.Is(crawlErr, repository.ErrExtractionFailed):
		errorType = "extraction"
	case errors.Is(crawlErr, repository.ErrContentRestricted):
		errorType = "restricted"
		// Try to extract status code from error message for logging
		fmt.Sscanf(crawlErr.Error(), "content is restricted or requires authentication: received status code %d", &httpStatusCode)
	}
	metrics.CrawlsTotal.WithLabelValues("failure", errorType).Inc() // Adopted from attempted content

	failedURL := &entity.FailedURL{
		URL:                  url,
		FailureReason:        crawlErr.Error(),
		HTTPStatusCode:       httpStatusCode,       // Adopted from attempted content
		LastAttemptTimestamp: time.Now(),           // Adopted from attempted content
		// NextRetryAt is now handled by the repository's SaveOrUpdate method
	}

	if err := uc.failedURLRepo.SaveOrUpdate(ctx, failedURL); err != nil {
		return fmt.Errorf("failed to save or update failed URL record for %s: %w", url, err) // Adopted improved error wrapping
	}

	return nil
}

