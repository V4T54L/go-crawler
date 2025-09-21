package request

type SubmitCrawlRequest struct {
	URL        string `json:"url"`
	ForceCrawl bool   `json:"force_crawl"`
	CrawlMode  string `json:"crawl_mode"` // "respectful" or "sneaky" - not used in this step
}

