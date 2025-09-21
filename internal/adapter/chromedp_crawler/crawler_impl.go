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
	"github.com/user/crawler-service/pkg/utils"
)

var (
	userAgents = []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.1 Safari/605.1.15",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36",
	}

	viewports = []struct{ W, H int }{
		{1920, 1080},
		{1366, 768},
		{1536, 864},
		{2560, 1440},
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

	if last, ok := rl.lastRequest[domain]; ok {
		since := time.Since(last)
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
				chromedp.Flag("blink-settings", "imagesEnabled=false"), // Optionally disable images
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
		proxies:       proxies,
		rateLimiter:   newDomainRateLimiter(1 * time.Second), // Default 1s delay
	}, nil
}

func (c *ChromedpCrawler) getNextProxy() string {
	if len(c.proxies) == 0 {
		return ""
	}
	c.proxyMu.Lock()
	defer c.proxyMu.Unlock()
	proxy := c.proxies[c.proxyIndex]
	c.proxyIndex = (c.proxyIndex + 1) % len(c.proxies)
	return proxy
}

// Crawl fetches a URL and extracts data from it.
func (c *ChromedpCrawler) Crawl(ctx context.Context, rawURL string, sneaky bool) (*entity.ExtractedData, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}
	c.rateLimiter.Wait(parsedURL.Hostname())

	// Get an allocator context from the pool
	allocCtx := c.allocatorPool.Get().(context.Context)
	defer c.allocatorPool.Put(allocCtx)

	opts := []chromedp.ContextOption{chromedp.WithLogf(slog.Debugf)}
	if proxy := c.getNextProxy(); proxy != "" {
		opts = append(opts, chromedp.ProxyServer(proxy))
	}

	// Create a new browser context from the allocator
	taskCtx, cancel := chromedp.NewContext(allocCtx, opts...)
	defer cancel()

	// Create a timeout for the entire crawl task
	taskCtx, cancel = context.WithTimeout(taskCtx, c.timeout)
	defer cancel()

	var (
		title, description, keywords, content string
		h1s                                   []string
		images                                []entity.ImageInfo
		statusCode                            int64
		responseHeaders                       network.Headers
	)

	startTime := time.Now()

	// Listen for response to get status code
	chromedp.ListenTarget(taskCtx, func(ev interface{}) {
		if resp, ok := ev.(*network.EventResponseReceived); ok {
			if resp.Type == network.ResourceTypeDocument && resp.Response.URL == rawURL {
				statusCode = resp.Response.Status
				responseHeaders = resp.Response.Headers
			}
		}
	})

	actions := []chromedp.Action{
		network.Enable(),
		chromedp.Navigate(rawURL),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	}

	if sneaky {
		vp := viewports[rand.Intn(len(viewports))]
		actions = append(actions,
			chromedp.EmulateViewport(int64(vp.W), int64(vp.H)),
			network.SetExtraHTTPHeaders(network.Headers{
				"User-Agent": userAgents[rand.Intn(len(userAgents))],
				"Referer":    "https://www.google.com/",
			}),
		)
	}

	// Extraction tasks
	extractionTasks := chromedp.Tasks{
		chromedp.Title(&title),
		chromedp.AttributeValue(`meta[name="description"]`, "content", &description, nil),
		chromedp.AttributeValue(`meta[name="keywords"]`, "content", &keywords, nil),
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
			var contentBuilder strings.Builder
			for _, node := range pNodes {
				var text string
				if err := chromedp.Text(node.NodeValue, &text, chromedp.ByNodeID).Do(ctx); err != nil {
					slog.Warn("failed to get text for p node", "url", rawURL, "error", err)
					continue
				}
				if text != "" {
					contentBuilder.WriteString(strings.TrimSpace(text))
					contentBuilder.WriteString("\n")
				}
			}
			content = contentBuilder.String()
			return nil
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var imgNodes []*cdp.Node
			if err := chromedp.Nodes(`img`, &imgNodes, chromedp.ByQueryAll).Do(ctx); err != nil {
				return err
			}
			for _, node := range imgNodes {
				attrs, err := chromedp.Attributes(node.NodeValue, chromedp.ByNodeID).Do(ctx)
				if err != nil {
					continue
				}
				var img entity.ImageInfo
				for i := 0; i < len(attrs); i += 2 {
					switch attrs[i] {
					case "src":
						absSrc, _ := utils.ToAbsoluteURL(parsedURL, attrs[i+1])
						img.Src = absSrc
					case "alt":
						img.Alt = attrs[i+1]
					case "data-src":
						absDataSrc, _ := utils.ToAbsoluteURL(parsedURL, attrs[i+1])
						img.DataSrc = absDataSrc
					}
				}
				if img.Src != "" || img.DataSrc != "" {
					images = append(images, img)
				}
			}
			return nil
		}),
	}

	actions = append(actions, extractionTasks...)

	if err := chromedp.Run(taskCtx, actions...); err != nil {
		slog.Error("Failed to crawl URL", "url", rawURL, "error", err)
		return nil, fmt.Errorf("chromedp run failed: %w", err)
	}

	responseTime := time.Since(startTime)

	if statusCode == 0 {
		return nil, errors.New("failed to capture main document response")
	}
	if statusCode >= 400 {
		return nil, fmt.Errorf("received non-success status code: %d", statusCode)
	}

	slog.Info("Successfully crawled URL", "url", rawURL, "title", title, "status", statusCode, "duration_ms", responseTime.Milliseconds())

	// Stubbed data extraction
	data := &entity.ExtractedData{
		URL:            rawURL,
		Title:          title,
		Description:    description,
		Keywords:       strings.Split(keywords, ","),
		H1Tags:         h1s,
		Content:        content,
		Images:         images,
		CrawlTimestamp: time.Now(),
		HTTPStatusCode: int(statusCode),
		ResponseTimeMS: int(responseTime.Milliseconds()),
	}

	_ = responseHeaders // Can be used for rate limiting headers later

	return data, nil
}

