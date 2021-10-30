package pi

import (
	"context"
	// spell-checker: ignore gomodule redigo
	"github.com/gomodule/redigo/redis"
)

type redisCache struct {
	*redis.Pool
}

func NewRedisCache(ctx context.Context, address string) *redisCache {
	return &redisCache{
		&redis.Pool{
			DialContext: func(ctx context.Context) (redis.Conn, error) {
				return redis.DialContext(ctx, "tcp", address)
			},
		},
	}
}

func (r *redisCache) GetValue(ctx context.Context, key string) (string, error) {
	l := logger.V(1).WithValues("key", key)
	l.Info("GetValue: enter")
	conn := r.Get()
	defer conn.Close()

	value, err := redis.String(conn.Do("GET", key))
	if err != nil && err == redis.ErrNil {
		l.Info("Value is not cached")
		return "", nil
	}
	if err != nil {
		l.Error(err, "Error returned from Redis cache")
		return "", err
	}
	l.Info("GetValue: exit", "value", value)
	return value, nil
}

func (r *redisCache) SetValue(ctx context.Context, key string, value string) error {
	l := logger.WithValues("key", key, "value", value)
	l.Info("SetValue: enter")
	conn := r.Get()
	defer conn.Close()

	_, err := conn.Do("SET", key, value)
	if err != nil {
		logger.Error(err, "Error writing to Redis cache")
		return err
	}
	return nil
}
