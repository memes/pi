package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/memes/pi/v2/api/v2/client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric/global"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const (
	CLIENT_SERVICE_NAME    = "client"
	DEFAULT_DIGIT_COUNT    = 100
	DEFAULT_CLIENT_TIMEOUT = 10 * time.Second
)

var (
	// Implements the client sub-command which attempts to connect to one or
	// more pi server instances and build up the digits of pi through multiple
	// requests.
	clientCmd = &cobra.Command{
		Use:   CLIENT_SERVICE_NAME + " endpoint [endpoint]",
		Short: "Run a gRPC client to request fractional digits of pi",
		Long: `Launches a client that will connect to the server instances in parallel and request the fractional digits of pi.
At least one endpoint address must be provided. Metrics and traces will be sent to an optionally provided OpenTelemetry collection endpoint.`,
		Args: cobra.MinimumNArgs(1),
		RunE: clientMain,
	}
)

func init() {
	clientCmd.PersistentFlags().UintP("count", "c", DEFAULT_DIGIT_COUNT, "number of decimal digits of pi to request")
	clientCmd.PersistentFlags().DurationP("timeout", "t", DEFAULT_CLIENT_TIMEOUT, "client timeout")
	_ = viper.BindPFlag("count", clientCmd.PersistentFlags().Lookup("count"))
	_ = viper.BindPFlag("timeout", clientCmd.PersistentFlags().Lookup("timeout"))
	rootCmd.AddCommand(clientCmd)
}

// Client sub-command entrypoint. This function will launch gRPC requests for
// each of the fractional digits requested.
func clientMain(cmd *cobra.Command, endpoints []string) error {
	count := viper.GetInt("count")
	logger := logger.V(1).WithValues("count", count, "endpoints", endpoints)
	logger.V(0).Info("Preparing telemetry")
	ctx := context.Background()
	shutdown := initTelemetry(ctx, CLIENT_SERVICE_NAME, sdktrace.AlwaysSample())
	defer shutdown(ctx)
	logger.V(0).Info("Running client")
	client := client.NewPiClient(
		client.WithLogger(logger),
		client.WithTimeout(viper.GetDuration("timeout")),
		client.WithTracer(otel.Tracer(CLIENT_SERVICE_NAME)),
		client.WithMeter(CLIENT_SERVICE_NAME, global.Meter(CLIENT_SERVICE_NAME)),
	)
	// Randomize the retrieval of numbers
	indices := rand.Perm(count)
	digits := make([]byte, count)
	var wg sync.WaitGroup
	for i, index := range indices {
		endpoint := endpoints[i%len(endpoints)]
		wg.Add(1)
		go func(endpoint string, index uint64) {
			defer wg.Done()
			digit, err := client.FetchDigit(endpoint, index)
			if err != nil {
				logger.Error(err, "Error fetching digit", "index", index)
				digits[index] = '-'
			} else {
				digits[index] = '0' + byte(digit)
			}
		}(endpoint, uint64(index))
	}
	wg.Wait()
	fmt.Print("Result is: 3.")
	if _, err := os.Stdout.Write(digits[:]); err != nil {
		return err
	}
	fmt.Println()
	return nil
}
