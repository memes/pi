package pi

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// Defines the signature of a function that will return the next largest prime
// number that is greater than the supplied value.
type FindNextPrimeFunc func(uint64) uint64

var (
	// Zap logger to use in this package; default is a no-op logger.
	logger = zap.NewNop()
	// Cache implementation to use; default is a no-op cache.
	cache Cache = NewNoopCache()
	// The next prime function to use in this package; default is a naive
	// brute-force calculator.
	findNextPrime FindNextPrimeFunc = BruteFindNextPrime
)

// Change the Zap logger instance used by this package.
func SetLogger(l *zap.Logger) {
	if l != nil {
		logger = l
	}
}

// Change the Cache implementation used by this package.
func SetCache(c Cache) {
	if c != nil {
		cache = c
	}
}

// Change the next prime calculation function used by this package.
func SetFindNextPrimeFunction(f FindNextPrimeFunc) {
	findNextPrime = f
}

//
func PiDigit(ctx context.Context, n uint64) (string, error) {
	l := logger.With(
		zap.Uint64("n", n),
	)
	l.Debug("PiDigit: enter")
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
