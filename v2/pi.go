package pi

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
)

// Defines the signature of a function that will return the next largest prime
// number that is greater than the supplied value.
type FindNextPrimeFunc func(uint64) uint64

var (
	// Logger to use in this package; default is a no-op logger.
	logger = logr.Discard()
	// Cache implementation to use; default is a no-op cache.
	cache Cache = NewNoopCache()
	// The next prime function to use in this package; default is a naive
	// brute-force calculator.
	findNextPrime FindNextPrimeFunc = BruteFindNextPrime
)

// Change the logger instance used by this package.
func SetLogger(l logr.Logger) {
	logger = l
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
	l := logger.V(1).WithValues("n", n)
	l.Info("PiDigit: enter")
	index := uint64(n/9) * 9
	key := fmt.Sprintf("%d", index)
	digits, err := cache.GetValue(ctx, key)
	if err != nil {
		return "", err
	}
	if digits == "" {
		digits = CalcDigits(index)
		err = cache.SetValue(ctx, key, digits)
		if err != nil {
			return "", err
		}
	}
	digit := string(digits[n%9])
	logger.Info("PiDigit: exit", "digit", digit)
	return digit, nil
}
