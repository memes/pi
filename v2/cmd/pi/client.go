package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/memes/pi/v2/pkg/client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric/global"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc/credentials"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"
)

const (
	ClientServiceName = "pi.client"
	DefaultDigitCount = 100
	DefaultMaxTimeout = 10 * time.Second
)

// Implements the client sub-command which attempts to connect to one or
// more pi server instances and build up the digits of pi through multiple
// requests.
func NewClientCmd() (*cobra.Command, error) {
	clientCmd := &cobra.Command{
		Use:   "client target [target]",
		Short: "Run a gRPC Pi Service client to request fractional digits of pi",
		Long: `Launches a gRPC client that will connect to Pi Service target(s) and request the fractional digits of pi.

		At least one target endpoint must be provided. Metrics and traces will be sent to an OpenTelemetry collection endpoint, if specified.`,
		Args: cobra.MinimumNArgs(1),
		RunE: clientMain,
	}
	clientCmd.PersistentFlags().UintP("count", "c", DefaultDigitCount, "The number of decimal digits of pi to request")
	clientCmd.PersistentFlags().DurationP("max-timeout", "m", DefaultMaxTimeout, "The maximum timeout for a Pi Service request")
	clientCmd.PersistentFlags().String("authority", "", "Set the authoritative name of the Pi Service target for TLS verification, overriding hostname")
	clientCmd.PersistentFlags().Bool("insecure", false, "Disable TLS for gRPC connection to Pi Service")
	if err := viper.BindPFlag("count", clientCmd.PersistentFlags().Lookup("count")); err != nil {
		return nil, fmt.Errorf("failed to bind count pflag: %w", err)
	}
	if err := viper.BindPFlag("max-timeout", clientCmd.PersistentFlags().Lookup("max-timeout")); err != nil {
		return nil, fmt.Errorf("failed to bind max-timeout pflag: %w", err)
	}
	if err := viper.BindPFlag("authority", clientCmd.PersistentFlags().Lookup("authority")); err != nil {
		return nil, fmt.Errorf("failed to bind authority pflag: %w", err)
	}
	if err := viper.BindPFlag("insecure", clientCmd.PersistentFlags().Lookup("insecure")); err != nil {
		return nil, fmt.Errorf("failed to bind insecure pflag: %w", err)
	}
	return clientCmd, nil
}

// Client sub-command entrypoint. This function will launch gRPC requests for
// each of the fractional digits requested.
func clientMain(cmd *cobra.Command, endpoints []string) error {
	count := viper.GetInt("count")
	cacerts := viper.GetStringSlice("cacert")
	cert := viper.GetString("cert")
	key := viper.GetString("key")
	otlpTarget := viper.GetString("otlp-target")
	authority := viper.GetString("authority")
	maxTimeout := viper.GetDuration("max-timeout")
	insecure := viper.GetBool("insecure")
	logger := logger.V(1).WithValues("count", count, "endpoints", endpoints, "cacerts", cacerts, "cert", cert, "key", key, "otlpTarget", otlpTarget, "authority", authority, "maxTimeout", maxTimeout, "insecure", insecure)
	ctx := context.Background()
	options := []client.PiClientOption{
		client.WithLogger(logger),
		client.WithMaxTimeout(maxTimeout),
		client.WithTracer(otel.Tracer(ClientServiceName)),
		client.WithMeter(global.Meter(ClientServiceName)),
		client.WithPrefix(ClientServiceName),
		client.WithUserAgent(ClientServiceName + "/" + version),
		client.WithAuthority(authority),
	}

	logger.V(0).Info("Preparing gRPC transport credentials")
	certPool, err := newCACertPool(cacerts)
	if err != nil {
		return err
	}
	if insecure {
		options = append(options,
			client.WithTransportCredentials(grpcinsecure.NewCredentials()),
		)
	} else {
		clientTLSConfig, err := newTLSConfig(cert, key, nil, certPool)
		if err != nil {
			return err
		}
		options = append(options,
			client.WithTransportCredentials(credentials.NewTLS(clientTLSConfig)),
		)
	}

	logger.V(0).Info("Preparing telemetry")
	var otelCreds credentials.TransportCredentials
	if viper.GetBool("otlp-insecure") {
		otelCreds = grpcinsecure.NewCredentials()
	} else {
		otelTLSConfig, err := newTLSConfig(viper.GetString("otlp-cert"), viper.GetString("otlp-key"), nil, certPool)
		if err != nil {
			return err
		}
		otelCreds = credentials.NewTLS(otelTLSConfig)
	}
	shutdown, err := initTelemetry(ctx, ClientServiceName, otlpTarget, otelCreds,
		sdktrace.TraceIDRatioBased(viper.GetFloat64("otlp-sampling-ratio")),
	)
	if err != nil {
		return err
	}
	defer shutdown(ctx)
	logger.V(0).Info("Preparing to start client")
	piClient := client.NewPiClient(options...)
	// Randomize the retrieval of numbers
	indices := rand.Perm(count)
	digits := make([]byte, count)
	var wg sync.WaitGroup
	for i, index := range indices {
		endpoint := endpoints[i%len(endpoints)]
		wg.Add(1)
		go func(endpoint string, index uint64) {
			defer wg.Done()
			digit, err := piClient.FetchDigit(endpoint, index)
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
	if _, err := os.Stdout.Write(digits); err != nil {
		return fmt.Errorf("failure writing result: %w", err)
	}
	fmt.Println()
	return nil
}
