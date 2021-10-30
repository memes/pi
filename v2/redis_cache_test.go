package pi

import (
	"context"
	"fmt"
	"testing"

	"github.com/alicebob/miniredis"
)

// The RedisCache will use a Redis-like in-memory instance to cache values. The
// test should confirm that a value can be added to the cache and recalled
// successfully.
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
	for i := 0; i < TEST_CACHE_LOOP_LIMIT; i++ {
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
	SetCache(cache)
	for i := 0; i < 9*TEST_CACHE_LOOP_LIMIT; i++ {
		expected := string(PI_DIGITS[i])
		actual, err := PiDigit(ctx, uint64(i))
		if err != nil {
			t.Errorf("Error calling PiDigits: %v", err)
		}
		if actual != expected {
			t.Errorf("Checking offset: %d: expected %s got %s", i, expected, actual)
		}
	}
}
