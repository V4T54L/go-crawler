package repository

import (
	"context"
	"github.com/user/crawler-service/internal/entity"
)

// FailedURLRepository defines the interface for managing URLs that failed to be crawled.
type FailedURLRepository interface {
	// SaveOrUpdate creates or updates a record for a failed URL.
	SaveOrUpdate(ctx context.Context, failedURL *entity.FailedURL) error
	// FindRetryable retrieves a batch of URLs that are due for a retry.
	FindRetryable(ctx context.Context, limit int) ([]*entity.FailedURL, error)
	// Delete removes a failed URL record, typically after a successful crawl.
	Delete(ctx context.Context, url string) error
}

