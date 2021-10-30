package pi

import (
	"fmt"
	"math"
	"sort"
	"testing"
)

func testBigPrimeFindNext(start uint64, expected uint64, t *testing.T) {
	t.Parallel()
	if actual := BigFindNextPrime(start); actual != expected {
		t.Errorf("Checking start: %d: expected %d got %d", start, expected, actual)
	}
}

// Verify that the brute force prime solver gives the correct next greater prime
// number for the set of integers [0, largest prime in table).
func TestBigPrimeFindNext(t *testing.T) {
	for i := uint64(0); i < primeVerifyLimit; i++ {
		expected := verificationPrimes[sort.Search(primeTableSize, func(idx int) bool { return verificationPrimes[idx] > i })]
		t.Run(fmt.Sprintf("start=%d", i), func(t *testing.T) {
			testBigPrimeFindNext(i, expected, t)
		})
	}
}

// Benchmark the brute force prime solver method with starting points as a power
// of 10.
func BenchmarkBigPrimeFindNext(b *testing.B) {
	for exp := 0; exp < BENCHMARK_PRIME_EXPONENT_LIMIT; exp++ {
		start := uint64(math.Pow10(exp))
		b.Run(fmt.Sprintf("start=%d", start), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = BigFindNextPrime(start)
			}
		})
	}
}
