// Package cache defines a common interface for cache implementations that can
// be used by PiService implementations.
package cache

import (
	"context"

	"github.com/gomodule/redigo/redis"
)

// Cache defines an interface for a cache implementation that can be used to
// store the results of a calculation for subsequent lookup requests.
type Cache interface {
	// Return the string that was set for key (or "" if unset) and an Error
	// if the implementation failed.
	// NOTE: a cache miss *should not* return an error.
	GetValue(ctx context.Context, key string) (string, error)
	// Store the value string with the provided key, returning an error if
	// the implementation failed.
	SetValue(ctx context.Context, key string, value string) error
}

// NoopCache implements Cache interface without any real cacheing.
type NoopCache struct{}

// Always returns an empty string and no error for every key.
func (n *NoopCache) GetValue(ctx context.Context, key string) (string, error) {
	return "", nil
}

// Ignores the value and returns nil error.
func (n *NoopCache) SetValue(ctx context.Context, key string, value string) error {
	return nil
}

// Creates a no-operation Cache implementation that satisfies the interface
// requirements without performing any real caching. All values are silently
// dropped by SetValue and calls to GetValue always return an empty string.
func NewNoopCache() *NoopCache {
	return &NoopCache{}
}

// RedisCache implements Cache interface backed by a Redis store.
type RedisCache struct {
	*redis.Pool
}

type RedisCacheOption func(*RedisCache)

// Return a new Cache implementation using Redis
func NewRedisCache(ctx context.Context, endpoint string, options ...RedisCacheOption) *RedisCache {
	cache := &RedisCache{
		&redis.Pool{
			DialContext: func(ctx context.Context) (redis.Conn, error) {
				return redis.DialContext(ctx, "tcp", endpoint)
			},
		},
	}
	for _, option := range options {
		option(cache)
	}
	return cache
}

// Returns the string value stored in Redis under key, if present, or an empty string.
func (r *RedisCache) GetValue(ctx context.Context, key string) (string, error) {
	conn := r.Get()
	defer conn.Close()

	value, err := redis.String(conn.Do("GET", key))
	if err == redis.ErrNil {
		// A cache miss is *NOT* an error to propagate
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

// Store the string key:value pair in Redis.
func (r *RedisCache) SetValue(ctx context.Context, key string, value string) error {
	conn := r.Get()
	defer conn.Close()
	_, err := conn.Do("SET", key, value)
	if err != nil {
		return err
	}
	return nil
}
