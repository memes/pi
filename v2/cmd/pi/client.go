package main

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/memes/pi/v2/pkg/client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	xdscreds "google.golang.org/grpc/credentials/xds"
)

const (
	ClientServiceName = "pi.client"
	DefaultDigitCount = 100
	DefaultMaxTimeout = 10 * time.Second
)

// Defines a function that will be called for each resulting.
type CollatorFunction func(index uint64, value uint32) error

// Contains a reference to the CollatorFunction that will be called for every
// result returned by a Pi service.
var collator CollatorFunction = noopCollator //nolint:gochecknoglobals // Global so that subcommands can set the CollatorFunction

// Implements the client sub-command which attempts to connect to one or
// more pi server instances and build up the digits of pi through multiple
// requests.
func NewClientCmd() (*cobra.Command, error) {
	clientCmd := &cobra.Command{
		Use:   "client target",
		Short: "Run a gRPC Pi Service client to request fractional digits of pi",
		Long:  "Launches a gRPC client that will connect to Pi Service target and request the fractional digits of pi.",
		Args:  cobra.ExactArgs(1),
		RunE:  clientMain,
	}
	clientCmd.PersistentFlags().Uint(CountFlagName, DefaultDigitCount, "The number of decimal digits of pi to accumulate")
	clientCmd.PersistentFlags().Duration(MaxTimeoutFlagName, DefaultMaxTimeout, "The maximum timeout for a Pi Service request")
	clientCmd.PersistentFlags().String(AuthorityFlagName, "", "Set the authoritative name of the remote server. This will also be used as the server name when verifying the server's TLS certificate")
	clientCmd.PersistentFlags().Bool(InsecureFlagName, false, "Disable TLS verification of gRPC connections to Pi Service")
	clientCmd.PersistentFlags().StringToString(HeaderFlagName, nil, "An optional header key=value to add to Pi Service request metadata; can be repeated")
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
	clientCmd.AddCommand(NewCollateCmd())
	return clientCmd, nil
}

// Client sub-command entrypoint. This function will launch gRPC requests for
// each of the fractional digits requested and pass the results into a collation
// function.
func clientMain(_ *cobra.Command, endpoints []string) error {
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
	creds, err := buildPiClientTransportCredentials()
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
	var wg sync.WaitGroup
	for _, index := range indices {
		wg.Add(1)
		go func(idx uint64) {
			defer wg.Done()
			digit, err := piClient.FetchDigit(ctx, idx)
			if err != nil {
				logger.Error(err, "Error fetching digit", "idx", idx)
				return
			}
			if err = collator(idx, digit); err != nil {
				logger.Error(err, "Error calling collator", "idx", idx, "digit", digit)
			}
		}(uint64(index)) //nolint:gosec // Risk of overflow is low
	}
	wg.Wait()
	return nil
}

// Creates the gRPC transport credentials that are appropriate for the PiService
// client as determined by various command line flags.
func buildPiClientTransportCredentials() (credentials.TransportCredentials, error) {
	if viper.GetBool(InsecureFlagName) {
		creds, err := xdscreds.NewClientCredentials(xdscreds.ClientOptions{FallbackCreds: insecure.NewCredentials()})
		if err != nil {
			return nil, fmt.Errorf("error generating xDS client credentials: %w", err)
		}
		return creds, nil
	}
	certPool, err := newCACertPool(viper.GetStringSlice(CACertFlagName))
	if err != nil {
		return nil, err
	}
	tlsConfig, err := newTLSConfig(viper.GetString(TLSCertFlagName), viper.GetString(TLSKeyFlagName), nil, certPool)
	if err != nil {
		return nil, err
	}
	creds, err := xdscreds.NewClientCredentials(xdscreds.ClientOptions{FallbackCreds: credentials.NewTLS(tlsConfig)})
	if err != nil {
		return nil, fmt.Errorf("error generating xDS client credentials: %w", err)
	}
	return creds, nil
}

// Do nothing CollatorFunction that ignores the results.
func noopCollator(_ uint64, _ uint32) error {
	return nil
}
