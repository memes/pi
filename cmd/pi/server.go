package main

import (
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/mediocregopher/radix.v2/redis"
	"github.com/mediocregopher/radix.v2/sentinel"
	v2 "github.com/memes/pi/api/v2"
	"github.com/memes/pi/pkg"
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
	DEFAULT_SENTINEL_ADDRESS    = "sentinel-service:26379"
	DEFAULT_MASTER_NAME         = "master"
	DEFAULT_REDIS_ADDRESS       = "redis-service:6379"
	DEFAULT_USE_CACHE           = true
)

var (
	grpcAddress     string
	restAddress     string
	sentinelAddress string
	masterName      string
	redisAddress    string
	useCache        bool
	addresses       []string
	sentinelClient  *sentinel.Client
	poolSize        = 10
	serverCmd       = &cobra.Command{
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
	serverCmd.PersistentFlags().StringVarP(&sentinelAddress, "sentinel", "s", DEFAULT_SENTINEL_ADDRESS, "Address for Redis Sentinel instance")
	serverCmd.PersistentFlags().StringVarP(&masterName, "mastername", "m", DEFAULT_MASTER_NAME, "Name of master as configured in Redis Sentinels")
	serverCmd.PersistentFlags().StringVarP(&redisAddress, "redis", "r", DEFAULT_REDIS_ADDRESS, "Address for Redis instance")
	serverCmd.PersistentFlags().BoolVarP(&useCache, "cache", "c", DEFAULT_USE_CACHE, "Use Redis cache")
	_ = viper.BindPFlag("grpcaddress", serverCmd.PersistentFlags().Lookup("grpcaddress"))
	_ = viper.BindPFlag("restaddress", serverCmd.PersistentFlags().Lookup("restaddress"))
	_ = viper.BindPFlag("sentinel", serverCmd.PersistentFlags().Lookup("sentinel"))
	_ = viper.BindPFlag("mastername", serverCmd.PersistentFlags().Lookup("mastername"))
	_ = viper.BindPFlag("redisAddress", serverCmd.PersistentFlags().Lookup("redisAddress"))
	_ = viper.BindPFlag("cache", serverCmd.PersistentFlags().Lookup("cache"))
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
	digit, cached, err := cachedDigit(ctx, index)
	if err != nil {
		logger.Error("Error retrieving digit from cache",
			zap.Error(err),
		)
		return nil, err
	}
	if digit == "" {
		calcIndex := uint64(index/9) * 9
		digits := pkg.PiDigits(calcIndex)
		if useCache {
			err = writeDigits(ctx, calcIndex, digits)
		}
		if err != nil {
			logger.Error("Error writing digits to cache",
				zap.Error(err),
			)
			return nil, err
		}
		digit = string(digits[index%9])
	}
	logger.Debug("GetDigit: exit",
		zap.Bool("cached", cached),
		zap.String("digit", digit),
	)
	return &v2.GetDigitResponse{
		Index:     index,
		Digit:     digit,
		Cached:    cached,
		Addresses: addresses,
	}, nil
}

func cachedDigit(ctx context.Context, index uint64) (string, bool, error) {
	logger := logger.With(
		zap.Uint64("index", index),
	)
	logger.Debug("cachedDigit: enter")
	if !useCache {
		logger.Info("Cache is disabled, returning empty string")
		return "", false, nil
	}
	client, err := redis.Dial("tcp", redisAddress)
	if err != nil {
		logger.Error("Error connecting to Redis",
			zap.Error(err),
		)
		return "", false, err
	}
	defer client.Close()

	key := strconv.FormatUint(uint64(index/9), 10)
	digits, err := client.Cmd("GET", key).Str()
	if err != nil && err == redis.ErrRespNil {
		logger.Info("Digits are not cached",
			zap.String("key", key),
			zap.Error(err),
		)
		return "", false, nil
	}
	if err != nil {
		logger.Error("Error returned from Redis cache",
			zap.String("key", key),
			zap.Error(err),
		)
		return "", false, err
	}
	logger.Debug("Cache lookup returned",
		zap.String("digits", digits),
	)
	digit := string(digits[index%9])
	logger.Debug("cachedDigit: exit",
		zap.String("result", digit),
	)
	return digit, true, nil
}

func writeDigits(ctx context.Context, index uint64, digits string) error {
	logger := logger.With(
		zap.Uint64("index", index),
		zap.String("digits", digits),
	)
	logger.Debug("Attempting to write digits to cache")
	conn, err := sentinelClient.GetMaster(masterName)
	if err != nil {
		logger.Error("Error retrieving master connection from sentinel",
			zap.Error(err),
		)
		return err
	}
	defer sentinelClient.PutMaster(masterName, conn)
	key := strconv.FormatUint(uint64(index/9), 10)
	err = conn.Cmd("SET", key, digits).Err
	if err != nil {
		logger.Error("Error writing to redis instance",
			zap.String("key", key),
			zap.Error(err),
		)
		return err
	}
	return nil
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
		zap.Bool("useCache", useCache),
	)
	pkg.SetLogger(logger)
	logger.Debug("Preparing servers")
	addresses = getIPAddresses()
	if useCache {
		for {
			client, err := sentinel.NewClient("tcp", sentinelAddress, poolSize, masterName)
			if err == nil {
				sentinelClient = client
				break
			}
			logger.Warn("Unable to connect to sentinel, sleeping",
				zap.Error(err),
			)
			time.Sleep(5000 * time.Millisecond)
		}
	} else {
		logger.Info("Cache is disabled")
	}
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
