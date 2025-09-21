package redis

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"github.com/user/crawler-service/pkg/utils"
	"time"
)

const visitedURLPrefix = "visited:"

// VisitedRepoImpl provides a concrete implementation for the VisitedRepository interface using Redis.
type VisitedRepoImpl struct {
	client *redis.Client
}

// NewVisitedRepo creates a new instance of VisitedRepoImpl.
func NewVisitedRepo(client *redis.Client) *VisitedRepoImpl {
	return &VisitedRepoImpl{client: client}
}

// generateKey creates a consistent Redis key for a given URL by hashing it.
func (r *VisitedRepoImpl) generateKey(url string) string {
	return fmt.Sprintf("%s%s", visitedURLPrefix, utils.HashURL(url))
}

// MarkVisited marks a URL as visited by setting a key in Redis with a specific expiry time.
func (r *VisitedRepoImpl) MarkVisited(ctx context.Context, url string, expiry time.Duration) error {
	key := r.generateKey(url)
	// SETEX is atomic and sets the key with an expiry.
	return r.client.SetEX(ctx, key, "1", expiry).Err()
}

// IsVisited checks if a URL has been visited recently by checking for the existence of its key.
func (r *VisitedRepoImpl) IsVisited(ctx context.Context, url string) (bool, error) {
	key := r.generateKey(url)
	// EXISTS returns 1 if the key exists, 0 otherwise.
	val, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return val == 1, nil
}

// RemoveVisited removes a URL from the visited set, used for force_crawl.
func (r *VisitedRepoImpl) RemoveVisited(ctx context.Context, url string) error {
	key := r.generateKey(url)
	// DEL removes the key.
	return r.client.Del(ctx, key).Err()
}

