package redis

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"os"
	"time"
)

func New() *redis.Client {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "redis:6379"
	}
	return redis.NewClient(&redis.Options{
		Addr:        addr,
		DialTimeout: 2 * time.Second,
		ReadTimeout: 2 * time.Second,
		DB:          0,
	})
}

func SetUserExistMarker(ctx context.Context, rdb *redis.Client, tgid string, ttl time.Duration) error {
	key := "user:" + tgid + ":exists"
	return rdb.SetEx(ctx, key, "1", ttl).Err()
}

func SaveUserSectionsByTitles(ctx context.Context, rdb *redis.Client, tgid string, titles []string, ttl time.Duration) error {
	if len(titles) == 0 {
		return nil
	}
	key := "user:" + tgid + ":sections"

	pipe := rdb.Pipeline()
	for _, title := range titles {
		idxKey := "section:title:" + title
		id, err := rdb.Get(ctx, idxKey).Result()
		if err == redis.Nil || id == "" {
			continue
		}
		if err != nil {
			return fmt.Errorf("GET %s: %w", idxKey, err)
		}
		pipe.SAdd(ctx, key, id)
	}
	pipe.Expire(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("pipeline SADD/EXPIRE %s: %w", key, err)
	}
	return nil
}
