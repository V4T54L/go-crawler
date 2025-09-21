package repository

import (
	"context"
	"errors" // Added from attempted content
	"github.com/user/crawler-service/internal/entity"
)

// Adopted specific error definitions from attempted content
var (
	ErrCrawlTimeout      = errors.New("crawl operation timed out")
	ErrNavigationFailed  = errors.New("navigation to URL failed")
	ErrExtractionFailed  = errors.New("data extraction failed")
	ErrContentRestricted = errors.New("content is restricted or requires authentication")
)

// CrawlerRepository defines the interface for the actual crawling component.
type CrawlerRepository interface {
	// Crawl fetches the given URL, renders it, and extracts structured data.
	// The 'sneaky' flag indicates whether to use anti-bot evasion techniques.
	Crawl(ctx context.Context, url string, sneaky bool) (*entity.ExtractedData, error)
}

