package pi

import (
	"context"
	// spell-checker: ignore gomodule redigo
	"github.com/gomodule/redigo/redis"
	"go.uber.org/zap"
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
	l := logger.With(
		zap.String("key", key),
	)
	l.Debug("GetValue: enter")
	conn := r.Get()
	defer conn.Close()

	value, err := redis.String(conn.Do("GET", key))
	if err != nil && err == redis.ErrNil {
		l.Info("Value is not cached",
			zap.Error(err),
		)
		return "", nil
	}
	if err != nil {
		l.Error("Error returned from Redis cache",
			zap.Error(err),
		)
		return "", err
	}
	l.Debug("GetValue: exit",
		zap.String("value", value),
	)
	return value, nil
}

func (r *redisCache) SetValue(ctx context.Context, key string, value string) error {
	l := logger.With(
		zap.String("key", key),
		zap.String("value", value),
	)
	l.Debug("SetValue: enter")
	conn := r.Get()
	defer conn.Close()

	_, err := conn.Do("SET", key, value)
	if err != nil {
		logger.Error("Error writing to Redis cache",
			zap.Error(err),
		)
		return err
	}
	return nil
}
