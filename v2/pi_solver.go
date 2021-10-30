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
	return a
}

// Returns (a^b) mod m
func powMod(a uint64, b uint64, m uint64) uint64 {
	var r uint64 = 1
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
	return r
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
	for a := int64(3); a <= (2 * N); a = int64(findNextPrime(uint64(a))) {
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
		if av < 0 {
			panic(fmt.Sprintf("av is %d", av))
		}

		t = int64(powMod(10, n, uint64(av)))
		s = (s * t) % av
		sum = math.Mod(sum+float64(s)/float64(av), 1.0)
	}
	result := fmt.Sprintf("%09d", int(sum*1e9))
	l.Debug("CalcDigits: exit",
		zap.String("result", result),
	)
	return result
}
