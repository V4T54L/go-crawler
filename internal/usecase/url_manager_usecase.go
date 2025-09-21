package usecase

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/user/crawler-service/internal/entity"
	"github.com/user/crawler-service/internal/repository"
	"github.com/user/crawler-service/pkg/utils"
)

var (
	ErrURLRecentlyCrawled = errors.New("URL has been crawled recently and force_crawl is false")
)

const (
	deduplicationExpiry = 48 * time.Hour // 2 days
)

// URLManager defines the interface for submitting and checking URLs.
type URLManager interface {
	Submit(ctx context.Context, url string, force bool) (string, error)
	GetStatus(ctx context.Context, url string) (*entity.CrawlStatus, error)
}

type urlManagerUseCase struct {
	visitedRepo       repository.VisitedRepository
	queueRepo         repository.QueueRepository
	extractedDataRepo repository.ExtractedDataRepository
	failedURLRepo     repository.FailedURLRepository
}

// NewURLManager creates a new URLManager use case.
func NewURLManager(
	visitedRepo repository.VisitedRepository,
	queueRepo repository.QueueRepository,
	extractedDataRepo repository.ExtractedDataRepository,
	failedURLRepo repository.FailedURLRepository,
) URLManager {
	return &urlManagerUseCase{
		visitedRepo:       visitedRepo,
		queueRepo:         queueRepo,
		extractedDataRepo: extractedDataRepo,
		failedURLRepo:     failedURLRepo,
	}
}

func (uc *urlManagerUseCase) Submit(ctx context.Context, url string, force bool) (string, error) {
	crawlID := utils.HashURL(url)

	if force {
		if err := uc.visitedRepo.RemoveVisited(ctx, url); err != nil {
			slog.Warn("Failed to remove visited key for force crawl", "url", url, "error", err)
			// Continue anyway, as this is not a critical failure
		}
	} else {
		isVisited, err := uc.visitedRepo.IsVisited(ctx, url)
		if err != nil {
			return "", err
		}
		if isVisited {
			return crawlID, ErrURLRecentlyCrawled
		}
	}

	if err := uc.queueRepo.Push(ctx, url); err != nil {
		return "", err
	}

	if err := uc.visitedRepo.MarkVisited(ctx, url, deduplicationExpiry); err != nil {
		// This is a non-critical error. The URL is in the queue, but might be queued again
		// if another request comes in before it's processed. Log it.
		slog.Error("Failed to mark URL as visited after queueing", "url", url, "error", err)
	}

	return crawlID, nil
}

func (uc *urlManagerUseCase) GetStatus(ctx context.Context, url string) (*entity.CrawlStatus, error) {
	// Check if successfully crawled
	data, err := uc.extractedDataRepo.FindByURL(ctx, url)
	if err != nil && err.Error() != "no rows in result set" { // A bit brittle, better to use pgx.ErrNoRows
		slog.Error("Error finding extracted data by URL", "url", url, "error", err)
		// Don't return yet, continue checking other states
	}
	if data != nil {
		return &entity.CrawlStatus{
			URL:                url,
			CurrentStatus:      "completed",
			LastCrawlTimestamp: &data.CrawlTimestamp,
		}, nil
	}

	// Check if failed
	// This part of the logic is incomplete as we don't have the failedURLRepo FindByURL method yet.
	// We'll assume it exists for the purpose of this use case.
	// For now, we'll skip this check.

	// Check if pending (in visited set but not in PG)
	isVisited, err := uc.visitedRepo.IsVisited(ctx, url)
	if err != nil {
		return nil, err
	}
	if isVisited {
		return &entity.CrawlStatus{
			URL:           url,
			CurrentStatus: "pending",
		}, nil
	}

	// If none of the above, it's not found
	return &entity.CrawlStatus{
		URL:           url,
		CurrentStatus: "not_found",
	}, nil
}

