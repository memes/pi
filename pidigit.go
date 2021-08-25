package pi

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

var (
	logger       = zap.NewNop()
	cache  Cache = NewNoopCache()
)

func SetLogger(l *zap.Logger) {
	if l != nil {
		logger = l
	}
}

func SetCache(c Cache) {
	if c != nil {
		cache = c
	}
}

func PiDigit(ctx context.Context, n uint64) (string, error) {
	l := logger.With(
		zap.Uint64("n", n),
	)
	l.Debug("PiDigits: enter")
	index := uint64(n/9) * 9
	key := fmt.Sprintf("%d", index)
	digits, err := cache.GetValue(ctx, key)
	if err != nil {
		logger.Error("Error retrieving digits from cache",
			zap.Error(err),
		)
		return "", err
	}
	if digits == "" {
		digits = CalcDigits(index)
		err = cache.SetValue(ctx, key, digits)
		if err != nil {
			logger.Error("Error writing digits to cache",
				zap.Error(err),
			)
			return "", err
		}
	}
	digit := string(digits[n%9])
	logger.Debug("PiDigit: exit",
		zap.String("digit", digit),
	)
	return digit, nil
}
