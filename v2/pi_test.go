package pi

import (
	"context"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis"
)

func TestFractionalDigit_WithNoopCache(t *testing.T) {
	ctx := context.Background()
	cache := NewNoopCache()
	if cache == nil {
		t.Error("Noop cache is nil")
	}
	SetCache(cache)
	for i := uint64(0); i < 9*TEST_CACHE_LOOP_LIMIT; i++ {
		expected, err := strconv.ParseUint(PI_DIGITS[i:i+1], 10, 32)
		if err != nil {
			t.Errorf("Error parsing Uint: %v", err)
		}
		actual, err := FractionalDigit(ctx, i)
		if err != nil {
			t.Errorf("Error calling FractionalDigit: %v", err)
		}
		if actual != uint32(expected) {
			t.Errorf("Checking offset: %d: expected %d got %d", i, expected, actual)
		}
	}
}

func TestFractionalDigit_WithRedisCache(t *testing.T) {
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
	for i := uint64(0); i < 9*TEST_CACHE_LOOP_LIMIT; i++ {
		expected, err := strconv.ParseUint(PI_DIGITS[i:i+1], 10, 32)
		if err != nil {
			t.Errorf("Error parsing Uint: %v", err)
		}
		actual, err := FractionalDigit(ctx, i)
		if err != nil {
			t.Errorf("Error calling FractionalDigit: %v", err)
		}
		if actual != uint32(expected) {
			t.Errorf("Checking offset: %d: expected %d got %d", i, expected, actual)
		}
	}
}
