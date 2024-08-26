package server_test

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis"
	"github.com/memes/pi/v2/pkg/cache"
	"github.com/memes/pi/v2/pkg/generated"
	"github.com/memes/pi/v2/pkg/server"
)

const (
	// First 99 fractional digits of pi.
	PiDigits = "1415926535897932384626433832795028841971693993751058209749445923078164062862089986280348253421170679"
)

func testGetDigit(ctx context.Context, t *testing.T, request *generated.GetDigitRequest, piServer *server.PiServer) {
	t.Helper()
	expected, err := strconv.ParseUint(PiDigits[request.Index:request.Index+1], 10, 32)
	if err != nil {
		t.Errorf("Error parsing Uint: %v", err)
	}
	actual, err := piServer.GetDigit(ctx, request)
	if err != nil {
		t.Errorf("Error calling FractionalDigit: %v", err)
	}
	if uint64(actual.Digit) != expected {
		t.Errorf("Checking index: %d: expected %d got %d", request.Index, expected, actual.Digit)
	}
}

func TestGetDigit_WithNoopCache(t *testing.T) {
	ctx := context.Background()
	testCache := cache.NewNoopCache()
	if testCache == nil {
		t.Error("Noop cache is nil")
	}
	t.Parallel()
	piServer, err := server.NewPiServer(server.WithCache(testCache))
	if err != nil {
		t.Errorf("Error calling NewPiServer: %v", err)
	}
	for index := 0; index < len(PiDigits); index++ {
		t.Run(fmt.Sprintf("index=%d", index), func(t *testing.T) {
			testGetDigit(ctx, t, &generated.GetDigitRequest{
				Index: uint64(index),
			}, piServer)
		})
	}
}

func TestGetDigit_WithRedisCache(t *testing.T) {
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
	piServer, err := server.NewPiServer(server.WithCache(testCache))
	if err != nil {
		t.Errorf("Error calling NewPiServer: %v", err)
	}
	for index := 0; index < len(PiDigits); index++ {
		t.Run(fmt.Sprintf("index=%d", index), func(t *testing.T) {
			testGetDigit(ctx, t, &generated.GetDigitRequest{
				Index: uint64(index),
			}, piServer)
		})
	}
}

// Verify that setting uint index to a number larger than supported by int64 will return an error.
func TestGetDigit_Overflow(t *testing.T) {
	ctx := context.Background()
	t.Parallel()
	piServer, err := server.NewPiServer()
	if err != nil {
		t.Errorf("Error calling NewPiServer: %v", err)
	}
	_, err = piServer.GetDigit(ctx, &generated.GetDigitRequest{
		Index: uint64(math.MaxInt64) + 1,
	})
	if err == nil {
		t.Error("expected GetDigit to return an error, but didn't")
	}
}
