package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/alicebob/miniredis"
	"github.com/memes/pi"
)

const (
	// First 999 digits of pi following decimal place
	PI_DIGITS = "141592653589793238462643383279502884197169399375105820974944592307816406286208998628034825342117067982148086513282306647093844609550582231725359408128481117450284102701938521105559644622948954930381964428810975665933446128475648233786783165271201909145648566923460348610454326648213393607260249141273724587006606315588174881520920962829254091715364367892590360011330530548820466521384146951941511609433057270365759591953092186117381932611793105118548074462379962749567351885752724891227938183011949129833673362440656643086021394946395224737190702179860943702770539217176293176752384674818467669405132000568127145263560827785771342757789609173637178721468440901224953430146549585371050792279689258923542019956112129021960864034418159813629774771309960518707211349999998372978049951059731732816096318595024459455346908302642522308253344685035261931188171010003137838752886587533208381420617177669147303598253490428755468731159562863882353787593751957781857780532171226806613001927876611195909216420198"
)

func TestRedisCache(t *testing.T) {
	ctx := context.Background()
	mock, err := miniredis.Run()
	if err != nil {
		t.Errorf("Error running miniredis: %v", err)
	}
	cache := NewRedisCache(ctx, mock.Addr())
	if cache == nil {
		t.Error("Redis cache is nil")
	}
	for i := 0; i < 10; i++ {
		expected := ""
		key := fmt.Sprintf("%d", i)
		actual, err := cache.GetValue(ctx, key)
		if err != nil {
			t.Errorf("GetValue returned an error: %v", err)
		}
		if actual != expected {
			t.Errorf("Index %d: Expected %s received %s", i, expected, actual)
		}
		expected = fmt.Sprintf("%.09d", i)
		if err = cache.SetValue(ctx, key, expected); err != nil {
			t.Errorf("Index: %d: SetValue returned an error: %v", i, err)

		}
		actual, err = cache.GetValue(ctx, key)
		if err != nil {
			t.Errorf("GetDigits returned an error: %v", err)
		}
		if actual != expected {
			t.Errorf("Index %d: Expected %s received %s", i, expected, actual)
		}

	}
}

func TestPiDigitWithRedisCache(t *testing.T) {
	ctx := context.Background()
	mock, err := miniredis.Run()
	if err != nil {
		t.Errorf("Error running miniredis: %v", err)
	}
	cache := NewRedisCache(ctx, mock.Addr())
	if cache == nil {
		t.Error("Redis cache is nil")
	}
	pi.SetCache(cache)
	for i := 0; i < len(PI_DIGITS); i++ {
		expected := string(PI_DIGITS[i])
		actual, err := pi.PiDigit(ctx, uint64(i))
		if err != nil {
			t.Errorf("Error calling PiDigits: %v", err)
		}
		if actual != expected {
			t.Errorf("Checking offset: %d: expected %s got %s", i, expected, actual)
		}
	}
}
