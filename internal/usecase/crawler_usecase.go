package usecase

import (
	"context"
	"errors"
	"log/slog"

	"github.com/redis/go-redis/v9"
	"github.com/user/crawler-service/internal/entity"
	"github.com/user/crawler-service/internal/repository"
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
			// Queue is empty, which is not an error.
			return nil
		}
		slog.Error("Failed to pop URL from queue", "error", err)
		return err
	}

	slog.Info("Processing URL from queue", "url", url)

	extractedData, err := uc.crawlerRepo.Crawl(ctx, url)
	if err != nil {
		slog.Warn("Crawling failed, scheduling retry", "url", url, "error", err)
		return uc.handleCrawlFailure(ctx, url, err)
	}

	slog.Info("Crawling successful, saving data", "url", url)
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
		// This is not a critical error, so we just log it.
		slog.Warn("Failed to delete URL from failed_urls table", "url", data.URL, "error", err)
	}

	return nil
}

func (uc *crawlerUseCase) handleCrawlFailure(ctx context.Context, url string, crawlErr error) error {
	failedURL := &entity.FailedURL{
		URL:           url,
		FailureReason: crawlErr.Error(),
		// The repository's SaveOrUpdate method is responsible for
		// incrementing the retry count and calculating the next_retry_at.
	}

	if err := uc.failedURLRepo.SaveOrUpdate(ctx, failedURL); err != nil {
		slog.Error("Failed to save or update failed URL record", "url", url, "error", err)
		return err
	}

	return nil
}
```
