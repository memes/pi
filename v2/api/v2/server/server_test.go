package server

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis"
	api "github.com/memes/pi/v2/api/v2"
)

const (
	// First 99 fractional digits of pi
	PI_DIGITS = "1415926535897932384626433832795028841971693993751058209749445923078164062862089986280348253421170679"
)

func testGetDigit(ctx context.Context, request *api.GetDigitRequest, server *PiServer, t *testing.T) {
	t.Parallel()
	expected, err := strconv.ParseUint(PI_DIGITS[request.Index:request.Index+1], 10, 32)
	if err != nil {
		t.Errorf("Error parsing Uint: %v", err)
	}
	actual, err := server.GetDigit(ctx, request)
	if err != nil {
		t.Errorf("Error calling FractionalDigit: %v", err)
	}
	if actual.Digit != uint32(expected) {
		t.Errorf("Checking index: %d: expected %d got %d", request.Index, expected, actual.Digit)
	}
}

func TestGetDigit_WithNoopCache(t *testing.T) {
	ctx := context.Background()
	cache := NewNoopCache()
	if cache == nil {
		t.Error("Noop cache is nil")
	}
	server := NewPiServer(WithCache(cache))
	for index := 0; index < len(PI_DIGITS); index++ {
		t.Run(fmt.Sprintf("index=%d", index), func(t *testing.T) {
			testGetDigit(ctx, &api.GetDigitRequest{
				Index: uint64(index),
			}, server, t)
		})
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
	server := NewPiServer(WithCache(cache))
	for index := 0; index < len(PI_DIGITS); index++ {
		t.Run(fmt.Sprintf("index=%d", index), func(t *testing.T) {
			testGetDigit(ctx, &api.GetDigitRequest{
				Index: uint64(index),
			}, server, t)
		})
	}
}
