package redis

import (
	"context"
	"github.com/redis/go-redis/v9"
)

const crawlQueueKey = "crawler:queue"

// QueueRepoImpl provides a concrete implementation for the QueueRepository interface using Redis Lists.
type QueueRepoImpl struct {
	client *redis.Client
}

// NewQueueRepo creates a new instance of QueueRepoImpl.
func NewQueueRepo(client *redis.Client) *QueueRepoImpl {
	return &QueueRepoImpl{client: client}
}

// Push adds a URL to the left side of the Redis list (acting as a queue).
func (r *QueueRepoImpl) Push(ctx context.Context, url string) error {
	return r.client.LPush(ctx, crawlQueueKey, url).Err()
}

// Pop removes and returns a URL from the right side of the Redis list (acting as a queue).
// It is a blocking operation if the list is empty, but we can add a timeout.
// For simplicity, we use RPop which returns redis.Nil error if empty.
func (r *QueueRepoImpl) Pop(ctx context.Context) (string, error) {
	return r.client.RPop(ctx, crawlQueueKey).Result()
}

// Size returns the current number of items in the queue.
func (r *QueueRepoImpl) Size(ctx context.Context) (int64, error) {
	return r.client.LLen(ctx, crawlQueueKey).Result()
}

