package crawler

import (
	"context"
	"crawler/internal/config"
	"crawler/internal/domain"
	"crawler/internal/monitoring"
	"crawler/internal/proxy"
	"crawler/internal/storage"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"go.uber.org/zap"
)

// Crawler manages the worker pool and crawling tasks.
type Crawler struct {
	config       *config.Config
	redisStore   *storage.RedisStore
	pgStore      *storage.PostgresStore
	proxyManager *proxy.Manager
	metrics      *monitoring.Metrics
	logger       *zap.Logger
	taskQueue    chan domain.URLTask
	stopChan     chan struct{}
	wg           sync.WaitGroup
	ctxPool      sync.Pool
}

func NewCrawler(cfg *config.Config, rs *storage.RedisStore, ps *storage.PostgresStore, pm *proxy.Manager, m *monitoring.Metrics, l *zap.Logger) *Crawler {
	c := &Crawler{
		config:       cfg,
		redisStore:   rs,
		pgStore:      ps,
		proxyManager: pm,
		metrics:      m,
		logger:       l,
		taskQueue:    make(chan domain.URLTask, cfg.CrawlWorkers*2),
		stopChan:     make(chan struct{}),
	}
	c.ctxPool.New = func() interface{} {
		opts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("no-sandbox", ""),
			chromedp.Flag("disable-dev-shm-usage", ""),
		)
		allocCtx, _ := chromedp.NewExecAllocator(context.Background(), opts...)
		return allocCtx
	}
	return c
}

func (c *Crawler) Start() {
	for i := 0; i < c.config.CrawlWorkers; i++ {
		c.wg.Add(1)
		go c.worker()
	}
}

func (c *Crawler) Stop() {
	close(c.stopChan)
	close(c.taskQueue)
	c.wg.Wait()
}

func (c *Crawler) Submit(task domain.URLTask) {
	c.taskQueue <- task
}

func (c *Crawler) worker() {
	defer c.wg.Done()
	for {
		select {
		case task, ok := <-c.taskQueue:
			if !ok {
				return // Channel closed
			}
			c.processURL(task)
		case <-c.stopChan:
			return
		}
	}
}

func (c *Crawler) processURL(task domain.URLTask) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.config.CrawlTimeout+10)*time.Second)
	defer cancel()

	if !task.ForceCrawl {
		isCrawled, err := c.redisStore.IsRecentlyCrawled(ctx, task.URL)
		if err != nil {
			c.logger.Error("failed to check redis for crawled status", zap.String("url", task.URL), zap.Error(err))
		}
		if isCrawled {
			c.logger.Info("skipping recently crawled URL", zap.String("url", task.URL))
			return
		}
	}

	// Mark as processing in DB
	processingData := &domain.PageData{URL: task.URL, Status: "processing"}
	if err := c.pgStore.SaveData(ctx, processingData); err != nil {
		c.logger.Error("failed to mark URL as processing", zap.String("url", task.URL), zap.Error(err))
	}

	allocCtx := c.ctxPool.Get().(context.Context)
	taskCtx, taskCancel := chromedp.NewContext(allocCtx)
	taskCtx, _ = context.WithTimeout(taskCtx, time.Duration(c.config.CrawlTimeout)*time.Second)
	defer taskCancel()
	defer c.ctxPool.Put(allocCtx)

	var htmlContent string
	err := chromedp.Run(taskCtx,
		chromedp.Navigate(task.URL),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.OuterHTML("html", &htmlContent),
	)

	c.metrics.IncCrawledTotal()

	if err != nil {
		c.handleFailure(ctx, task.URL, err)
		return
	}

	pageData, err := ExtractPageData(task.URL, htmlContent)
	if err != nil {
		c.handleFailure(ctx, task.URL, err)
		return
	}

	pageData.CrawledAt = time.Now()
	if err := c.pgStore.SaveData(ctx, pageData); err != nil {
		c.logger.Error("error saving data", zap.String("url", task.URL), zap.Error(err))
		c.metrics.IncErrorsTotal("db_save_failed")
	} else {
		c.logger.Info("successfully crawled and saved", zap.String("url", task.URL))
		ttl := time.Duration(c.config.DeduplicationDays) * 24 * time.Hour
		c.redisStore.MarkAsCrawled(ctx, task.URL, ttl)
	}
}

func (c *Crawler) handleFailure(ctx context.Context, url string, crawlErr error) {
	c.logger.Warn("failed to crawl", zap.String("url", url), zap.Error(crawlErr))
	c.metrics.IncErrorsTotal("crawl_failed")

	retryCount, err := c.redisStore.IncrementRetryCount(ctx, url)
	if err != nil {
		c.logger.Error("failed to increment retry count", zap.String("url", url), zap.Error(err))
		return
	}

	if retryCount >= int64(c.config.MaxRetries) {
		c.logger.Error("max retries reached, marking as failed", zap.String("url", url))
		failedData := &domain.PageData{
			URL:        url,
			Status:     "failed",
			FailReason: crawlErr.Error(),
			CrawledAt:  time.Now(),
		}
		if err := c.pgStore.SaveData(ctx, failedData); err != nil {
			c.logger.Error("failed to mark URL as failed in db", zap.String("url", url), zap.Error(err))
		}
	} else {
		c.logger.Info("URL will be retried later", zap.String("url", url), zap.Int64("attempt", retryCount))
		// For a more robust retry, add it to a delayed queue (e.g., Redis ZSET)
	}
}
