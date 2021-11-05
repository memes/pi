package pi

import (
	"context"

	"github.com/gomodule/redigo/redis"
)

// Cache defines an interface for a cache implementation that can be used to
// store the results of calcDigits for subsequent lookup requests.
type Cache interface {
	// Return the string that was set for key (or "" if unset) and an Error
	// if the implementation failed.
	// NOTE: a cache miss *should not* return an error.
	GetValue(ctx context.Context, key string) (string, error)
	// Store the value string with the provided key, returning an error if
	// the implementation failed.
	SetValue(ctx context.Context, key string, value string) error
}

// noopCache implements Cache interface without any real cacheing.
type noopCache struct{}

// Always returns an empty string and no error for every key.
func (n *noopCache) GetValue(ctx context.Context, key string) (string, error) {
	return "", nil
}

// Ignores the value and returns nil error.
func (n *noopCache) SetValue(ctx context.Context, key string, value string) error {
	return nil
}

// Creates a no-operation Cache implementation that satisfies the interface
// requirements without performing any real caching. All values are silently
// dropped by SetValue and calls to GetValue always return an empty string.
func NewNoopCache() *noopCache {
	return &noopCache{}
}

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
