package postgres

import (
	"context"
	"fmt"
	"time" // Added from attempted content

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/user/crawler-service/internal/entity"
)

const (
	maxRetries     = 5             // Adopted from attempted content
	initialBackoff = 5 * time.Second // Adopted from attempted content
)

// FailedURLRepoImpl provides a concrete implementation for the FailedURLRepository interface using PostgreSQL.
type FailedURLRepoImpl struct {
	db *pgxpool.Pool
}

// NewFailedURLRepo creates a new instance of FailedURLRepoImpl.
func NewFailedURLRepo(db *pgxpool.Pool) *FailedURLRepoImpl {
	return &FailedURLRepoImpl{db: db}
}

// SaveOrUpdate creates or updates a record for a failed URL.
// It increments the retry_count on conflict and calculates next_retry_at using exponential backoff with jitter.
func (r *FailedURLRepoImpl) SaveOrUpdate(ctx context.Context, failedURL *entity.FailedURL) error {
	// Adopted the sophisticated query from attempted content
	query := `
        INSERT INTO failed_urls (url, failure_reason, http_status_code, last_attempt_timestamp, retry_count, next_retry_at)
        VALUES ($1, $2, $3, $4, 1, NOW() + ($5 * INTERVAL '1 second'))
        ON CONFLICT (url) DO UPDATE
        SET
            failure_reason = EXCLUDED.failure_reason,
            http_status_code = EXCLUDED.http_status_code,
            last_attempt_timestamp = EXCLUDED.last_attempt_timestamp,
            retry_count = failed_urls.retry_count + 1,
            next_retry_at = CASE
                WHEN failed_urls.retry_count + 1 >= $6 THEN NULL
                ELSE NOW() + (
                    ($5 * pow(2, failed_urls.retry_count)) -- Exponential backoff
                    * (1 + random() * 0.4 - 0.2)           -- Jitter +/- 20%
                ) * INTERVAL '1 second'
            END;
    `
	_, err := r.db.Exec(ctx, query,
		failedURL.URL,
		failedURL.FailureReason,
		failedURL.HTTPStatusCode,
		failedURL.LastAttemptTimestamp,
		initialBackoff.Seconds(), // Use constant
		maxRetries,               // Use constant
	)

	if err != nil { // Adopted improved error wrapping
		return fmt.Errorf("failed to save or update failed URL: %w", err)
	}
	return nil
}

// FindRetryable retrieves a batch of URLs that are due for a retry.
func (r *FailedURLRepoImpl) FindRetryable(ctx context.Context, limit int) ([]*entity.FailedURL, error) {
	query := `
        SELECT id, url, failure_reason, http_status_code, last_attempt_timestamp, retry_count, next_retry_at
        FROM failed_urls
        WHERE next_retry_at IS NOT NULL AND next_retry_at <= NOW() -- Added IS NOT NULL from attempted
        ORDER BY next_retry_at
        LIMIT $1;
    `
	rows, err := r.db.Query(ctx, query, limit)
	if err != nil { // Adopted improved error wrapping
		return nil, fmt.Errorf("failed to query retryable URLs: %w", err)
	}
	defer rows.Close()

	var failedURLs []*entity.FailedURL // Renamed from 'urls' for clarity
	for rows.Next() {
		var fu entity.FailedURL // Renamed from 'u' for clarity
		if err := rows.Scan(
			&fu.ID,
			&fu.URL,
			&fu.FailureReason,
			&fu.HTTPStatusCode,
			&fu.LastAttemptTimestamp,
			&fu.RetryCount,
			&fu.NextRetryAt,
		); err != nil { // Adopted improved error wrapping
			return nil, fmt.Errorf("failed to scan retryable URL: %w", err)
		}
		failedURLs = append(failedURLs, &fu)
	}

	if err := rows.Err(); err != nil { // Adopted improved error wrapping
		return nil, fmt.Errorf("error after iterating over retryable URLs: %w", err)
	}

	return failedURLs, nil
}

// Delete removes a failed URL record, typically after a successful crawl.
func (r *FailedURLRepoImpl) Delete(ctx context.Context, url string) error {
	query := `DELETE FROM failed_urls WHERE url = $1;`
	cmdTag, err := r.db.Exec(ctx, query, url) // Adopted cmdTag from attempted
	if err != nil {                           // Adopted improved error wrapping
		return fmt.Errorf("failed to delete failed URL: %w", err)
	}
	if cmdTag.RowsAffected() == 0 { // Adopted from attempted
		// This is not necessarily an error, could just mean the URL was never in the failed table.
		// For strictness, one could return pgx.ErrNoRows, but for this use case, it's fine.
	}
	return nil
}

