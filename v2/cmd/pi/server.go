package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/memes/pi/v2/pkg/cache"
	"github.com/memes/pi/v2/pkg/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric/global"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"golang.org/x/net/context"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	ServerServiceName        = "server"
	DefaultGRPCListenAddress = ":443"
)

// Implements the server sub-command.
func NewServerCmd() (*cobra.Command, error) {
	serverCmd := &cobra.Command{
		Use:   ServerServiceName,
		Short: "Run gRPC service to return fractional digits of pi",
		Long: `Launches a gRPC Pi Service server that can calculate the decimal digits of pi.

A single decimal digit of pi will be returned per request. An optional Redis DB can be used to cache the calculated digits. Metrics and traces will be sent to an OpenTelemetry collection endpoint, if specified.`,
		RunE: serverMain,
	}
	serverCmd.PersistentFlags().StringP("address", "a", DefaultGRPCListenAddress, "Address to listen for gRPC PiService requests")
	serverCmd.PersistentFlags().String("rest-address", "", "An optional listen address to launch a REST/gRPC gateway process")
	serverCmd.PersistentFlags().String("redis-target", "", "An optional Redis endpoint to use as a PiService cache")
	serverCmd.PersistentFlags().StringToStringP("label", "l", nil, "An optional label key=value to add to PiService response metadata; can be repeated")
	serverCmd.PersistentFlags().String("cacert", "", "An optional CA certificate to use for PiService client TLS verification")
	serverCmd.PersistentFlags().String("cert", "", "An optional client TLS certificate to use with PiService")
	serverCmd.PersistentFlags().String("key", "", "An optional client TLS private key to use with PiService")
	serverCmd.PersistentFlags().Bool("tls-client-auth", false, "Require PiService clients to provide a valid TLS client certificate")
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
	if err := viper.BindPFlag("cacert", serverCmd.PersistentFlags().Lookup("cacert")); err != nil {
		return nil, fmt.Errorf("failed to bind cacert pflag: %w", err)
	}
	if err := viper.BindPFlag("cert", serverCmd.PersistentFlags().Lookup("cert")); err != nil {
		return nil, fmt.Errorf("failed to bind cert pflag: %w", err)
	}
	if err := viper.BindPFlag("key", serverCmd.PersistentFlags().Lookup("key")); err != nil {
		return nil, fmt.Errorf("failed to bind key pflag: %w", err)
	}
	if err := viper.BindPFlag("tls-client-auth", serverCmd.PersistentFlags().Lookup("tls-client-auth")); err != nil {
		return nil, fmt.Errorf("failed to bind tls-client-auth pflag: %w", err)
	}
	return serverCmd, nil
}

// Server sub-command entrypoint. This function will launch the gRPC PiService
// and an optional REST gateway.
func serverMain(cmd *cobra.Command, args []string) error {
	address := viper.GetString("address")
	restAddress := viper.GetString("rest-address")
	redisTarget := viper.GetString("redis-target")
	logger := logger.V(1).WithValues("address", address, "redisTarget", redisTarget, "restAddress", restAddress)
	ctx := context.Background()
	logger.V(0).Info("Preparing telemetry")
	telemetryShutdown := initTelemetry(ctx, ServerServiceName, sdktrace.AlwaysSample())

	logger.V(0).Info("Preparing services")
	options := []server.PiServerOption{
		server.WithLogger(logger),
		server.WithMetadata(viper.GetStringMapString("label")),
		server.WithTracer(otel.Tracer(ServerServiceName)),
		server.WithMeter(global.Meter(ServerServiceName)),
		server.WithPrefix(ServerServiceName),
	}
	if redisTarget != "" {
		options = append(options, server.WithCache(cache.NewRedisCache(ctx, redisTarget)))
	}
	tlsCreds, err := newServerTLSCredentials()
	if err != nil {
		return err
	}
	options = append(options, server.WithTransportCredentials(tlsCreds))
	piServer := server.NewPiServer(options...)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupt)

	var grpcServer *grpc.Server
	var restServer *http.Server
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		logger.V(0).Info("Starting gRPC service")
		grpcServer = piServer.NewGrpcServer()
		listener, err := net.Listen("tcp", address)
		if err != nil {
			return fmt.Errorf("failed to start gRPC listener: %w", err)
		}
		if err := grpcServer.Serve(listener); err != nil {
			return fmt.Errorf("failed to start gRPC server: %w", err)
		}
		return nil
	})
	if restAddress != "" {
		g.Go(func() error {
			logger.V(0).Info("Starting REST/gRPC gateway")
			restHandler, err := piServer.NewRestGatewayHandler(ctx, address)
			if err != nil {
				return fmt.Errorf("failed to create new REST gateway handler: %w", err)
			}
			restServer = &http.Server{
				Addr:    restAddress,
				Handler: restHandler,
			}
			if err := restServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
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
	if restServer != nil {
		if err := restServer.Shutdown(ctx); err != nil {
			logger.Error(err, "Failed to shutdown REST gateway cleanly")
		}
	}
	if grpcServer != nil {
		grpcServer.GracefulStop()
	}
	telemetryShutdown(ctx)
	return g.Wait() // nolint:wrapcheck
}

// Creates the gRPC transport credentials to use with PiService server from the
// various configuration options provided.
func newServerTLSCredentials() (credentials.TransportCredentials, error) {
	var tlsConf tls.Config
	certFile := viper.GetString("cert")
	keyFile := viper.GetString("key")
	cacertFile := viper.GetString("cacert")
	tlsClientAuth := viper.GetBool("tls-client-auth")
	logger := logger.V(1).WithValues("certFile", certFile, "keyFile", keyFile, "cacertFile", cacertFile)
	logger.V(0).Info("Preparing server TLS credentials")
	if certFile != "" {
		logger.V(1).Info("Loading x509 certificate and key")
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read certificate and key from files %s %s: %w", certFile, keyFile, err)
		}
		tlsConf.Certificates = []tls.Certificate{cert}
	}

	if cacertFile != "" {
		logger.V(1).Info("Loading CA from file")
		ca, err := ioutil.ReadFile(cacertFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate from file %s: %w", cacertFile, err)
		}
		certPool := x509.NewCertPool()
		if ok := certPool.AppendCertsFromPEM(ca); !ok {
			logger.V(0).Info("Failed to append CA cert %s to CA pool", cacertFile)
			return nil, errFailedToAppendCACert
		}
		tlsConf.ClientCAs = certPool
	}

	switch {
	case tlsClientAuth:
		tlsConf.ClientAuth = tls.RequireAndVerifyClientCert

	case cacertFile != "":
		tlsConf.ClientAuth = tls.VerifyClientCertIfGiven

	default:
		tlsConf.ClientAuth = tls.NoClientCert
	}
	return credentials.NewTLS(&tlsConf), nil
}
