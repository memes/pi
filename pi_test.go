package pi

import (
	"context"
	"testing"
)

func TestPiDigit(t *testing.T) {
	ctx := context.Background()
	for i := 0; i < 100; i++ {
		expected := string(PI_DIGITS[i])
		actual, err := PiDigit(ctx, uint64(i))
		if err != nil {
			t.Errorf("Error calling PiDigit: %v", err)
		}
		if actual != expected {
			t.Errorf("Checking offset: %d: expected %s got %s", i, expected, actual)
		}
	}
}
