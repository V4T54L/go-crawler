package repository

import (
	"context"
	"time"
)

// VisitedRepository defines the interface for deduplication of visited URLs.
type VisitedRepository interface {
	// MarkVisited marks a URL as visited with a specific expiry time.
	MarkVisited(ctx context.Context, url string, expiry time.Duration) error
	// IsVisited checks if a URL has been visited recently.
	IsVisited(ctx context.Context, url string) (bool, error)
	// RemoveVisited removes a URL from the visited set, used for force_crawl.
	RemoveVisited(ctx context.Context, url string) error
}
```
