package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	api "github.com/memes/pi/v2/api/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
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
		RunE: client,
	}
)

func init() {
	clientCmd.PersistentFlags().UintP("count", "c", DEFAULT_DIGIT_COUNT, "number of decimal digits of pi to request")
	clientCmd.PersistentFlags().DurationP("timeout", "t", DEFAULT_CLIENT_TIMEOUT, "client timeout")
	_ = viper.BindPFlag("count", clientCmd.PersistentFlags().Lookup("count"))
	_ = viper.BindPFlag("timeout", clientCmd.PersistentFlags().Lookup("timeout"))
	rootCmd.AddCommand(clientCmd)
}

// Initiate a gRPC connect to endpoint and retrieve a single fractional digit of
// pi at the zero-based index.
func fetchDigit(endpoint string, index uint64, timeout time.Duration) (uint32, error) {
	logger := logger.V(0).WithValues("endpoint", endpoint, "index", index, "timeout", timeout)
	logger.Info("Starting connection to service")
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	conn, err := grpc.DialContext(ctx, endpoint, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()))
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	client := api.NewPiServiceClient(conn)
	response, err := client.GetDigit(ctx, &api.GetDigitRequest{
		Index: index,
	})
	if err != nil {
		return 0, err
	}
	logger.Info("Response from remote", "result", response.Digit, "metadata", response.Metadata)

	return response.Digit, nil
}

// Client sub-command entrypoint. This function will launch gRPC requests for
// each of the fractional digits requested.
func client(cmd *cobra.Command, endpoints []string) error {
	count := viper.GetInt("count")
	timeout := viper.GetDuration("timeout")
	logger := logger.V(0).WithValues("count", count, "endpoints", endpoints, "timeout", timeout)
	logger.V(1).Info("Preparing telemetry")
	ctx := context.Background()
	shutdown := initTelemetry(ctx, CLIENT_SERVICE_NAME, sdktrace.AlwaysSample())
	defer shutdown(ctx)

	logger.V(1).Info("Running client")
	// Randomize the retrieval of numbers
	indices := rand.Perm(count)
	digits := make([]byte, count)
	var wg sync.WaitGroup
	for i, index := range indices {
		endpoint := endpoints[i%len(endpoints)]
		wg.Add(1)
		go func(endpoint string, index uint64) {
			defer wg.Done()
			digit, err := fetchDigit(endpoint, index, timeout)
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
