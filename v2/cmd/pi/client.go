package main

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	api "github.com/memes/pi/v2/api/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
			logger := logger.WithValues("count", count, "args", args)
			timeout := viper.GetDuration("timeout")
			logger.V(0).Info("Running client")
			// Randomize the retrieval of numbers
			indices := rand.Perm(count)
			digits := make([]string, count)
			var wg sync.WaitGroup
			for _, index := range indices {
				wg.Add(1)
				go func(index uint64) {
					defer wg.Done()
					log := logger.WithValues("index", index)
					log.V(1).Info("In goroutine")
					digit, err := fetchDigit(args, index, timeout)
					if err != nil {
						log.Error(err, "Error getting digit")
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
	logger := logger.V(1).WithValues("endpoints", endpoints, "index", index, "timeout", timeout)
	logger.Info("Starting connection to service")
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	conn, err := grpc.DialContext(ctx, endpoints[0], grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return "", err
	}
	defer conn.Close()
	client := api.NewPiServiceClient(conn)
	response, err := client.GetDigit(ctx, &api.GetDigitRequest{
		Index: index,
	})
	if err != nil {
		return "", err
	}
	logger.Info("Response from remote", "result", response.Digit, "metadata", response.Metadata)

	return response.Digit, nil
}
