package domain

import "time"

// CrawlRequest is the payload for the API
type CrawlRequest struct {
	URLs       []string `json:"urls"`
	ForceCrawl bool     `json:"force_crawl"` // Bypass 2-day rule
}

// PageData holds the extracted information from a crawled page
type PageData struct {
	URL        string
	Title      string
	Content    string
	Headers    []string // e.g., H1, H2 tags
	MetaTags   map[string]string
	Images     []string
	Status     string // "completed", "failed", "processing"
	FailReason string
	CrawledAt  time.Time
}

// URLTask represents a single URL to be processed by a worker
type URLTask struct {
	URL        string
	ForceCrawl bool
}

// CrawlStatusResponse is the API response for a URL status query
type CrawlStatusResponse struct {
	URL        string    `json:"url"`
	Status     string    `json:"status"`
	FailReason string    `json:"fail_reason,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
}
