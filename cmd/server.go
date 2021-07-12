package main

import (
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/memes/pi"
	v2 "github.com/memes/pi/api/v2"
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
	grpcAddress  string
	restAddress  string
	redisAddress string
	addresses    []string
	serverCmd    = &cobra.Command{
		Use:   "server",
		Short: "Run gRPC/REST service to return pi digits",
		Long: `Launches a gRPC service and REST gateway listening at the specified addresses for incoming client connections, and returns a digit of pi.

Also see 'client' command for usage.`,
		RunE: service,
	}
)

func init() {
	serverCmd.PersistentFlags().StringVarP(&grpcAddress, "grpcaddress", "g", DEFAULT_GRPC_LISTEN_ADDRESS, "Address to use to listen for gRPC connections")
	serverCmd.PersistentFlags().StringVarP(&restAddress, "restaddress", "a", DEFAULT_REST_LISTEN_ADDRESS, "Address to use to listen for REST connections")
	serverCmd.PersistentFlags().StringVarP(&redisAddress, "redis", "r", "", "Address for Redis instance")
	_ = viper.BindPFlag("grpcaddress", serverCmd.PersistentFlags().Lookup("grpcaddress"))
	_ = viper.BindPFlag("restaddress", serverCmd.PersistentFlags().Lookup("restaddress"))
	_ = viper.BindPFlag("redisAddress", serverCmd.PersistentFlags().Lookup("redisAddress"))
	rootCmd.AddCommand(serverCmd)
}

type piServer struct {
	v2.UnimplementedPiServiceServer
}

func (s *piServer) GetDigit(ctx context.Context, in *v2.GetDigitRequest) (*v2.GetDigitResponse, error) {
	index := in.Index
	logger := logger.With(
		zap.Uint64("index", index),
	)
	logger.Debug("GetDigit: enter")

	if redisAddress != "" {
		pi.SetCache(NewRedisCache(ctx, redisAddress))

	}
	digit, err := pi.PiDigits(ctx, index)
	if err != nil {
		logger.Error("Error retrieving digit",
			zap.Error(err),
		)
		return nil, err
	}
	logger.Debug("GetDigit: exit",
		zap.String("digit", digit),
	)
	return &v2.GetDigitResponse{
		Index:     index,
		Digit:     digit,
		Addresses: addresses,
	}, nil
}

// Returns all non-loopback IP addresses found
func getIPAddresses() []string {
	logger.Debug("Getting IP addresses")
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		logger.Error("Error getting interface addresses, returning empty string",
			zap.Error(err),
		)
		return []string{}
	}
	addresses := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		if net, ok := addr.(*net.IPNet); ok && !net.IP.IsLoopback() && !net.IP.IsMulticast() && !net.IP.IsLinkLocalUnicast() {
			addresses = append(addresses, net.IP.String())
		}
	}
	logger.Debug("Returning IP addresses",
		zap.Strings("addresses", addresses),
	)
	return addresses
}

func service(cmd *cobra.Command, args []string) error {
	logger := logger.With(
		zap.String("grpcAddress", grpcAddress),
		zap.String("restAddress", restAddress),
	)
	pi.SetLogger(logger)
	logger.Debug("Preparing servers")
	addresses = getIPAddresses()
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
		v2.RegisterPiServiceServer(grpcServer, &piServer{})
		reflection.Register(grpcServer)
		healthServer.SetServingStatus("grpc.health.v1.Health", grpc_health_v1.HealthCheckResponse_SERVING)
		return grpcServer.Serve(listener)
	})
	g.Go(func() error {
		mux := runtime.NewServeMux()
		opts := []grpc.DialOption{grpc.WithInsecure()}
		if err := v2.RegisterPiServiceHandlerFromEndpoint(ctx, mux, grpcAddress, opts); err != nil {
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

	select {
	case <-interrupt:
		break
	case <-ctx.Done():
		break
	}
	logger.Info("Shutting down on signal")
	healthServer.SetServingStatus("grpc.health.v1.Health", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
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
