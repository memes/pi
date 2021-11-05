package pi

// Implements a Bailey-Borwein-Plouffe algorithm based on source code published
// by Fabrice Bellard at https://bellard.org/pi/pi.c

import (
	"fmt"
	"math"
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
func powMod(a int64, b int64, m int64) int64 {
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
	return r
}

// Implements a BPP spigot algorithm to determine the nth and 8 following
// fractional digits of pi at the specified zero-based offset.
func calcDigits(n uint64) string {
	l := logger.V(0).WithValues("n", n)
	l.Info("calcDigits: enter")
	N := int64(float64(n+21) * math.Log(10) / math.Log(2))
	var sum float64 = 0
	var t int64
	for a := int64(3); a <= (2 * N); a = int64(findNextPrime(uint64(a))) {
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

		t = int64(powMod(10, int64(n), av))
		s = (s * t) % av
		sum = math.Mod(sum+float64(s)/float64(av), 1.0)
	}
	result := fmt.Sprintf("%09d", int(sum*1e9))
	l.Info("calcDigits: exit", "result", result)
	return result
}
