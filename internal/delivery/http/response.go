package response

import "time"

type SubmitCrawlResponse struct {
	Status         string `json:"status"`
	Message        string `json:"message"`
	CrawlRequestID string `json:"crawl_request_id"`
}

// CrawlStatusResponse is a DTO for crawl status, mirroring entity.CrawlStatus
type CrawlStatusResponse struct {
	URL                string     `json:"url"`
	CurrentStatus      string     `json:"current_status"` // "pending", "crawling", "completed", "failed"
	LastCrawlTimestamp *time.Time `json:"last_crawl_timestamp,omitempty"`
	NextRetryAt        *time.Time `json:"next_retry_at,omitempty"`
	FailureReason      string     `json:"failure_reason,omitempty"`
}

