package pi

import (
	"math/big"
)

const (
	// The number of MR rounds to use when determining if the number is
	// probably a prime. A value of zero will apply a Baillie-PSW only test
	// and required Go 1.8+.
	MILLER_RABIN_ROUNDS = 0
)

var (
	two = big.NewInt(2)
)

// Determine the next prime number greater than n by iterating over the set of
// integers greater than n until one passes the "math/big" package's ProbablyPrime
// method.
func BigFindNextPrime(n uint64) uint64 {
	l := logger.V(0).WithValues("n", n)
	l.Info("BigFindNextPrime: entered")
	var result uint64
	if n < 2 {
		result = 2
	} else {
		var offset uint64
		if n%2 == 0 {
			offset++
		} else {
			offset += 2
		}
		next := big.NewInt(int64(n + offset))
		for ; !next.ProbablyPrime(MILLER_RABIN_ROUNDS); next = next.Add(next, two) {
		}
		result = next.Uint64()
	}
	l.Info("BigFindNextPrime: exit", "result", result)
	return result
}
