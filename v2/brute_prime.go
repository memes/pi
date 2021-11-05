package pi

import (
	"math"
)

// Use a naive, brute force approach to determining if a positive integer is
// prime by iterating through the set of integers [2, sqrt(n)] to see if they
// divide wholly.
func bruteIsPrime(n uint64) bool {
	l := logger.V(1).WithValues("n", n)
	l.Info("bruteIsPrime: entered")
	if n%2 == 0 {
		l.Info("bruteIsPrime: exit", "result", false)
		return false
	}
	r := uint64(math.Sqrt(float64(n)))
	for i := uint64(3); i <= r; i += 2 {
		if n%i == 0 {
			l.Info("bruteIsPrime: exit", "result", false)
			return false
		}
	}
	l.Info("bruteIsPrime: exit", "result", true)
	return true
}

// Determine the next prime number greater than n by iterating over the set of
// integers greater than n until one passes the brute force prime test.
func BruteFindNextPrime(n uint64) uint64 {
	l := logger.V(1).WithValues("n", n)
	var next uint64
	l.Info("BruteFindNextPrime: enter")
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
	l.Info("BruteFindNextPrime: exit", "result", next)
	return next
}
