package server_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis"
	api "github.com/memes/pi/v2/internal/api/v2"
	"github.com/memes/pi/v2/pkg/cache"
	"github.com/memes/pi/v2/pkg/server"
)

const (
	// First 99 fractional digits of pi.
	PiDigits = "1415926535897932384626433832795028841971693993751058209749445923078164062862089986280348253421170679"
)

func testGetDigit(ctx context.Context, t *testing.T, request *api.GetDigitRequest, piServer *server.PiServer) {
	t.Helper()
	expected, err := strconv.ParseUint(PiDigits[request.Index:request.Index+1], 10, 32)
	if err != nil {
		t.Errorf("Error parsing Uint: %v", err)
	}
	actual, err := piServer.GetDigit(ctx, request)
	if err != nil {
		t.Errorf("Error calling FractionalDigit: %v", err)
	}
	if actual.Digit != uint32(expected) {
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
			testGetDigit(ctx, t, &api.GetDigitRequest{
				Index: uint64(index),
			}, piServer)
		})
	}
}

func TestFractionalDigit_WithRedisCache(t *testing.T) {
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
			testGetDigit(ctx, t, &api.GetDigitRequest{
				Index: uint64(index),
			}, piServer)
		})
	}
}
