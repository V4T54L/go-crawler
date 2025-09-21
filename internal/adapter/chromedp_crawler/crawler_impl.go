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
				chromedp.Flag("blink-settings", "imagesEnabled=false"), // Kept from original
			)
			allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...) // Adopted cancel from attempted
			return context.CancelFunc(func() { // Adopted context.CancelFunc for pool management
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
	domain := parsedURL.Hostname() // Adopted from attempted
	c.rateLimiter.Wait(domain)

	// Get an allocator context from the pool
	allocatorCtx, cancelAllocator := c.allocatorPool.Get().(context.CancelFunc) // Adopted pool management
	defer c.allocatorPool.Put(cancelAllocator)                                   // Adopted pool management

	// Create a new browser context from the allocator
	browserCtx, cancelBrowser := chromedp.NewContext(context.Background(), chromedp.WithLogf(slog.Debugf)) // Original approach for browser context
	defer cancelBrowser()

	// Create a timeout for the entire crawl task
	taskCtx, cancelTask := context.WithTimeout(browserCtx, c.timeout) // Original approach for task timeout
	defer cancelTask()

	var (
		title, description, content string
		keywords                    []string // Changed to slice for keywords
		h1s                         []string
		images                      []*cdp.Node // Adopted from attempted for image nodes
		statusCode                  int64
		finalURL                    string // Adopted from attempted
	)

	startTime := time.Now()

	// Listen for network responses to capture status code and final URL
	// Adopted from attempted content for more robust status/final URL capture
	listenCtx, cancelListen := context.WithCancel(taskCtx)
	defer cancelListen()

	chromedp.ListenTarget(listenCtx, func(ev interface{}) {
		if resp, ok := ev.(*network.EventResponseReceived); ok {
			if resp.Type == network.ResourceTypeDocument {
				// Capture the status code of the main document request
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

	if proxy := c.getNextProxy(); proxy != "" {
		actions = append(actions, chromedp.ProxyServer(proxy)) // Proxy added as an action
	}

	if sneaky {
		vp := viewports[rand.Intn(len(viewports))]
		ua := userAgents[rand.Intn(len(userAgents))] // Adopted from attempted
		actions = append(actions,
			chromedp.EmulateViewport(int64(vp.W), int64(vp.H)),
			network.SetExtraHTTPHeaders(network.Headers{ // Kept from original
				"User-Agent": ua,
				"Referer":    "https://www.google.com/",
			}),
			chromedp.UserAgent(ua), // Adopted from attempted
		)
	}

	actions = append(actions,
		chromedp.Navigate(rawURL),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Title(&title),
		chromedp.Location(&finalURL), // Fallback for final URL, adopted from attempted
		chromedp.AttributeValue(`meta[name="description"]`, "content", &description, nil),
		chromedp.AttributeValue(`meta[name="keywords"]`, "content", &keywords, nil), // Changed to slice
		chromedp.ActionFunc(func(ctx context.Context) error { // Kept original H1 extraction
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
		chromedp.ActionFunc(func(ctx context.Context) error { // Kept original P content extraction
			var pNodes []*cdp.Node
			if err := chromedp.Nodes(`p`, &pNodes, chromedp.ByQueryAll).Do(ctx); err != nil {
				return err
				// If we want to concatenate all p tags into a single string, we can do this:
				// var contentBuilder strings.Builder
				// for _, node := range pNodes {
				// 	var text string
				// 	if err := chromedp.Text(node.NodeValue, &text, chromedp.ByNodeID).Do(ctx); err != nil {
				// 		slog.Warn("failed to get text for p node", "url", rawURL, "error", err)
				// 		continue
				// 	}
				// 	if text != "" {
				// 		contentBuilder.WriteString(strings.TrimSpace(text))
				// 		contentBuilder.WriteString("\n")
				// 	}
				// }
				// content = contentBuilder.String()
			}
			// For now, let's just get the text of the first p tag or concatenate them.
			// The attempted content uses chromedp.Text(`p`, &content, chromedp.ByQueryAll) which concatenates.
			// Let's adopt that simpler concatenation for 'content'.
			if err := chromedp.Text(`p`, &content, chromedp.ByQueryAll).Do(ctx); err != nil {
				slog.Warn("failed to get text for p tags", "url", rawURL, "error", err)
			}
			return nil
		}),
		chromedp.Nodes(`img`, &images, chromedp.ByQueryAll), // Adopted from attempted for image nodes
	)

	if err := chromedp.Run(taskCtx, actions...); err != nil {
		if errors.Is(err, context.DeadlineExceeded) { // Adopted specific error handling
			return nil, fmt.Errorf("%w: %v", repository.ErrCrawlTimeout, err)
		}
		if strings.Contains(err.Error(), "net::") { // Adopted specific error handling
			return nil, fmt.Errorf("%w: %v", repository.ErrNavigationFailed, err)
		}
		slog.Error("Chromedp run failed", "url", rawURL, "error", err)
		return nil, fmt.Errorf("%w: %v", repository.ErrExtractionFailed, err) // Adopted specific error handling
	}

	responseTime := time.Since(startTime)

	// If listener didn't catch status, it might be a cached response or other issue.
	// We can consider this a partial success or failure. For now, let's mark as 200 if successful.
	if statusCode == 0 { // Adopted from attempted
		slog.Warn("Could not determine status code from network events, assuming 200", "url", rawURL)
		statusCode = 200
	}

	if statusCode >= 400 && statusCode < 500 { // Adopted from attempted
		return nil, fmt.Errorf("%w: received status code %d", repository.ErrContentRestricted, statusCode)
	}
	if statusCode >= 500 { // Adopted from attempted
		return nil, fmt.Errorf("%w: received status code %d", repository.ErrNavigationFailed, statusCode)
	}

	slog.Info("Successfully crawled URL", "url", rawURL, "title", title, "status", statusCode, "duration_ms", responseTime.Milliseconds())

	data := &entity.ExtractedData{
		URL:            rawURL, // Store original URL
		Title:          title,
		Description:    description,
		H1Tags:         h1s,
		Content:        content,
		CrawlTimestamp: time.Now(),
		HTTPStatusCode: int(statusCode),
		ResponseTimeMS: int(responseTime.Milliseconds()),
	}

	if len(keywords) > 0 { // Adopted from attempted for keyword processing
		data.Keywords = strings.Split(keywords[0], ",")
		for i := range data.Keywords {
			data.Keywords[i] = strings.TrimSpace(data.Keywords[i])
		}
	}

	base, _ := url.Parse(finalURL) // Use finalURL for base, adopted from attempted
	for _, imgNode := range images {
		src, _ := imgNode.Attribute("src")
		dataSrc, _ := imgNode.Attribute("data-src")
		alt, _ := imgNode.Attribute("alt")

		absSrc, _ := utils.ToAbsoluteURL(base, src)
		absDataSrc, _ := utils.ToAbsoluteURL(base, dataSrc)

		if absSrc != "" || absDataSrc != "" {
			data.Images = append(data.Images, entity.ImageInfo{
				Src:     absSrc,
				Alt:     alt,
				DataSrc: absDataSrc,
			})
		}
	}

	return data, nil
}

