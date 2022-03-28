package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	api "github.com/memes/pi/v2/api/v2"
	"github.com/memes/pi/v2/pkg/cache"
	"github.com/memes/pi/v2/pkg/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric/global"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"golang.org/x/net/context"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/credentials"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"
)

const (
	ServerServiceName        = "pi.server"
	DefaultGRPCListenAddress = ":8443"
	RESTAddressFlagName      = "rest-address"
	RedisTargetFlagName      = "redis-target"
	LabelFlagName            = "label"
	TLSClientAuthFlagName    = "tls-client-auth"
	RESTAuthorityFlagName    = "rest-authority"
	XDSFlagName              = "xds"
)

// Implements the server sub-command.
func NewServerCmd() (*cobra.Command, error) {
	serverCmd := &cobra.Command{
		Use:   "server",
		Short: "Run gRPC service to return fractional digits of pi",
		Long: `Launches a gRPC Pi Service server that can calculate the decimal digits of pi.

A single decimal digit of pi will be returned per request. An optional Redis DB can be used to cache the calculated digits. Metrics and traces will be sent to an OpenTelemetry collection endpoint, if specified.`,
		Args: cobra.MaximumNArgs(1),
		RunE: serverMain,
	}
	serverCmd.PersistentFlags().String(RESTAddressFlagName, "", "An optional listen address to launch a REST/gRPC gateway process")
	serverCmd.PersistentFlags().String(RedisTargetFlagName, "", "An optional Redis endpoint to use as a PiService cache")
	serverCmd.PersistentFlags().StringToStringP(LabelFlagName, "l", nil, "An optional label key=value to add to PiService response metadata; can be repeated")
	serverCmd.PersistentFlags().Bool(TLSClientAuthFlagName, false, "Require PiService clients to provide a valid TLS client certificate")
	serverCmd.PersistentFlags().String(RESTAuthorityFlagName, "", "Set the Authority header for REST/gRPC gateway communication")
	serverCmd.PersistentFlags().Bool("xds", false, "Enable xDS for PiService; requires an xDS environment")
	if err := viper.BindPFlag(RESTAddressFlagName, serverCmd.PersistentFlags().Lookup(RESTAddressFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind rest-address pflag: %w", err)
	}
	if err := viper.BindPFlag(RedisTargetFlagName, serverCmd.PersistentFlags().Lookup(RedisTargetFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind redis-target pflag: %w", err)
	}
	if err := viper.BindPFlag(LabelFlagName, serverCmd.PersistentFlags().Lookup(LabelFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind label pflag: %w", err)
	}
	if err := viper.BindPFlag(TLSClientAuthFlagName, serverCmd.PersistentFlags().Lookup(TLSClientAuthFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind label pflag: %w", err)
	}
	if err := viper.BindPFlag(RESTAuthorityFlagName, serverCmd.PersistentFlags().Lookup(RESTAuthorityFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind rest-authority pflag: %w", err)
	}
	if err := viper.BindPFlag(XDSFlagName, serverCmd.PersistentFlags().Lookup(XDSFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind xds pflag: %w", err)
	}
	return serverCmd, nil
}

// Server sub-command entrypoint. This function will launch the gRPC PiService
// and an optional REST gateway.
func serverMain(cmd *cobra.Command, args []string) error {
	address := DefaultGRPCListenAddress
	if len(args) > 0 {
		address = args[0]
	}
	restAddress := viper.GetString(RESTAddressFlagName)
	redisTarget := viper.GetString(RedisTargetFlagName)
	cacerts := viper.GetStringSlice(CACertFlagName)
	cert := viper.GetString(TLSCertFlagName)
	key := viper.GetString(TLSKeyFlagName)
	requireTLSClientAuth := viper.GetBool(TLSClientAuthFlagName)
	otlpTarget := viper.GetString(OpenTelemetryTargetFlagName)
	labels := viper.GetStringMapString(LabelFlagName)
	restClientAuthority := viper.GetString(RESTAuthorityFlagName)
	xds := viper.GetBool(XDSFlagName)
	logger := logger.V(1).WithValues("address", address, "redisTarget", redisTarget, "restAddress", restAddress, "cacerts", cacerts, TLSCertFlagName, cert, TLSKeyFlagName, key, "requireTLSClientAuth", requireTLSClientAuth, "otlpTarget", otlpTarget, "labels", labels, "restClientAuthority", restClientAuthority, XDSFlagName, xds)
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

	logger.V(0).Info("Preparing telemetry")
	telemetryShutdownFuncs, err := initTelemetry(ctx, ServerServiceName,
		sdktrace.ParentBased(sdktrace.TraceIDRatioBased(viper.GetFloat64(OpenTelemetrySamplingRatioFlagName))),
	)
	if err != nil {
		return err
	}
	shutdownFuncs = append(telemetryShutdownFuncs, shutdownFuncs...)

	options := []server.PiServerOption{
		server.WithLogger(logger),
		server.WithMetadata(newMetadata(labels)),
		server.WithTracer(otel.Tracer(ServerServiceName)),
		server.WithMeter(global.Meter(ServerServiceName)),
		server.WithPrefix(ServerServiceName),
	}
	if redisTarget != "" {
		logger.V(0).Info("Adding Redis cache option to PiServer")
		options = append(options, server.WithCache(cache.NewRedisCache(ctx, redisTarget)))
	}

	logger.V(0).Info("Preparing gRPC transport security options")
	certPool, err := newCACertPool(cacerts)
	if err != nil {
		return err
	}
	var serverTLSConfig *tls.Config
	if cert != "" && key != "" {
		serverTLSConfig, err = newTLSConfig(cert, key, certPool, nil)
		if err != nil {
			return err
		}
		if requireTLSClientAuth {
			serverTLSConfig.ClientAuth = tls.RequireAndVerifyClientCert
		}
		clientTLSConfig, err := newTLSConfig(cert, key, nil, certPool)
		if err != nil {
			return err
		}
		options = append(options,
			server.WithGRPCServerTransportCredentials(credentials.NewTLS(serverTLSConfig)),
			server.WithRestClientGRPCTransportCredentials(credentials.NewTLS(clientTLSConfig)),
			server.WithRestClientAuthority(restClientAuthority),
		)
	} else {
		options = append(options,
			server.WithRestClientGRPCTransportCredentials(grpcinsecure.NewCredentials()),
		)
	}

	logger.V(0).Info("Preparing to start services")
	piServer := server.NewPiServer(options...)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to start gRPC listener: %w", err)
	}
	g, ctx := errgroup.WithContext(ctx)
	if xds {
		xdsServer := piServer.NewXDSServer()
		shutdownFuncs = append([]shutdownFunction{func(_ context.Context) error {
			xdsServer.GracefulStop()
			return nil
		}}, shutdownFuncs...)
		g.Go(func() error {
			logger.V(0).Info("Starting xDS gRPC service")
			if err := xdsServer.Serve(listener); err != nil {
				return fmt.Errorf("failed to start xDS gRPC server: %w", err)
			}
			return nil
		})
	} else {
		grpcServer := piServer.NewGrpcServer()
		shutdownFuncs = append([]shutdownFunction{func(_ context.Context) error {
			grpcServer.GracefulStop()
			return nil
		}}, shutdownFuncs...)
		g.Go(func() error {
			logger.V(0).Info("Starting gRPC service")
			if err := grpcServer.Serve(listener); err != nil {
				return fmt.Errorf("failed to start gRPC server: %w", err)
			}
			return nil
		})
	}
	if restAddress != "" {
		logger.V(0).Info("Preparing REST/gRPC gateway")
		restHandler, err := piServer.NewRestGatewayHandler(ctx, address)
		if err != nil {
			return fmt.Errorf("failed to create new REST gateway handler: %w", err)
		}
		restServer := &http.Server{
			Addr:      restAddress,
			Handler:   restHandler,
			TLSConfig: serverTLSConfig,
		}
		shutdownFuncs = append(shutdownFuncs, func(ctx context.Context) error {
			if err := restServer.Shutdown(ctx); err != nil {
				return fmt.Errorf("error returned by REST service shutdown: %w", err)
			}
			return nil
		})
		g.Go(func() error {
			if serverTLSConfig != nil {
				logger.V(0).Info("Starting REST/gRPC gateway with TLS")
				err = restServer.ListenAndServeTLS("", "")
			} else {
				logger.V(0).Info("Starting REST/gRPC gateway without TLS")
				err = restServer.ListenAndServe()
			}
			if !errors.Is(err, http.ErrServerClosed) {
				return fmt.Errorf("restServer listener returned an error: %w", err)
			}
			return nil
		})
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupt)
	select {
	case <-interrupt:
		break
	case <-ctx.Done():
		break
	}
	logger.V(0).Info("Shutting down on signal")
	cancel()
	ctx, cancelShutdown := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelShutdown()
	for _, fn := range shutdownFuncs {
		if err := fn(ctx); err != nil {
			logger.Error(err, "Failure during service shutdown; continuing")
		}
	}
	return g.Wait() //nolint:wrapcheck // Errors returned from group are already wrapped
}

// Creates a new metadata struct populated from labels given.
func newMetadata(labels map[string]string) *api.GetDigitMetadata {
	logger := logger.V(1).WithValues("labels", labels)
	logger.V(0).Info("Preparing metadata")
	var hostname string
	if host, err := os.Hostname(); err == nil {
		hostname = host
	} else {
		logger.Error(err, "Failed to get hostname; continuing")
		hostname = "unknown"
	}
	metadata := &api.GetDigitMetadata{
		Identity: hostname,
		Labels:   labels,
	}
	logger.V(1).Info("Metadata created", "metadata", metadata)
	return metadata
}
