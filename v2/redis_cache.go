package pi

import (
	"context"

	"github.com/gomodule/redigo/redis"
)

// redisCache implements Cache interface with a Redis store.
type redisCache struct {
	*redis.Pool
}

// Return a new Cache implementation using Redis
func NewRedisCache(ctx context.Context, endpoint string) *redisCache {
	return &redisCache{
		&redis.Pool{
			DialContext: func(ctx context.Context) (redis.Conn, error) {
				return redis.DialContext(ctx, "tcp", endpoint)
			},
		},
	}
}

// Returns the string value stored in Redis under key, if present, or an empty string.
func (r *redisCache) GetValue(ctx context.Context, key string) (string, error) {
	l := logger.V(0).WithValues("key", key)
	l.Info("GetValue: enter")
	conn := r.Get()
	defer conn.Close()

	value, err := redis.String(conn.Do("GET", key))
	if err == redis.ErrNil {
		l.Info("Value is not cached")
		return "", nil
	}
	if err != nil {
		return "", err
	}
	l.Info("GetValue: exit", "value", value)
	return value, nil
}

// Store the string key:value pair in Redis.
func (r *redisCache) SetValue(ctx context.Context, key string, value string) error {
	l := logger.V(0).WithValues("key", key, "value", value)
	l.Info("SetValue: enter")
	conn := r.Get()
	defer conn.Close()

	_, err := conn.Do("SET", key, value)
	if err != nil {
		return err
	}
	return nil
}
