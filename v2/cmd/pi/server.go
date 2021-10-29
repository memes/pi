package main

import (
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	api "github.com/memes/pi/api/v2"
	pi "github.com/memes/pi/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
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
	logger := logger.With(
		zap.Uint64("index", index),
	)
	logger.Debug("GetDigit: enter")

	redisAddress := viper.GetString("redis-address")
	if redisAddress != "" {
		pi.SetCache(pi.NewRedisCache(ctx, redisAddress))

	}
	digit, err := pi.PiDigit(ctx, index)
	if err != nil {
		logger.Error("Error retrieving digit",
			zap.Error(err),
		)
		return nil, err
	}
	logger.Debug("GetDigit: exit",
		zap.String("digit", digit),
	)
	return &api.GetDigitResponse{
		Index:    index,
		Digit:    digit,
		Metadata: metadata,
	}, nil
}

// Populate a metadata structure for this instance
func getMetadata() *api.GetDigitMetadata {
	logger.Debug("Getting Metadata")
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
	return &metadata
}

func service(cmd *cobra.Command, args []string) error {
	grpcAddress := viper.GetString("grpc-address")
	restAddress := viper.GetString("rest-address")
	enableREST := viper.GetBool("enable-rest")
	logger := logger.With(
		zap.String("grpcAddress", grpcAddress),
		zap.String("restAddress", restAddress),
		zap.Bool("enableREST", enableREST),
	)
	pi.SetLogger(logger)
	logger.Debug("Preparing servers")
	metadata = getMetadata()
	logger.Debug("Starting to listen")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupt)

	var grpcServer *grpc.Server
	var restServer *http.Server
	g, ctx := errgroup.WithContext(ctx)
	healthServer := health.NewServer()
	g.Go(func() error {
		listener, err := net.Listen("tcp", grpcAddress)
		if err != nil {
			return err
		}
		grpcServer = grpc.NewServer()
		grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
		api.RegisterPiServiceServer(grpcServer, &piServer{})
		reflection.Register(grpcServer)
		healthServer.SetServingStatus("api.v2.PiService", grpc_health_v1.HealthCheckResponse_SERVING)
		return grpcServer.Serve(listener)
	})
	if enableREST {
		logger.Debug("Enabling REST gateway")
		g.Go(func() error {
			mux := runtime.NewServeMux()
			opts := []grpc.DialOption{grpc.WithInsecure()}
			if err := api.RegisterPiServiceHandlerFromEndpoint(ctx, mux, grpcAddress, opts); err != nil {
				return err
			}
			if err := mux.HandlePath("GET", "/v1/digit/{index}", func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
				w.WriteHeader(http.StatusGone)
			}); err != nil {
				return err
			}
			restServer = &http.Server{
				Addr:    restAddress,
				Handler: mux,
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
	healthServer.SetServingStatus("api.v2.PiService", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	cancel()
	ctx, shutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdown()
	if restServer != nil {
		_ = restServer.Shutdown(ctx)
	}
	if grpcServer != nil {
		grpcServer.GracefulStop()
	}
	return g.Wait()
}
