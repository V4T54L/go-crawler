package repository

import "context"

// QueueRepository defines the interface for a FIFO queue for URLs to be crawled.
type QueueRepository interface {
	// Push adds a URL to the end of the queue.
	Push(ctx context.Context, url string) error
	// Pop removes and returns a URL from the front of the queue.
	Pop(ctx context.Context) (string, error)
	// Size returns the current number of items in the queue.
	Size(ctx context.Context) (int64, error)
}

