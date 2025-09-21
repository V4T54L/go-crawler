package entity

import "time"

type CrawlStatus struct {
	URL                string
	CurrentStatus      string // "pending", "crawling", "completed", "failed", "not_found"
	LastCrawlTimestamp *time.Time
	NextRetryAt        *time.Time
	FailureReason      string
}

