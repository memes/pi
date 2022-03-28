// Package pi calculates the nth and 8 following fractional decimal digits of pi
// using a Bailey-Borwein-Plouffe algorithm
// (see https://wikipedia.org/wiki/Bailey%E2%80%93Borwein%E2%80%93Plouffe_formula).
// This allows any arbitrary fractional digit of pi to be calculated independently
// of the preceding digits albeit with longer calculation times as the value
// of n increases because of the need to calculate prime numbers of
// increasing value.
//
// NOTE 1: This package is intended to be used in distributed computing and cloud
// scaling demos, and does not guarantee accuracy or efficiency of calculated
// fractional digits.
//
// NOTE 2: The algorithms used invMod, powMod, and BPPDigits are based on the
// work of Fabrice Bellard (original source code published at https://bellard.org/pi/pi.c).
package pi

import (
	"fmt"
	"math"
	"math/big"

	"github.com/go-logr/logr"
)

var (
	// The package logr.Logger instance to use.
	//nolint: gochecknoglobals // Allow package consumers to set the logger
	Logger = logr.Discard()
	// The number of Miller-Rabin rounds to use in FindNextPrime when
	// determining if an integer is probabilistically a prime. A value of
	// zero will apply a Baillie-PSW only test and requires Go 1.8+.
	//nolint: gochecknoglobals // Allow package consumers to override
	MillerRabinRounds = 0
	// The constant 2; declared to avoid repeated allocation in FindNextPrime.
	//nolint: gochecknoglobals // Avoid repeated allocation
	two = big.NewInt(2)
)

// Returns the inverse of x mod y.
func invMod(x, y int64) int64 {
	logger := Logger.V(2).WithValues("x", x, "y", y)
	logger.Info("invMod: entered")
	var u, v, c, a int64 = x, y, 1, 0
	for {
		q := v / u
		t := c
		c = a - q*c
		a = t
		t = u
		u = v - q*u
		v = t
		if u == 0 {
			break
		}
	}
	a %= y
	if a < 0 {
		a = y + a
	}
	logger.Info("invMod: exit", "a", a)
	return a
}

// Returns (a^b) mod m.
func powMod(a, b, m int64) int64 {
	logger := Logger.V(2).WithValues("a", a, "b", b, "m", m)
	logger.Info("powMod: entered")
	var r int64 = 1
	for {
		if b&1 > 0 {
			r = (r * a) % m
		}
		b >>= 1
		if b == 0 {
			break
		}
		a = (a * a) % m
	}
	logger.Info("powMod: exit", "r", r)
	return r
}

// Return the next largest prime number that is greater than n.
func FindNextPrime(n int64) int64 {
	logger := Logger.V(2).WithValues("n", n)
	logger.Info("FindNextPrime: entered")
	var result int64
	if n < 2 {
		result = 2
	} else {
		var next *big.Int
		if n%2 == 0 {
			next = big.NewInt(n + 1)
		} else {
			next = big.NewInt(n + 2)
		}
		for ; !next.ProbablyPrime(MillerRabinRounds); next = next.Add(next, two) {
		}
		result = next.Int64()
	}
	logger.Info("FindNextPrime: exit", "result", result)
	return result
}

// Implements a BBP spigot algorithm to determine the nth and 8 following
// fractional decimal digits of pi at the specified zero-based offset.
func BBPDigits(n uint64) string {
	logger := Logger.V(1).WithValues("n", n)
	logger.Info("BBPDigits: enter")
	N := int64(float64(n+21) * math.Log(10) / math.Log(2))
	var sum float64
	var t int64
	for a := int64(3); a <= (2 * N); a = FindNextPrime(a) {
		vmax := int64(math.Log(float64(2*N)) / math.Log(float64(a)))
		av := int64(1)
		for i := int64(0); i < vmax; i++ {
			av *= a
		}
		var s, num, den, v, kq, kq2 int64 = 0, 1, 1, 0, 1, 1

		for k := int64(1); k <= N; k++ {
			t = k
			if kq >= a {
				for {
					t /= a
					v--
					if t%a != 0 {
						break
					}
				}
				kq = 0
			}
			kq++
			num = (num * t) % av

			t = (2*k - 1)
			if kq2 >= a {
				if kq2 == a {
					for {
						t /= a
						v++
						if t%a != 0 {
							break
						}
					}
				}
				kq2 -= a
			}
			den = (den * t) % av
			kq2 += 2

			if v > 0 {
				t = invMod(den, av)
				t = (t * num) % av
				t = (t * k) % av
				for i := v; i < vmax; i++ {
					t = (t * a) % av
				}
				s += t
				if s >= av {
					s -= av
				}
			}
		}

		t = powMod(10, int64(n), av)
		s = (s * t) % av
		sum = math.Mod(sum+float64(s)/float64(av), 1.0)
	}
	result := fmt.Sprintf("%09d", int(sum*1e9))
	logger.Info("BBPDigits: exit", "result", result)
	return result
}
