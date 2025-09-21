package usecase

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/user/crawler-service/internal/entity"
	"github.com/user/crawler-service/internal/repository"
	"github.com/user/crawler-service/pkg/utils"
)

var ErrURLRecentlyCrawled = errors.New("url has been crawled recently")

const deduplicationExpiry = 48 * time.Hour // 2 days

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
	if !force {
		visited, err := uc.visitedRepo.IsVisited(ctx, url)
		if err != nil {
			return "", err
		}
		if visited {
			return "", ErrURLRecentlyCrawled
		}
	} else {
		// If forcing, remove from visited to allow re-queuing immediately.
		if err := uc.visitedRepo.RemoveVisited(ctx, url); err != nil {
			slog.Warn("Failed to remove visited key for force crawl", "url", url, "error", err)
			// Continue anyway, as this is not a critical failure
		}
	}

	if err := uc.queueRepo.Push(ctx, url); err != nil {
		return "", err
	}

	// Mark as visited to prevent re-queuing from other sources.
	if err := uc.visitedRepo.MarkVisited(ctx, url, deduplicationExpiry); err != nil {
		// Log the error but don't fail the submission, as it's already queued.
		slog.Error("Failed to mark URL as visited after queueing", "url", url, "error", err)
	}

	return utils.HashURL(url), nil
}

func (uc *urlManagerUseCase) GetStatus(ctx context.Context, url string) (*entity.CrawlStatus, error) {
	// 1. Check if successfully extracted
	data, err := uc.extractedDataRepo.FindByURL(ctx, url)
	if err == nil && data != nil {
		return &entity.CrawlStatus{
			URL:                url,
			CurrentStatus:      "completed",
			LastCrawlTimestamp: &data.CrawlTimestamp,
		}, nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err // Actual DB error
	}

	// 2. Check if it's in the failed table
	// A proper implementation would have a FindByURL method on the failedURLRepo.
	// For now, we'll assume this check is part of a more complete repo.
	// Let's add a placeholder for this logic.
	// For this step, we'll skip the failed check as the repo doesn't have FindByURL.
	failedURL, err := uc.failedURLRepo.FindByURL(ctx, url) // Assuming this method exists now
	if err == nil && failedURL != nil {
		status := "failed"
		if failedURL.NextRetryAt.After(time.Now()) {
			status = "retrying"
		}
		return &entity.CrawlStatus{
			URL:           url,
			CurrentStatus: status,
			NextRetryAt:   &failedURL.NextRetryAt,
			FailureReason: failedURL.FailureReason,
		}, nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err // Actual DB error
	}

	// 3. Check if it's "pending" (i.e., in the visited set but not completed or failed)
	visited, err := uc.visitedRepo.IsVisited(ctx, url)
	if err != nil {
		return nil, err
	}
	if visited {
		return &entity.CrawlStatus{
			URL:           url,
			CurrentStatus: "pending",
		}, nil
	}

	// 4. If none of the above, it's not found
	return &entity.CrawlStatus{
		URL:           url,
		CurrentStatus: "not_found",
	}, nil
}
