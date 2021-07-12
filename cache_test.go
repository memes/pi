package pi

import (
	"context"
	"fmt"
	"testing"
)

func TestNoopCache(t *testing.T) {
	ctx := context.Background()
	cache := NewNoopCache()
	if cache == nil {
		t.Error("Noop cache is nil")
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
		if err = cache.SetValue(ctx, key, "1234"); err != nil {
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
