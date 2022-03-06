package cache_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis"
	"github.com/memes/pi/v2/pkg/cache"
)

const (
	TestCacheLoopLimit = 10
)

// The noopCache should do nothing useful. This test confirms that values can
// appear to be added successfully, but an attempt to recall the value will
// result in an empty string.
func TestNoopCache(t *testing.T) {
	ctx := context.Background()
	testCache := cache.NewNoopCache()
	if testCache == nil {
		t.Error("Noop cache is nil")
	}
	t.Parallel()
	for i := uint64(0); i < TestCacheLoopLimit; i++ {
		expected := ""
		key := strconv.FormatUint(i, 16)
		actual, err := testCache.GetValue(ctx, key)
		if err != nil {
			t.Errorf("GetValue returned an error: %v", err)
		}
		if actual != expected {
			t.Errorf("Index %d: Expected %s received %s", i, expected, actual)
		}
		if err = testCache.SetValue(ctx, key, "1234"); err != nil {
			t.Errorf("Index: %d: SetValue returned an error: %v", i, err)
		}
		actual, err = testCache.GetValue(ctx, key)
		if err != nil {
			t.Errorf("GetDigits returned an error: %v", err)
		}
		if actual != expected {
			t.Errorf("Index %d: Expected %s received %s", i, expected, actual)
		}
	}
}

// The RedisCache will use a Redis-like in-memory instance to cache values. The
// test should confirm that a value can be added to the cache and recalled
// successfully.
func TestRedisCache(t *testing.T) {
	ctx := context.Background()
	mock, err := miniredis.Run()
	if err != nil {
		t.Errorf("Error running miniredis: %v", err)
	}
	testCache := cache.NewRedisCache(ctx, mock.Addr())
	if testCache == nil {
		t.Error("Redis cache is nil")
	}
	t.Parallel()
	for i := uint64(0); i < TestCacheLoopLimit; i++ {
		expected := ""
		key := strconv.FormatUint(i, 16)
		actual, err := testCache.GetValue(ctx, key)
		if err != nil {
			t.Errorf("GetValue returned an error: %v", err)
		}
		if actual != expected {
			t.Errorf("Index %d: Expected %s received %s", i, expected, actual)
		}
		expected = fmt.Sprintf("%09d", i)
		if err = testCache.SetValue(ctx, key, expected); err != nil {
			t.Errorf("Index: %d: SetValue returned an error: %v", i, err)
		}
		actual, err = testCache.GetValue(ctx, key)
		if err != nil {
			t.Errorf("GetDigits returned an error: %v", err)
		}
		if actual != expected {
			t.Errorf("Index %d: Expected %s received %s", i, expected, actual)
		}
	}
}
