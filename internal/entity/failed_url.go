package entity

import "time"

// FailedURL mirrors the `failed_urls` PostgreSQL table schema.
type FailedURL struct {
	ID                   int64
	URL                  string
	FailureReason        string
	HTTPStatusCode       int
	LastAttemptTimestamp time.Time
	RetryCount           int
	NextRetryAt          time.Time
}

