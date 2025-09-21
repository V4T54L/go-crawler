package chromedp_crawler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/user/crawler-service/internal/entity"
	"github.com/user/crawler-service/internal/repository"
)

type ChromedpCrawler struct {
	allocatorPool *sync.Pool
	timeout       time.Duration
}

// NewChromedpCrawler creates a new crawler implementation using chromedp.
func NewChromedpCrawler(maxConcurrency int, pageLoadTimeout time.Duration) (repository.CrawlerRepository, error) {
	pool := &sync.Pool{
		New: func() interface{} {
			opts := append(chromedp.DefaultExecAllocatorOptions[:],
				chromedp.Flag("headless", true),
				chromedp.Flag("disable-gpu", true),
				chromedp.Flag("no-sandbox", true),
				chromedp.Flag("disable-dev-shm-usage", true),
				chromedp.UserAgent(`Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36`),
			)
			allocCtx, _ := chromedp.NewExecAllocator(context.Background(), opts...)
			return allocCtx
		},
	}

	// Pre-warm the pool
	for i := 0; i < maxConcurrency; i++ {
		allocCtx := pool.Get().(context.Context)
		pool.Put(allocCtx)
	}

	return &ChromedpCrawler{
		allocatorPool: pool,
		timeout:       pageLoadTimeout,
	}, nil
}

// Crawl fetches a URL and extracts data from it.
func (c *ChromedpCrawler) Crawl(ctx context.Context, url string) (*entity.ExtractedData, error) {
	// Get an allocator context from the pool
	allocCtx := c.allocatorPool.Get().(context.Context)
	defer c.allocatorPool.Put(allocCtx)

	// Create a new browser context from the allocator
	taskCtx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(slog.Debugf))
	defer cancel()

	// Create a timeout for the entire crawl task
	taskCtx, cancel = context.WithTimeout(taskCtx, c.timeout)
	defer cancel()

	var title string
	statusCode := 200 // Placeholder, a proper implementation would use network events.
	var responseTime int64

	startTime := time.Now()

	// For now, we only extract the title as a stub.
	// In a future step, this will be expanded to extract all required data.
	err := chromedp.Run(taskCtx,
		chromedp.Navigate(url),
		chromedp.Title(&title),
	)

	responseTime = time.Since(startTime).Milliseconds()

	if err != nil {
		slog.Error("Failed to crawl URL", "url", url, "error", err)
		return nil, err
	}

	slog.Info("Successfully crawled URL", "url", url, "title", title)

	// Stubbed data extraction
	data := &entity.ExtractedData{
		URL:            url,
		Title:          title,
		Description:    "",                   // Stub
		Keywords:       []string{},           // Stub
		H1Tags:         []string{},           // Stub
		Content:        "",                   // Stub
		Images:         []entity.ImageInfo{}, // Stub
		CrawlTimestamp: time.Now(),
		HTTPStatusCode: statusCode,
		ResponseTimeMS: int(responseTime),
	}

	return data, nil
}

