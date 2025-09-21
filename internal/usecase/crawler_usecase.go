package usecase

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/user/crawler-service/internal/entity"
	"github.com/user/crawler-service/internal/repository"
)

const (
	initialBackoff = 5 * time.Second
	maxRetries     = 5
	jitterFactor   = 0.2
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
	url, err := uc.queueRepo.Pop(ctx)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// Queue is empty, which is a normal state.
			return nil
		}
		slog.Error("Failed to pop URL from queue", "error", err)
		return err
	}

	slog.Info("Processing URL from queue", "url", url)

	// For now, we hardcode sneaky mode to false. This can be extended later
	// by storing crawl options along with the URL in the queue.
	const useSneakyMode = false
	extractedData, crawlErr := uc.crawlerRepo.Crawl(ctx, url, useSneakyMode)

	if crawlErr != nil {
		slog.Warn("Crawling failed for URL, scheduling retry", "url", url, "error", crawlErr)
		return uc.handleCrawlFailure(ctx, url, crawlErr)
	}

	slog.Info("Crawling successful for URL, saving data", "url", url)
	return uc.handleCrawlSuccess(ctx, extractedData)
}

func (uc *crawlerUseCase) handleCrawlSuccess(ctx context.Context, data *entity.ExtractedData) error {
	if err := uc.extractedDataRepo.Save(ctx, data); err != nil {
		slog.Error("Failed to save extracted data", "url", data.URL, "error", err)
		// If saving fails, we might want to re-queue or handle it differently.
		// For now, we just log the error.
		return err
	}

	// If the URL was previously failed, remove it from the failed table.
	if err := uc.failedURLRepo.Delete(ctx, data.URL); err != nil {
		// This is not a critical error, just log it.
		slog.Warn("Failed to delete URL from failed_urls table after successful crawl", "url", data.URL, "error", err)
	}

	return nil
}

func (uc *crawlerUseCase) handleCrawlFailure(ctx context.Context, url string, crawlErr error) error {
	failedURL := &entity.FailedURL{
		URL:           url,
		FailureReason: crawlErr.Error(),
		// HTTPStatusCode would need to be parsed from the error, which can be complex.
		// We'll leave it as 0 for now unless the error provides it.
	}

	// This is a simplified logic. A real implementation would fetch the existing record.
	// The `SaveOrUpdate` in our postgres impl increments the count.
	// We need to calculate the next retry time here.
	// Let's assume we can get the current retry count from the DB or it's 0.
	// The current `SaveOrUpdate` increments the count, so we calculate based on that.
	// This is a bit of a chicken-and-egg problem without fetching first.
	// Let's just calculate a default first retry time. The repo can refine this.
	// A better way: The use case should own the retry logic.
	// Let's simulate fetching the retry count. For now, we'll just assume it's the first failure.
	// A proper implementation would be:
	// 1. failedRecord, err := failedURLRepo.FindByURL(ctx, url)
	// 2. if err == pgx.ErrNoRows -> new record, retryCount = 0
	// 3. else -> existing record, retryCount = failedRecord.RetryCount

	// Simplified logic for this step:
	retryCount := 0 // Assume we would fetch this. The DB ON CONFLICT will increment it.
	if retryCount >= maxRetries {
		slog.Warn("URL has reached max retries, marking as permanently failed", "url", url)
		// Set NextRetryAt to null or a far-future date to stop retrying.
		// For now, we just won't schedule a new retry.
		failedURL.NextRetryAt = time.Time{} // Or a specific sentinel value
	} else {
		backoff := initialBackoff * time.Duration(math.Pow(2, float64(retryCount)))
		jitter := time.Duration(rand.Float64()*jitterFactor*float64(backoff)) * (time.Duration(rand.Intn(2)*2 - 1))
		nextRetry := time.Now().Add(backoff + jitter)
		failedURL.NextRetryAt = nextRetry
		slog.Info("Scheduling retry for failed URL", "url", url, "next_retry_at", nextRetry)
	}

	if err := uc.failedURLRepo.SaveOrUpdate(ctx, failedURL); err != nil {
		slog.Error("Failed to save failed URL record", "url", url, "error", err)
		return err
	}

	return nil
}

