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
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric/global"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"
	xdscreds "google.golang.org/grpc/credentials/xds"
)

const (
	ClientServiceName  = "pi.client"
	DefaultDigitCount  = 100
	DefaultMaxTimeout  = 10 * time.Second
	CountFlagName      = "count"
	MaxTimeoutFlagName = "max-timeout"
	AuthorityFlagName  = "authority"
	InsecureFlagName   = "insecure"
	// A default round-robin configuration to use with client-side load balancer.
	DefaultRoundRobinConfig = `{"loadBalancingConfig": [{"round_robin":{}}]}`
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
	clientCmd.PersistentFlags().UintP(CountFlagName, "c", DefaultDigitCount, "The number of decimal digits of pi to request")
	clientCmd.PersistentFlags().DurationP(MaxTimeoutFlagName, "m", DefaultMaxTimeout, "The maximum timeout for a Pi Service request")
	clientCmd.PersistentFlags().String(AuthorityFlagName, "", "Set the authoritative name of the Pi Service target for TLS verification, overriding hostname")
	clientCmd.PersistentFlags().Bool(InsecureFlagName, false, "Disable TLS for gRPC connection to Pi Service")
	if err := viper.BindPFlag(CountFlagName, clientCmd.PersistentFlags().Lookup(CountFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind count pflag: %w", err)
	}
	if err := viper.BindPFlag(MaxTimeoutFlagName, clientCmd.PersistentFlags().Lookup(MaxTimeoutFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind max-timeout pflag: %w", err)
	}
	if err := viper.BindPFlag(AuthorityFlagName, clientCmd.PersistentFlags().Lookup(AuthorityFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind authority pflag: %w", err)
	}
	if err := viper.BindPFlag(InsecureFlagName, clientCmd.PersistentFlags().Lookup(InsecureFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind insecure pflag: %w", err)
	}
	return clientCmd, nil
}

// Client sub-command entrypoint. This function will launch gRPC requests for
// each of the fractional digits requested.
func clientMain(cmd *cobra.Command, endpoints []string) error {
	count := viper.GetInt(CountFlagName)
	logger := logger.V(1).WithValues(CountFlagName, count, "endpoints", endpoints)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shutdownFuncs := []shutdownFunction{}
	defer func(ctx context.Context) {
		for _, fn := range shutdownFuncs {
			if err := fn(ctx); err != nil {
				logger.Error(err, "Failure during service shutdown; continuing")
			}
		}
	}(ctx)

	logger.V(0).Info("Preparing OpenTelemetry")
	telemetryShutdownFuncs, err := initTelemetry(ctx, ClientServiceName,
		sdktrace.TraceIDRatioBased(viper.GetFloat64(OpenTelemetrySamplingRatioFlagName)),
	)
	if err != nil {
		return err
	}
	shutdownFuncs = append(telemetryShutdownFuncs, shutdownFuncs...)

	logger.V(0).Info("Preparing gRPC client connection")
	dialOptions, err := buildDialOptions(ctx)
	if err != nil {
		return err
	}
	conn, err := grpc.DialContext(ctx, endpoints[0], dialOptions...)
	if err != nil {
		return fmt.Errorf("failure establishing client dial context: %w", err)
	}
	defer conn.Close()

	logger.V(0).Info("Preparing PiService client")
	piClientOptions := []client.PiClientOption{
		client.WithLogger(logger),
		client.WithMaxTimeout(viper.GetDuration(MaxTimeoutFlagName)),
		client.WithTracer(otel.Tracer(ClientServiceName)),
		client.WithMeter(global.Meter(ClientServiceName)),
		client.WithPrefix(ClientServiceName),
	}
	piClient := client.NewPiClient(piClientOptions...)

	// Randomize the retrieval of numbers
	indices := rand.Perm(count)
	digits := make([]byte, count)
	var wg sync.WaitGroup
	for _, index := range indices {
		wg.Add(1)
		go func(conn *grpc.ClientConn, index uint64) {
			defer wg.Done()
			digit, err := piClient.FetchDigit(ctx, conn, index)
			if err != nil {
				logger.Error(err, "Error fetching digit", "index", index)
				digits[index] = '-'
			} else {
				digits[index] = '0' + byte(digit)
			}
		}(conn, uint64(index))
	}
	wg.Wait()
	fmt.Print("Result is: 3.")
	if _, err := os.Stdout.Write(digits); err != nil {
		return fmt.Errorf("failure writing result: %w", err)
	}
	fmt.Println()
	return nil
}

// Creates a set of gRPC DialOptions that are appropriate for the PiService client
// as determined by various command line flags.
func buildDialOptions(_ context.Context) ([]grpc.DialOption, error) {
	cacerts := viper.GetStringSlice(CACertFlagName)
	cert := viper.GetString(TLSCertFlagName)
	key := viper.GetString(TLSKeyFlagName)
	authority := viper.GetString(AuthorityFlagName)
	insecure := viper.GetBool(InsecureFlagName)
	logger := logger.V(1).WithValues(
		"cacerts", cacerts,
		"cert", cert,
		"key", key,
		"authority", authority,
		"insecure", insecure,
	)
	logger.V(0).Info("Preparing gRPC transport credentials")
	options := []grpc.DialOption{
		grpc.WithUserAgent(ClientServiceName + "/" + version),
		grpc.WithDefaultServiceConfig(DefaultRoundRobinConfig),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
	}
	certPool, err := newCACertPool(cacerts)
	if err != nil {
		return nil, err
	}
	var clientCreds credentials.TransportCredentials
	if insecure {
		clientCreds = grpcinsecure.NewCredentials()
	} else {
		clientTLSConfig, err := newTLSConfig(cert, key, nil, certPool)
		if err != nil {
			return nil, err
		}
		clientCreds = credentials.NewTLS(clientTLSConfig)
	}
	creds, err := xdscreds.NewClientCredentials(xdscreds.ClientOptions{FallbackCreds: clientCreds})
	if err != nil {
		return nil, err
	}
	options = append(options,
		grpc.WithTransportCredentials(creds),
	)
	if authority != "" {
		options = append(options, grpc.WithAuthority(authority))
	}
	return options, nil
}
