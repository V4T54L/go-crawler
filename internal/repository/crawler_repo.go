package repository

import (
	"context"
	"github.com/user/crawler-service/internal/entity"
)

// CrawlerRepository defines the contract for the actual web page crawling mechanism.
type CrawlerRepository interface {
	// Crawl fetches a URL and extracts data from it.
	Crawl(ctx context.Context, url string, sneaky bool) (*entity.ExtractedData, error)
}

