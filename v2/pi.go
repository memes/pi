// Package pi calculates the nth fractional digit of pi using a Bailey-Borwein-Plouffe
// algorithm (see https://wikipedia.org/wiki/Bailey%E2%80%93Borwein%E2%80%93Plouffe_formula).
// This allows any arbitrary fractional digit of pi to be calculated independently
// of the preceding digits albeit with longer calculation times as the value
// of n increases because of the need to calculate prime numbers of
// increasing value.
//
//
//
// NOTE: This package is intended to be used in distributed computing and cloud
// scaling demos, and does not guarantee accuracy or efficiency of calculated
// fractional digits.
package pi

import (
	"context"
	"strconv"

	"github.com/go-logr/logr"
)

var (
	// Logger to use in this package; default is a no-op logger.
	logger = logr.Discard()
	// Cache implementation to use; default is a no-op cache.
	cache Cache = NewNoopCache()
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

// Returns the zero-based nth fractional digit of pi.
func FractionalDigit(ctx context.Context, n uint64) (uint32, error) {
	l := logger.V(1).WithValues("n", n)
	l.Info("FractionalDigit: enter")
	index := uint64(n/9) * 9
	key := strconv.FormatUint(index, 16)
	digits, err := cache.GetValue(ctx, key)
	if err != nil {
		return 0, err
	}
	if digits == "" {
		digits = calcDigits(index)
		err = cache.SetValue(ctx, key, digits)
		if err != nil {
			return 0, err
		}
	}
	offset := n % 9
	result, err := strconv.ParseUint(digits[offset:offset+1], 10, 32)
	if err != nil {
		return 0, err
	}
	logger.Info("FractionalDigit: exit", "result", result)
	return uint32(result), nil
}
