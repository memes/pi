package pi

import (
	"math"

	"go.uber.org/zap"
)

// Use a naive, brute force approach to determining if a positive integer is
// prime by iterating through the set of integers [2, sqrt(n)] to see if they
// divide wholly.
func bruteIsPrime(n uint64) bool {
	l := logger.With(
		zap.Uint64("n", n),
	)
	l.Debug("bruteIsPrime: entered")
	if n%2 == 0 {
		l.Debug("bruteIsPrime: exit",
			zap.Bool("result", false),
		)
		return false
	}
	r := uint64(math.Sqrt(float64(n)))
	var i uint64 = 3
	for ; i <= r; i += 2 {
		if n%i == 0 {
			l.Debug("bruteIsPrime: exit",
				zap.Bool("result", false),
			)
			return false
		}
	}
	l.Debug("bruteIsPrime: exit",
		zap.Bool("result", true),
	)
	return true
}

// Determine the next prime number greater than n by iterating over the set of
// integers greater than n until one passes the brute force prime test.
func BruteFindNextPrime(n uint64) uint64 {
	l := logger.With(
		zap.Uint64("n", n),
	)
	var next uint64
	l.Debug("BruteFindNextPrime: enter")
	if n < 2 {
		next = 2
	} else {
		if n%2 == 0 {
			next = n + 1
		} else {
			next = n + 2
		}
		for ; !bruteIsPrime(next); next++ {
		}
	}
	l.Debug("BruteFindNextPrime: exit",
		zap.Uint64("result", next),
	)
	return next
}
