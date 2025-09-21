package postgres

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/user/crawler-service/internal/entity"
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
// It increments the retry_count on conflict.
func (r *FailedURLRepoImpl) SaveOrUpdate(ctx context.Context, failedURL *entity.FailedURL) error {
	query := `
		INSERT INTO failed_urls (url, failure_reason, http_status_code, last_attempt_timestamp, retry_count, next_retry_at)
		VALUES ($1, $2, $3, $4, 1, $5)
		ON CONFLICT (url) DO UPDATE SET
			failure_reason = EXCLUDED.failure_reason,
			http_status_code = EXCLUDED.http_status_code,
			last_attempt_timestamp = EXCLUDED.last_attempt_timestamp,
			retry_count = failed_urls.retry_count + 1,
			next_retry_at = EXCLUDED.next_retry_at;
	`
	_, err := r.db.Exec(ctx, query,
		failedURL.URL,
		failedURL.FailureReason,
		failedURL.HTTPStatusCode,
		failedURL.LastAttemptTimestamp,
		failedURL.NextRetryAt,
	)
	return err
}

// FindRetryable retrieves a batch of URLs that are due for a retry.
func (r *FailedURLRepoImpl) FindRetryable(ctx context.Context, limit int) ([]*entity.FailedURL, error) {
	query := `
		SELECT id, url, failure_reason, http_status_code, last_attempt_timestamp, retry_count, next_retry_at
		FROM failed_urls
		WHERE next_retry_at <= NOW()
		ORDER BY next_retry_at ASC
		LIMIT $1;
	`
	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var failedURLs []*entity.FailedURL
	for rows.Next() {
		var fu entity.FailedURL
		if err := rows.Scan(
			&fu.ID,
			&fu.URL,
			&fu.FailureReason,
			&fu.HTTPStatusCode,
			&fu.LastAttemptTimestamp,
			&fu.RetryCount,
			&fu.NextRetryAt,
		); err != nil {
			return nil, err
		}
		failedURLs = append(failedURLs, &fu)
	}

	return failedURLs, rows.Err()
}

// Delete removes a failed URL record, typically after a successful crawl.
func (r *FailedURLRepoImpl) Delete(ctx context.Context, url string) error {
	query := `DELETE FROM failed_urls WHERE url = $1;`
	_, err := r.db.Exec(ctx, query, url)
	return err
}

