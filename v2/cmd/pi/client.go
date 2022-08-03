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
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
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
	HeaderFlagName     = "header"
)

// Implements the client sub-command which attempts to connect to one or
// more pi server instances and build up the digits of pi through multiple
// requests.
func NewClientCmd() (*cobra.Command, error) {
	clientCmd := &cobra.Command{
		Use:   "client target",
		Short: "Run a gRPC Pi Service client to request fractional digits of pi",
		Long: `Launches a gRPC client that will connect to Pi Service target and request the fractional digits of pi.

Metrics and traces will be sent to an OpenTelemetry collection endpoint, if specified.`,
		Args: cobra.ExactArgs(1),
		RunE: clientMain,
	}
	clientCmd.PersistentFlags().UintP(CountFlagName, "c", DefaultDigitCount, "The number of decimal digits of pi to accumulate")
	clientCmd.PersistentFlags().DurationP(MaxTimeoutFlagName, "m", DefaultMaxTimeout, "The maximum timeout for a Pi Service request")
	clientCmd.PersistentFlags().String(AuthorityFlagName, "", "Set the authoritative name of the remote server. This will also be used as the server name when verifying the server's TLS certificate")
	clientCmd.PersistentFlags().BoolP(InsecureFlagName, "k", false, "Disable TLS verification of gRPC connections to Pi Service")
	clientCmd.PersistentFlags().StringToStringP(HeaderFlagName, "H", nil, "An optional header key=value to add to Pi Service request metadata; can be repeated")
	if err := viper.BindPFlag(CountFlagName, clientCmd.PersistentFlags().Lookup(CountFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind %s pflag: %w", CountFlagName, err)
	}
	if err := viper.BindPFlag(MaxTimeoutFlagName, clientCmd.PersistentFlags().Lookup(MaxTimeoutFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind %s pflag: %w", MaxTimeoutFlagName, err)
	}
	if err := viper.BindPFlag(AuthorityFlagName, clientCmd.PersistentFlags().Lookup(AuthorityFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind %s pflag: %w", AuthorityFlagName, err)
	}
	if err := viper.BindPFlag(InsecureFlagName, clientCmd.PersistentFlags().Lookup(InsecureFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind %s pflag: %w", InsecureFlagName, err)
	}
	if err := viper.BindPFlag(HeaderFlagName, clientCmd.PersistentFlags().Lookup(HeaderFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind %s pflag: %w", HeaderFlagName, err)
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

	shutdownFunctions := &ShutdownFunctions{}
	defer shutdownFunctions.Execute(ctx, logger)

	logger.V(0).Info("Preparing OpenTelemetry")
	telemetryShutdownFuncs, err := initTelemetry(ctx, ClientServiceName,
		sdktrace.TraceIDRatioBased(viper.GetFloat64(OpenTelemetrySamplingRatioFlagName)),
	)
	if err != nil {
		return err
	}
	shutdownFunctions.Merge(telemetryShutdownFuncs)

	logger.V(0).Info("Preparing gRPC transport credentials")
	creds, err := buildTransportCredentials()
	if err != nil {
		return err
	}

	logger.V(0).Info("Preparing PiService client")
	piClient, err := client.NewPiClient(
		client.WithLogger(logger),
		client.WithMaxTimeout(viper.GetDuration(MaxTimeoutFlagName)),
		client.WithHeaders(viper.GetStringMapString(HeaderFlagName)),
		client.WithEndpoint(endpoints[0]),
		client.WithTransportCredentials(creds),
		client.WithAuthority(viper.GetString(AuthorityFlagName)),
		client.WithUserAgent(ClientServiceName+"/"+version),
	)
	if err != nil {
		return fmt.Errorf("failed to create new PiService client: %w", err)
	}
	shutdownFunctions.AppendFunction(piClient.Shutdown)

	// Randomize the retrieval of numbers
	indices := rand.Perm(count)
	digits := make([]byte, count)
	var wg sync.WaitGroup
	for _, index := range indices {
		wg.Add(1)
		go func(idx uint64) {
			defer wg.Done()
			digit, err := piClient.FetchDigit(ctx, idx)
			if err != nil {
				logger.Error(err, "Error fetching digit", "idx", idx)
				digits[idx] = '-'
			} else {
				digits[idx] = '0' + byte(digit)
			}
		}(uint64(index))
	}
	wg.Wait()
	fmt.Print("Result is: 3.")
	if _, err := os.Stdout.Write(digits); err != nil {
		return fmt.Errorf("failure writing result: %w", err)
	}
	fmt.Println()
	return nil
}

// Creates the gRPC transport credentials that are appropriate for the PiService
// client as determined by various command line flags.
func buildTransportCredentials() (credentials.TransportCredentials, error) {
	cacerts := viper.GetStringSlice(CACertFlagName)
	cert := viper.GetString(TLSCertFlagName)
	key := viper.GetString(TLSKeyFlagName)
	insecure := viper.GetBool(InsecureFlagName)
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
		return nil, fmt.Errorf("error generating xDS client credentials: %w", err)
	}
	return creds, nil
}
