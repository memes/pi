package main

// spell-checker: ignore otelgrpc otel sdktrace otelhttp otelcodes

import (
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	pi "github.com/memes/pi/v2"
	api "github.com/memes/pi/v2/api/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/net/context"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

const (
	DEFAULT_GRPC_LISTEN_ADDRESS = ":9090"
	DEFAULT_REST_LISTEN_ADDRESS = ":8080"
)

var (
	metadata  *api.GetDigitMetadata
	serverCmd = &cobra.Command{
		Use:   "server",
		Short: "Run gRPC/REST service to return pi digits",
		Long: `Launches a gRPC service and REST gateway listening at the specified addresses for incoming client connections, and returns a digit of pi.

Also see 'client' command for usage.`,
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

type piServer struct {
	api.UnimplementedPiServiceServer
}

func (s *piServer) GetDigit(ctx context.Context, in *api.GetDigitRequest) (*api.GetDigitResponse, error) {
	index := in.Index
	logger := logger.WithValues("index", index)
	logger.Info("GetDigit: enter")
	ctx, span := otel.Tracer("server").Start(ctx, "GetDigit")
	defer span.End()
	span.SetAttributes(attribute.Int("index", int(index)))
	digit, err := pi.PiDigit(ctx, index)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return nil, err
	}
	logger.Info("GetDigit: exit", "digit", digit)
	return &api.GetDigitResponse{
		Index:    index,
		Digit:    digit,
		Metadata: metadata,
	}, nil
}

func (s *piServer) Check(ctx context.Context, in *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
}

func (s *piServer) Watch(in *grpc_health_v1.HealthCheckRequest, _ grpc_health_v1.Health_WatchServer) error {
	return status.Error(codes.Unimplemented, "unimplemented")
}

// Populate a metadata structure for this instance
func getMetadata() *api.GetDigitMetadata {
	logger.V(1).Info("Getting Metadata")
	metadata := api.GetDigitMetadata{
		Labels: viper.GetStringMapString("label"),
	}
	if hostname, err := os.Hostname(); err == nil {
		metadata.Identity = hostname
	}
	if addrs, err := net.InterfaceAddrs(); err == nil {
		addresses := make([]string, 0, len(addrs))
		for _, addr := range addrs {
			if net, ok := addr.(*net.IPNet); ok && !net.IP.IsLoopback() && !net.IP.IsMulticast() && !net.IP.IsLinkLocalUnicast() {
				addresses = append(addresses, net.IP.String())
			}
		}
		metadata.Addresses = addresses
	}
	logger.V(1).Info("Returning metadata", "metadata.identity", metadata.Identity, "metadata.addresses", metadata.Addresses, "metadata.labels", metadata.Labels)
	return &metadata
}

func service(cmd *cobra.Command, args []string) error {
	grpcAddress := viper.GetString("grpc-address")
	restAddress := viper.GetString("rest-address")
	enableREST := viper.GetBool("enable-rest")
	redisAddress := viper.GetString("redis-address")
	logger := logger.V(0).WithValues("grpcAddress", grpcAddress, "redisAddress", redisAddress, "restAddress", restAddress, "enableREST", enableREST)
	pi.SetLogger(logger)
	ctx := context.Background()
	if redisAddress != "" {
		pi.SetCache(pi.NewRedisCache(ctx, redisAddress))

	}
	metadata = getMetadata()
	logger.Info("Preparing telemetry")
	telemetryShutdown := initTelemetry(ctx, "server", sdktrace.AlwaysSample())

	logger.Info("Starting to listen")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupt)

	var grpcServer *grpc.Server
	var restServer *http.Server
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		listener, err := net.Listen("tcp", grpcAddress)
		if err != nil {
			return err
		}
		grpcServer = grpc.NewServer(
			grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
		)
		healthServer := health.NewServer()
		grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
		api.RegisterPiServiceServer(grpcServer, &piServer{})
		reflection.Register(grpcServer)
		return grpcServer.Serve(listener)
	})
	if enableREST {
		logger.Info("Enabling REST gateway")
		g.Go(func() error {
			mux := runtime.NewServeMux()
			opts := []grpc.DialOption{
				grpc.WithInsecure(),
				grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
			}
			if err := api.RegisterPiServiceHandlerFromEndpoint(ctx, mux, grpcAddress, opts); err != nil {
				return err
			}
			if err := mux.HandlePath("GET", "/v1/digit/{index}", func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
				span := trace.SpanFromContext(r.Context())
				span.SetStatus(otelcodes.Error, "v1 API")
				w.WriteHeader(http.StatusGone)
			}); err != nil {
				return err
			}
			restServer = &http.Server{
				Addr:    restAddress,
				Handler: otelhttp.NewHandler(mux, "grpc-gateway", otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents)),
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
	logger.Info("Shutting down on signal")
	cancel()
	ctx, shutdown := context.WithTimeout(context.Background(), 10*time.Second)
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
