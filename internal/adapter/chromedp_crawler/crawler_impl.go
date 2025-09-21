package chromedp_crawler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/user/crawler-service/internal/entity"
	"github.com/user/crawler-service/internal/repository"
)

var (
	userAgents = []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.1 Safari/605.1.15", // From original
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36", // From attempted
	}

	viewports = []struct{ W, H int }{
		{1920, 1080},
		{1366, 768},
		{1536, 864},
		{2560, 1440}, // From original
	}
)

type domainRateLimiter struct {
	lastRequest map[string]time.Time
	delay       time.Duration
	mu          sync.Mutex
}

func newDomainRateLimiter(delay time.Duration) *domainRateLimiter {
	return &domainRateLimiter{
		lastRequest: make(map[string]time.Time),
		delay:       delay,
	}
}

func (rl *domainRateLimiter) Wait(domain string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if last, ok := rl.lastRequest[domain]; ok { // Kept original's 'ok'
		since := time.Since(last) // Kept original's 'since'
		if since < rl.delay {
			time.Sleep(rl.delay - since)
		}
	}
	rl.lastRequest[domain] = time.Now()
}

type ChromedpCrawler struct {
	allocatorPool *sync.Pool
	timeout       time.Duration
	proxies       []string
	proxyIndex    int
	proxyMu       sync.Mutex
	rateLimiter   *domainRateLimiter
}

// NewChromedpCrawler creates a new crawler implementation using chromedp.
func NewChromedpCrawler(maxConcurrency int, pageLoadTimeout time.Duration, proxies []string) (repository.CrawlerRepository, error) {
	pool := &sync.Pool{
		New: func() interface{} {
			opts := append(chromedp.DefaultExecAllocatorOptions[:],
				chromedp.Flag("headless", true),
				chromedp.Flag("disable-gpu", true),
				chromedp.Flag("no-sandbox", true),
				chromedp.Flag("disable-dev-shm-usage", true),
				chromedp.Flag("blink-settings", "imagesEnabled=false"),
			)

			// Apply proxy if available
			if proxy := getNextProxy(proxies); proxy != "" {
				opts = append(opts, chromedp.ProxyServer(proxy))
			}

			allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
			return context.CancelFunc(func() {
				cancel()
				chromedp.Cancel(allocCtx)
			})
		},
	}

	// Pre-warm the pool
	for i := 0; i < maxConcurrency; i++ {
		pool.Put(pool.New()) // Adopted simpler pre-warming
	}

	return &ChromedpCrawler{
		allocatorPool: pool,
		timeout:       pageLoadTimeout,
		proxies:       proxies,
		rateLimiter:   newDomainRateLimiter(1 * time.Second), // Default 1s delay
	}, nil
}

func getNextProxy(proxies []string) string {
	if len(proxies) == 0 {
		return ""
	}
	return proxies[rand.Intn(len(proxies))]
}

