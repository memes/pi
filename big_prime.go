package pi

import (
	"math/big"

	"go.uber.org/zap"
)

const (
	MILLER_RABIN_ROUNDS = 0
)

var (
	unit = big.NewInt(1)
)

// Determine the next prime number greater than n by iterating over the set of
// integers greater than n until one passes the brute force prime test.
func BigFindNextPrime(n uint64) uint64 {
	l := logger.With(
		zap.Uint64("n", n),
	)
	l.Debug("BigFindNextPrime: entered")
	var result uint64
	if n < 2 {
		result = 2
	} else {
		next := big.NewInt(int64(n + 1))
		for ; !next.ProbablyPrime(MILLER_RABIN_ROUNDS); next = next.Add(next, unit) {
		}
		result = next.Uint64()
	}
	l.Debug("BigFindNextPrime: exit",
		zap.Uint64("result", result),
	)
	return result
}
