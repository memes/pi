package main

import (
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	piserver "github.com/memes/pi/v2/api/v2/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric/global"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"golang.org/x/net/context"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

const (
	SERVER_SERVICE_NAME         = "server"
	DEFAULT_GRPC_LISTEN_ADDRESS = ":9090"
	DEFAULT_REST_LISTEN_ADDRESS = ":8080"
)

var (
	// Implements the server sub-command
	serverCmd = &cobra.Command{
		Use:   SERVER_SERVICE_NAME,
		Short: "Run gRPC service to return fractional digits of pi",
		Long: `Launches a gRPC service and optional REST gateway listening at the specified addresses for incoming client connections.
A single fractional digit of pi will be returned per request. Metrics and traces will be sent to an optionally provided OpenTelemetry collection endpoint.`,
		RunE: service,
	}
)

func init() {
	serverCmd.PersistentFlags().StringP("grpc-address", "g", DEFAULT_GRPC_LISTEN_ADDRESS, "Address to listen for gRPC connections")
	serverCmd.PersistentFlags().StringP("rest-address", "a", DEFAULT_REST_LISTEN_ADDRESS, "Address to listen for REST connections")
	serverCmd.PersistentFlags().StringP("redis-address", "r", "", "Address for Redis instance")
	serverCmd.PersistentFlags().StringToStringP("label", "l", nil, "Optional label key=value to apply to server. Can be repeated.")
	serverCmd.PersistentFlags().BoolP("enable-rest", "e", false, "Enable REST gateway for gRPC service")
	_ = viper.BindPFlag("grpc-address", serverCmd.PersistentFlags().Lookup("grpc-address"))
	_ = viper.BindPFlag("rest-address", serverCmd.PersistentFlags().Lookup("rest-address"))
	_ = viper.BindPFlag("redis-address", serverCmd.PersistentFlags().Lookup("redis-address"))
	_ = viper.BindPFlag("label", serverCmd.PersistentFlags().Lookup("label"))
	_ = viper.BindPFlag("enable-rest", serverCmd.PersistentFlags().Lookup("enable-rest"))
	rootCmd.AddCommand(serverCmd)
}

// Server sub-command entrypoint. This function will launch the gRPC PiService
// and an optional REST gateway.
func service(cmd *cobra.Command, args []string) error {
	grpcAddress := viper.GetString("grpc-address")
	restAddress := viper.GetString("rest-address")
	enableREST := viper.GetBool("enable-rest")
	redisAddress := viper.GetString("redis-address")
	logger := logger.V(0).WithValues("grpcAddress", grpcAddress, "redisAddress", redisAddress, "restAddress", restAddress, "enableREST", enableREST)
	ctx := context.Background()
	logger.V(1).Info("Preparing telemetry")
	telemetryShutdown := initTelemetry(ctx, SERVER_SERVICE_NAME, sdktrace.AlwaysSample())
	var cache piserver.Cache
	if redisAddress != "" {
		cache = piserver.NewRedisCache(ctx, redisAddress)

	}
	server := piserver.NewPiServer(
		piserver.WithLogger(logger),
		piserver.WithCache(cache),
		piserver.WithMetadata(viper.GetStringMapString("label")),
		piserver.WithTracer(otel.Tracer(SERVER_SERVICE_NAME)),
		piserver.WithMeter(SERVER_SERVICE_NAME, global.Meter(SERVER_SERVICE_NAME)),
	)

	logger.V(1).Info("Preparing services")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupt)

	var grpcServer *grpc.Server
	var restServer *http.Server
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		logger.V(1).Info("Starting gRPC service")
		grpcServer = server.NewGrpcServer()
		listener, err := net.Listen("tcp", grpcAddress)
		if err != nil {
			return err
		}
		return grpcServer.Serve(listener)
	})
	if enableREST {
		g.Go(func() error {
			logger.V(1).Info("Starting REST/gRPC gateway")
			var err error
			restServer, err = server.NewRestGatewayServer(ctx, restAddress, grpcAddress)
			if err != nil {
				return err
			}
			if err := restServer.ListenAndServe(); err != http.ErrServerClosed {
				return err
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
	logger.V(1).Info("Shutting down on signal")
	cancel()
	ctx, shutdown := context.WithTimeout(context.Background(), 60*time.Second)
	defer shutdown()
	if restServer != nil {
		_ = restServer.Shutdown(ctx)
	}
	if grpcServer != nil {
		grpcServer.GracefulStop()
	}
	telemetryShutdown(ctx)
	return g.Wait()
}
