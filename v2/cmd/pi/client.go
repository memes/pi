package main

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	api "github.com/memes/pi/api/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

const (
	DEFAULT_COUNT   = 100
	DEFAULT_TIMEOUT = 10 * time.Second
)

var (
	clientCmd = &cobra.Command{
		Use:   "client gRPCEndpoint...",
		Short: "Run a gRPC client to request pi digits",
		Long:  "Launch a client that attempts to connect to servers and return a subset of the mantissa of pi.",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			count := viper.GetInt("count")
			logger := logger.With(
				zap.Int("count", count),
				zap.Strings("args", args),
			)
			timeout := viper.GetDuration("timeout")
			logger.Debug("Running client")
			// Randomize the retrieval of numbers
			indices := rand.Perm(count)
			digits := make([]string, count)
			var wg sync.WaitGroup
			for _, index := range indices {
				wg.Add(1)
				go func(index uint64) {
					defer wg.Done()
					log := logger.With(
						zap.Uint64("index", index),
					)
					log.Debug("In goroutine")
					digit, err := fetchDigit(args, index, timeout)
					if err != nil {
						log.Error("Error getting digit",
							zap.Error(err),
						)
						digit = "#"
					}
					digits[index] = digit
				}(uint64(index))
			}
			wg.Wait()
			fmt.Printf("Result is: 3.%s\n", strings.Join(digits, ""))
		},
	}
)

func init() {
	clientCmd.PersistentFlags().IntP("count", "c", DEFAULT_COUNT, "number of decimal digits of pi to request")
	clientCmd.PersistentFlags().DurationP("timeout", "t", DEFAULT_TIMEOUT, "client timeout")
	_ = viper.BindPFlag("count", clientCmd.PersistentFlags().Lookup("count"))
	_ = viper.BindPFlag("timeout", clientCmd.PersistentFlags().Lookup("timeout"))
	rootCmd.AddCommand(clientCmd)
}

func fetchDigit(endpoints []string, index uint64, timeout time.Duration) (string, error) {
	logger := logger.With(
		zap.Strings("endpoints", endpoints),
		zap.Uint64("index", index),
		zap.Duration("timeout", timeout),
	)
	logger.Debug("Starting connection to service")
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	conn, err := grpc.DialContext(ctx, endpoints[0], grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		logger.Error("Error creating gRPC connection",
			zap.Error(err),
		)
		return "", err
	}
	defer conn.Close()
	client := api.NewPiServiceClient(conn)
	response, err := client.GetDigit(ctx, &api.GetDigitRequest{
		Index: index,
	})
	if err != nil {
		logger.Error("Error in gRPC request",
			zap.Error(err),
		)
		return "", err
	}
	logger.Debug("Response from remote",
		zap.String("result", response.Digit),
		zap.String("metadata.identity", response.Metadata.Identity),
		zap.Strings("metadata.addresses", response.Metadata.Addresses),
		zap.Any("metadata.labels", response.Metadata.Labels),
	)

	return response.Digit, nil
}
