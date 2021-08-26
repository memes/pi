package pi

// Calculates and returns the n-th and 8 following digits of Pi
//
// Based on source provided by Fabrice Bellard, published at https://bellard.org/pi/pi.c

import (
	"fmt"
	"math"

	"go.uber.org/zap"
)

// Returns the inverse of x mod y
func invMod(x int64, y int64) int64 {
	l := logger.With(
		zap.Int64("x", x),
		zap.Int64("y", y),
	)
	l.Debug("invMod: enter")
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
	l.Debug("invMod: exit",
		zap.Int64("result", a),
	)
	return a
}

// Returns (a^b) mod m
func powMod(a int64, b int64, m int64) int64 {
	l := logger.With(
		zap.Int64("a", a),
		zap.Int64("b", b),
		zap.Int64("m", m),
	)
	l.Debug("powMod: entered")
	var r, aa int64 = 1, a
	for {
		if b&1 > 0 {
			r = (r * aa) % m
		}
		b = b >> 1
		if b == 0 {
			break
		}
		aa = (aa * aa) % m
	}
	l.Debug("powMod: exit",
		zap.Int64("result", r),
	)
	return r
}

// Return true if n is a prime
func isPrime(n uint64) bool {
	l := logger.With(
		zap.Uint64("n", n),
	)
	l.Debug("isPrime: entered")
	if n%2 == 0 {
		l.Debug("isPrime: exit",
			zap.Bool("result", false),
		)
		return false
	}
	r := uint64(math.Sqrt(float64(n)))
	var i uint64 = 3
	for ; i <= r; i += 2 {
		if n%i == 0 {
			l.Debug("isPrime: exit",
				zap.Bool("result", false),
			)
			return false
		}
	}
	l.Debug("isPrime: exit",
		zap.Bool("result", true),
	)
	return true
}

// Return the next prime number greater than n
func nextPrime(n uint64) uint64 {
	l := logger.With(
		zap.Uint64("n", n),
	)
	var next uint64
	l.Debug("nextPrime: enter")
	if n < 2 {
		next = 2
	} else {
		for next = n + 1; !isPrime(next); next++ {
		}
	}
	l.Debug("nextPrime: exit",
		zap.Uint64("result", next),
	)
	return next
}

// Returns a 9 chararcter string containing the decimal digits of pi starting
// at the specified offset. E.g. CalcDigits(0) -> "141592653",
// CalcDigits(1) -> 415926535, etc.
//
// NOTE: this function has been modified to be zero-based, unlike original code
func CalcDigits(n uint64) string {
	l := logger.With(
		zap.Uint64("n", n),
	)
	l.Debug("CalcDigits: enter")
	N := int64(float64(n+21) * math.Log(10) / math.Log(2))
	var sum float64 = 0
	var t int64
	for a := int64(3); a <= (2 * N); a = int64(nextPrimeFn(uint64(a))) {
		// spell-checker: ignore vmax
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
	l.Debug("CalcDigits: exit",
		zap.String("result", result),
	)
	return result
}
