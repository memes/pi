// Package pi calculates the nth and 8 following fractional decimal digits of pi
// using a Bailey-Borwein-Plouffe algorithm (see https://wikipedia.org/wiki/Bailey%E2%80%93Borwein%E2%80%93Plouffe_formula).
// This allows any arbitrary fractional digit of pi to be calculated independently
// of the preceding digits albeit with longer calculation times as the value
// of n increases because of the need to calculate prime numbers of
// increasing value.
//
// NOTE 1: This package is intended to be used in distributed computing and cloud
// scaling demos, and does not guarantee accuracy or efficiency of calculated
// fractional digits.
//
// NOTE 2: The algorithm in BBPDigits is based on the work of Fabrice Bellard,
// original source code published at https://bellard.org/pi/pi.c
package pi

import (
	"fmt"
	"math"
	"math/big"

	"github.com/go-logr/logr"
)

const (
	// The number of MR rounds to use when determining if the number is
	// probably a prime. A value of zero will apply a Baillie-PSW only test
	// and requires Go 1.8+.
	DEFAULT_MILLER_RABIN_ROUNDS = 0
)

var (
	TWO = big.NewInt(2)
)

// Calculator object holds the options to use when calculating fractional digits
// of pi.
type Calculator struct {
	// The logr.Logger instance to use.
	logger logr.Logger
	// The number of Miller-Rabin rounds to use in Probably Prime calls.
	millerRabinRounds int
}

// Defines the function signature for Calculator options.
type CalculatorOption func(*Calculator)

// Creates a new Calculator instance, applying the options provided.
func NewCalculator(options ...CalculatorOption) *Calculator {
	calculator := &Calculator{
		logger:            logr.Discard(),
		millerRabinRounds: DEFAULT_MILLER_RABIN_ROUNDS,
	}
	for _, option := range options {
		option(calculator)
	}
	return calculator
}

// Change the logger instance used by the calculator instance.
func WithLogger(logger logr.Logger) CalculatorOption {
	return func(c *Calculator) {
		c.logger = logger
	}
}

// Change the number of Miller-Rabin rounds used when determining if a number is
// prime.
func WithMillerRabinRounds(millerRabinRounds int) CalculatorOption {
	return func(c *Calculator) {
		c.millerRabinRounds = millerRabinRounds
	}
}

// Returns the inverse of x mod y
func (calc *Calculator) invMod(x int64, y int64) int64 {
	l := calc.logger.V(2).WithValues("x", x, "y", y)
	l.Info("invMod: entered")
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
	a = a % y
	if a < 0 {
		a = y + a
	}
	l.Info("invMod: exit", "a", a)
	return a
}

// Returns (a^b) mod m
func (c *Calculator) powMod(a int64, b int64, m int64) int64 {
	l := c.logger.V(2).WithValues("a", a, "b", b, "m", m)
	l.Info("powMod: entered")
	var r int64 = 1
	for {
		if b&1 > 0 {
			r = (r * a) % m
		}
		b = b >> 1
		if b == 0 {
			break
		}
		a = (a * a) % m
	}
	l.Info("powMod: exit", "r", r)
	return r
}

// Return the next largest prime number that is greater than n.
func (c *Calculator) findNextPrime(n int64) int64 {
	l := c.logger.V(2).WithValues("n", n)
	l.Info("findNextPrime: entered")
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
		for ; !next.ProbablyPrime(c.millerRabinRounds); next = next.Add(next, TWO) {
		}
		result = next.Int64()
	}
	l.Info("findNextPrime: exit", "result", result)
	return result
}

// Implements a BBP spigot algorithm to determine the nth and 8 following
// fractional decimal digits of pi at the specified zero-based offset, with
// the configured Calculator.
func (c *Calculator) BBPDigits(n uint64) string {
	l := c.logger.V(1).WithValues("n", n)
	l.Info("BBPDigits: enter")
	N := int64(float64(n+21) * math.Log(10) / math.Log(2))
	var sum float64 = 0
	var t int64
	for a := int64(3); a <= (2 * N); a = c.findNextPrime(a) {
		vmax := int64(math.Log(float64(2*N)) / math.Log(float64(a)))
		av := int64(1)
		for i := int64(0); i < vmax; i++ {
			av = av * a
		}
		var s, num, den, v, kq, kq2 int64 = 0, 1, 1, 0, 1, 1

		for k := int64(1); k <= N; k++ {
			t = k
			if kq >= a {
				for {
					t = t / a
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
						t = t / a
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
				t = c.invMod(den, av)
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

		t = int64(c.powMod(10, int64(n), av))
		s = (s * t) % av
		sum = math.Mod(sum+float64(s)/float64(av), 1.0)
	}
	result := fmt.Sprintf("%09d", int(sum*1e9))
	l.Info("BBPDigits: exit", "result", result)
	return result
}

// Implements a BBP spigot algorithm to determine the nth and 8 following
// fractional decimal digits of pi at the specified zero-based offset, with
// a default Calculator instance.
func BBPDigits(n uint64) string {
	return NewCalculator().BBPDigits(n)
}
