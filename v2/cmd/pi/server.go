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
	"google.golang.org/grpc/credentials/insecure"
)

const (
	ServerServiceName        = "pi.server"
	DefaultGRPCListenAddress = ":8443"
)

// Implements the server sub-command.
func NewServerCmd() (*cobra.Command, error) {
	serverCmd := &cobra.Command{
		Use:   "server",
		Short: "Run gRPC service to return fractional digits of pi",
		Long: `Launches a gRPC Pi Service server that can calculate the decimal digits of pi.

A single decimal digit of pi will be returned per request. An optional Redis DB can be used to cache the calculated digits. Metrics and traces will be sent to an OpenTelemetry collection endpoint, if specified.`,
		RunE: serverMain,
	}
	serverCmd.PersistentFlags().StringP("address", "a", DefaultGRPCListenAddress, "Address to listen for gRPC PiService requests")
	serverCmd.PersistentFlags().String("rest-address", "", "An optional listen address to launch a REST/gRPC gateway process")
	serverCmd.PersistentFlags().String("redis-target", "", "An optional Redis endpoint to use as a PiService cache")
	serverCmd.PersistentFlags().StringToStringP("label", "l", nil, "An optional label key=value to add to PiService response metadata; can be repeated")
	serverCmd.PersistentFlags().Bool("tls-client-auth", false, "Require PiService clients to provide a valid TLS client certificate")
	serverCmd.PersistentFlags().String("rest-authority", "", "Set the Authority header for REST/gRPC gateway communication")
	serverCmd.PersistentFlags().Bool("xds", false, "Enable xDS for PiService; requires an xDS environment")
	if err := viper.BindPFlag("address", serverCmd.PersistentFlags().Lookup("address")); err != nil {
		return nil, fmt.Errorf("failed to bind address pflag: %w", err)
	}
	if err := viper.BindPFlag("rest-address", serverCmd.PersistentFlags().Lookup("rest-address")); err != nil {
		return nil, fmt.Errorf("failed to bind rest-address pflag: %w", err)
	}
	if err := viper.BindPFlag("redis-target", serverCmd.PersistentFlags().Lookup("redis-target")); err != nil {
		return nil, fmt.Errorf("failed to bind redis-target pflag: %w", err)
	}
	if err := viper.BindPFlag("label", serverCmd.PersistentFlags().Lookup("label")); err != nil {
		return nil, fmt.Errorf("failed to bind label pflag: %w", err)
	}
	if err := viper.BindPFlag("tls-client-auth", serverCmd.PersistentFlags().Lookup("tls-client-auth")); err != nil {
		return nil, fmt.Errorf("failed to bind label pflag: %w", err)
	}
	if err := viper.BindPFlag("rest-authority", serverCmd.PersistentFlags().Lookup("rest-authority")); err != nil {
		return nil, fmt.Errorf("failed to bind rest-authority pflag: %w", err)
	}
	if err := viper.BindPFlag("xds", serverCmd.PersistentFlags().Lookup("xds")); err != nil {
		return nil, fmt.Errorf("failed to bind xds pflag: %w", err)
	}
	return serverCmd, nil
}

// Server sub-command entrypoint. This function will launch the gRPC PiService
// and an optional REST gateway.
func serverMain(cmd *cobra.Command, args []string) error {
	address := viper.GetString("address")
	restAddress := viper.GetString("rest-address")
	redisTarget := viper.GetString("redis-target")
	cacerts := viper.GetStringSlice("cacert")
	cert := viper.GetString("cert")
	key := viper.GetString("key")
	requireTLSClientAuth := viper.GetBool("tls-client-auth")
	otlpTarget := viper.GetString("otlp-target")
	labels := viper.GetStringMapString("label")
	restClientAuthority := viper.GetString("rest-authority")
	xds := viper.GetBool("xds")
	logger := logger.V(1).WithValues("address", address, "redisTarget", redisTarget, "restAddress", restAddress, "cacerts", cacerts, "cert", cert, "key", key, "requireTLSClientAuth", requireTLSClientAuth, "otlpTarget", otlpTarget, "labels", labels, "restClientAuthority", restClientAuthority, "xds", xds)
	ctx := context.Background()
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

	logger.V(0).Info("Preparing gRPC transport credentials")
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
			server.WithRestClientGRPCTransportCredentials(insecure.NewCredentials()),
		)
	}

	logger.V(0).Info("Preparing telemetry")
	var otelCreds credentials.TransportCredentials
	if viper.GetBool("otlp-insecure") {
		otelCreds = insecure.NewCredentials()
	} else {
		otelTLSConfig, err := newTLSConfig(viper.GetString("otlp-cert"), viper.GetString("otlp-key"), nil, certPool)
		if err != nil {
			return err
		}
		otelCreds = credentials.NewTLS(otelTLSConfig)
	}
	shutdownFunctions, err := initTelemetry(ctx, ServerServiceName, otlpTarget, otelCreds,
		sdktrace.ParentBased(sdktrace.TraceIDRatioBased(viper.GetFloat64("otlp-sampling-ratio"))),
	)
	if err != nil {
		return err
	}

	logger.V(0).Info("Preparing to start services")
	piServer := server.NewPiServer(options...)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupt)

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to start gRPC listener: %w", err)
	}
	g, ctx := errgroup.WithContext(ctx)
	if xds {
		xdsServer := piServer.NewXDSServer()
		shutdownFunctions = append([]shutdownFunction{func(_ context.Context) error {
			xdsServer.GracefulStop()
			return nil
		}}, shutdownFunctions...)
		g.Go(func() error {
			logger.V(0).Info("Starting xDS gRPC service")
			if err := xdsServer.Serve(listener); err != nil {
				return fmt.Errorf("failed to start xDS gRPC server: %w", err)
			}
			return nil
		})
	} else {
		grpcServer := piServer.NewGrpcServer()
		shutdownFunctions = append([]shutdownFunction{func(_ context.Context) error {
			grpcServer.GracefulStop()
			return nil
		}}, shutdownFunctions...)
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
		shutdownFunctions = append(shutdownFunctions, func(ctx context.Context) error {
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

	select {
	case <-interrupt:
		break
	case <-ctx.Done():
		break
	}
	logger.V(0).Info("Shutting down on signal")
	cancel()
	ctx, shutdown := context.WithTimeout(context.Background(), 60*time.Second)
	defer shutdown()
	for _, fn := range shutdownFunctions {
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