// Crawl fetches a URL and extracts data from it.
func (c *ChromedpCrawler) Crawl(ctx context.Context, rawURL string, sneaky bool) (*entity.ExtractedData, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}
	domain := parsedURL.Hostname()
	c.rateLimiter.Wait(domain)

	// _, cancelAllocator := c.allocatorPool.Get().(context.CancelFunc)
	// defer cancelAllocator()

	browserCtx, cancelBrowser := chromedp.NewContext(context.Background(), chromedp.WithLogf(slog.Debug))
	defer cancelBrowser()

	taskCtx, cancelTask := context.WithTimeout(browserCtx, c.timeout)
	defer cancelTask()

	var (
		title, description, content string
		keywords                    []string // Changed to slice for keywords
		h1s                         []string
		images                      []*cdp.Node
		statusCode                  int64
		finalURL                    string
	)

	startTime := time.Now()

	listenCtx, cancelListen := context.WithCancel(taskCtx)
	defer cancelListen()

	chromedp.ListenTarget(listenCtx, func(ev interface{}) {
		if resp, ok := ev.(*network.EventResponseReceived); ok {
			if resp.Type == network.ResourceTypeDocument {
				if statusCode == 0 {
					statusCode = resp.Response.Status
					finalURL = resp.Response.URL
					slog.Debug("Captured response", "url", rawURL, "final_url", finalURL, "status", statusCode)
				}
			}
		}
	})

	actions := []chromedp.Action{
		network.Enable(),
	}

	if sneaky {
		vp := viewports[rand.Intn(len(viewports))]
		ua := userAgents[rand.Intn(len(userAgents))]
		actions = append(actions,
			chromedp.EmulateViewport(int64(vp.W), int64(vp.H)),
			network.SetExtraHTTPHeaders(network.Headers{
				"User-Agent": ua,
				"Referer":    "https://www.google.com/",
			}),
		)
	}

	actions = append(actions,
		chromedp.Navigate(rawURL),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Title(&title),
		chromedp.Location(&finalURL),
		chromedp.AttributeValue(`meta[name="description"]`, "content", &description, nil),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var h1Nodes []*cdp.Node
			if err := chromedp.Nodes(`h1`, &h1Nodes, chromedp.ByQueryAll).Do(ctx); err != nil {
				return err
			}
			for _, node := range h1Nodes {
				var text string
				if err := chromedp.Text(node.NodeValue, &text, chromedp.ByNodeID).Do(ctx); err != nil {
					slog.Warn("failed to get text for h1 node", "url", rawURL, "error", err)
					continue
				}
				if text != "" {
					h1s = append(h1s, strings.TrimSpace(text))
				}
			}
			return nil
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var pNodes []*cdp.Node
			if err := chromedp.Nodes(`p`, &pNodes, chromedp.ByQueryAll).Do(ctx); err != nil {
				return err
			}
			if err := chromedp.Text(`p`, &content, chromedp.ByQueryAll).Do(ctx); err != nil {
				slog.Warn("failed to get text for p tags", "url", rawURL, "error", err)
			}
			return nil
		}),
		chromedp.Nodes(`img`, &images, chromedp.ByQueryAll),
	)

	if err := chromedp.Run(taskCtx, actions...); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("%w: %v", repository.ErrCrawlTimeout, err)
		}
		if strings.Contains(err.Error(), "net::") {
			return nil, fmt.Errorf("%w: %v", repository.ErrNavigationFailed, err)
		}
		slog.Error("Chromedp run failed", "url", rawURL, "error", err)
		return nil, fmt.Errorf("%w: %v", repository.ErrExtractionFailed, err)
	}

	responseTime := time.Since(startTime)

	if statusCode == 0 {
		slog.Warn("Could not determine status code from network events, assuming 200", "url", rawURL)
		statusCode = 200
	}

	if statusCode >= 400 && statusCode < 500 {
		slog.Warn("Client error while crawling", "url", rawURL, "status_code", statusCode)
		return nil, fmt.Errorf("client error %d for URL %s", statusCode, rawURL)
	}

	if statusCode >= 500 {
		slog.Warn("Server error while crawling", "url", rawURL, "status_code", statusCode)
		return nil, fmt.Errorf("server error %d for URL %s", statusCode, rawURL)
	}

	var keywordsStr string
	err = chromedp.AttributeValue(`meta[name="keywords"]`, "content", &keywordsStr, nil).Do(taskCtx)
	if err != nil {
		return nil, err
	}

	if keywordsStr != "" {
		keywords = strings.Split(keywordsStr, ",")
		for i := range keywords {
			keywords[i] = strings.TrimSpace(keywords[i])
		}
	}

	return &entity.ExtractedData{
		Title:       title,
		Description: description,
		Keywords:    keywords,
		H1Tags:      h1s,
		// Images:         images[0],
		Images:         []entity.ImageInfo{},
		Content:        content,
		HTTPStatusCode: int(statusCode),
		URL:            finalURL,
		ResponseTimeMS: int(responseTime),
	}, nil
}
