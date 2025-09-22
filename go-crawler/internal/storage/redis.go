package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore handles interactions with Redis for caching and queues.
type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(addr string) *RedisStore {
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	return &RedisStore{client: rdb}
}

func (s *RedisStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

// MarkAsCrawled sets a key with a TTL to prevent re-crawling.
func (s *RedisStore) MarkAsCrawled(ctx context.Context, url string, ttl time.Duration) error {
	key := fmt.Sprintf("crawled:%s", url)
	return s.client.Set(ctx, key, "1", ttl).Err()
}

// IsRecentlyCrawled checks if a URL has been crawled within the TTL.
func (s *RedisStore) IsRecentlyCrawled(ctx context.Context, url string) (bool, error) {
	key := fmt.Sprintf("crawled:%s", url)
	val, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return val == 1, nil
}

// IncrementRetryCount increments the retry counter for a URL.
func (s *RedisStore) IncrementRetryCount(ctx context.Context, url string) (int64, error) {
	key := fmt.Sprintf("retry:%s", url)
	count, err := s.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	// Set an expiration on the retry key so it doesn't live forever
	s.client.Expire(ctx, key, 24*time.Hour)
	return count, nil
}
