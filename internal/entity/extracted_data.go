package entity

import "time"

// ImageInfo represents the structured data for an image extracted from a page.
type ImageInfo struct {
	Src     string `json:"src"`
	Alt     string `json:"alt"`
	DataSrc string `json:"data_src,omitempty"` // For lazy-loaded images
}

// ExtractedData mirrors the `extracted_data` PostgreSQL table schema.
type ExtractedData struct {
	ID               int64
	URL              string
	Title            string
	Description      string
	Keywords         []string
	H1Tags           []string
	Content          string
	Images           []ImageInfo // Stored as JSONB in PostgreSQL
	CrawlTimestamp   time.Time
	HTTPStatusCode   int
	ResponseTimeMS   int
}

