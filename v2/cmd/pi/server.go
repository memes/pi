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

	"github.com/memes/pi/v2/pkg/cache"
	"github.com/memes/pi/v2/pkg/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"golang.org/x/net/context"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/credentials"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"
	xdscreds "google.golang.org/grpc/credentials/xds"
)

const (
	ServerServiceName        = "pi.server"
	DefaultGRPCListenAddress = ":8443"
	RESTAddressFlagName      = "rest-address"
	RedisTargetFlagName      = "redis-target"
	TagFlagName              = "tag"
	AnnotationFlagName       = "annotation"
	MutualTLSFlagName        = "mtls"
	RESTAuthorityFlagName    = "rest-authority"
	XDSFlagName              = "xds"
	DefaultReadHeaderTimeout = 10 * time.Second
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
	serverCmd.PersistentFlags().String(RedisTargetFlagName, "", "An optional Redis endpoint to use as a Pi Service cache")
	serverCmd.PersistentFlags().StringArray(TagFlagName, nil, "An optional string tag to add to Pi Service response metadata; can be repeated")
	serverCmd.PersistentFlags().StringToString(AnnotationFlagName, nil, "An optional key=value annotation to add to Pi Service response metadata; can be repeated")
	serverCmd.PersistentFlags().Bool(MutualTLSFlagName, false, "Enforce mutual TLS authentication for Pi Service gRPC clients")
	serverCmd.PersistentFlags().String(RESTAuthorityFlagName, "", "Set the Authority header for REST/gRPC gateway communication")
	serverCmd.PersistentFlags().Bool("xds", false, "Enable xDS for Pi Service; requires an xDS environment")
	if err := viper.BindPFlag(RESTAddressFlagName, serverCmd.PersistentFlags().Lookup(RESTAddressFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind %s pflag: %w", RESTAddressFlagName, err)
	}
	if err := viper.BindPFlag(RedisTargetFlagName, serverCmd.PersistentFlags().Lookup(RedisTargetFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind %s pflag: %w", RedisTargetFlagName, err)
	}
	if err := viper.BindPFlag(TagFlagName, serverCmd.PersistentFlags().Lookup(TagFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind %s pflag: %w", TagFlagName, err)
	}
	if err := viper.BindPFlag(AnnotationFlagName, serverCmd.PersistentFlags().Lookup(AnnotationFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind %s pflag: %w", AnnotationFlagName, err)
	}
	if err := viper.BindPFlag(MutualTLSFlagName, serverCmd.PersistentFlags().Lookup(MutualTLSFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind %s pflag: %w", MutualTLSFlagName, err)
	}
	if err := viper.BindPFlag(RESTAuthorityFlagName, serverCmd.PersistentFlags().Lookup(RESTAuthorityFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind %s pflag: %w", RESTAuthorityFlagName, err)
	}
	if err := viper.BindPFlag(XDSFlagName, serverCmd.PersistentFlags().Lookup(XDSFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind %s pflag: %w", XDSFlagName, err)
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
	mTLS := viper.GetBool(MutualTLSFlagName)
	otlpTarget := viper.GetString(OpenTelemetryTargetFlagName)
	tags := viper.GetStringSlice(TagFlagName)
	annotations := viper.GetStringMapString(AnnotationFlagName)
	restClientAuthority := viper.GetString(RESTAuthorityFlagName)
	xds := viper.GetBool(XDSFlagName)
	logger := logger.V(1).WithValues("address", address, "redisTarget", redisTarget, "restAddress", restAddress, "cacerts", cacerts, TLSCertFlagName, cert, TLSKeyFlagName, key, "mTLS", mTLS, "otlpTarget", otlpTarget, "tags", tags, "annotations", annotations, "restClientAuthority", restClientAuthority, XDSFlagName, xds)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shutdownFunctions := ShutdownFunctions{}
	defer shutdownFunctions.Execute(ctx, logger)

	logger.V(0).Info("Preparing telemetry")
	telemetryShutdownFuncs, err := initTelemetry(ctx, ServerServiceName,
		sdktrace.ParentBased(sdktrace.TraceIDRatioBased(viper.GetFloat64(OpenTelemetrySamplingRatioFlagName))),
	)
	if err != nil {
		return err
	}
	shutdownFunctions.Merge(telemetryShutdownFuncs)

	options := []server.PiServerOption{
		server.WithLogger(logger),
		server.WithTags(tags),
		server.WithAnnotations(annotations),
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
		if mTLS {
			serverTLSConfig.ClientAuth = tls.RequireAndVerifyClientCert
		}
		clientTLSConfig, err := newTLSConfig(cert, key, nil, certPool)
		if err != nil {
			return err
		}
		serverCreds := credentials.NewTLS(serverTLSConfig)
		clientCreds := credentials.NewTLS(clientTLSConfig)
		if xds {
			serverCreds, err = xdscreds.NewServerCredentials(xdscreds.ServerOptions{FallbackCreds: serverCreds})
			if err != nil {
				return fmt.Errorf("error creating xDS server credentials: %w", err)
			}
			clientCreds, err = xdscreds.NewClientCredentials(xdscreds.ClientOptions{FallbackCreds: clientCreds})
			if err != nil {
				return fmt.Errorf("error creating xDS client credentials: %w", err)
			}
		}
		options = append(options,
			server.WithGRPCServerTransportCredentials(serverCreds),
			server.WithRestClientGRPCTransportCredentials(clientCreds),
			server.WithRestClientAuthority(restClientAuthority),
		)
	} else {
		clientCreds := grpcinsecure.NewCredentials()
		if xds {
			clientCreds, err = xdscreds.NewClientCredentials(xdscreds.ClientOptions{FallbackCreds: clientCreds})
			if err != nil {
				return fmt.Errorf("error creating xDS client credentials with insecure fallback: %w", err)
			}
		}
		options = append(options,
			server.WithRestClientGRPCTransportCredentials(clientCreds),
		)
	}

	logger.V(0).Info("Preparing to start services")
	piServer, err := server.NewPiServer(options...)
	if err != nil {
		return fmt.Errorf("failed to create new PiService server: %w", err)
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to start gRPC listener: %w", err)
	}
	g, ctx := errgroup.WithContext(ctx)
	if xds {
		xdsServer := piServer.NewXDSServer()
		shutdownFunctions.AppendFunction(func(_ context.Context) error {
			xdsServer.GracefulStop()
			return nil
		})
		g.Go(func() error {
			logger.V(0).Info("Starting xDS gRPC service")
			if err := xdsServer.Serve(listener); err != nil {
				return fmt.Errorf("failed to start xDS gRPC server: %w", err)
			}
			return nil
		})
	} else {
		grpcServer := piServer.NewGrpcServer()
		shutdownFunctions.AppendFunction(func(_ context.Context) error {
			grpcServer.GracefulStop()
			return nil
		})
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
			Addr:              restAddress,
			Handler:           restHandler,
			TLSConfig:         serverTLSConfig,
			ReadHeaderTimeout: DefaultReadHeaderTimeout,
		}
		shutdownFunctions.AppendFunctions([]ShutdownFunction{func(ctx context.Context) error {
			if err := restServer.Shutdown(ctx); err != nil {
				return fmt.Errorf("error returned by REST service shutdown: %w", err)
			}
			return nil
		}})
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
	ctx, cancelShutdown := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelShutdown()
	shutdownFunctions.Execute(ctx, logger)
	cancel()
	return g.Wait() //nolint:wrapcheck // Errors returned from group are already wrapped
}
