// Implements a gRPC service that returns a set of digits of pi starting at a
// specified index.
package cmd

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/mediocregopher/radix.v2/redis"
	"github.com/mediocregopher/radix.v2/sentinel"
	"github.com/memes/pi/transfer"
	"github.com/rs/xhandler"
	"github.com/rs/xmux"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

const (
	DEFAULT_LISTEN_ADDRESS   = ":8080"
	DEFAULT_SENTINEL_ADDRESS = "sentinel-service:26379"
	DEFAULT_MASTER_NAME      = "master"
	DEFAULT_REDIS_ADDRESS    = "redis-service:6379"
)

var (
	address         string
	sentinelAddress string
	masterName      string
	redisAddress    string
	sentinelClient  *sentinel.Client
	poolSize        = 10
	serverCmd       = &cobra.Command{
		Use:   "server",
		Short: "Run a JSON servive to return pi digits",
		Long: `Launches an HTTP server listening at the specified addresses for incoming client connections, and returns a digit of pi. 

Also see 'client' command for usage.`,
		Run: func(cmd *cobra.Command, args []string) {
			logger := Logger.With(
				zap.String("address", address),
			)
			logger.Debug("Starting server")
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
			logger.Debug("Starting to listen")
			chain := xhandler.Chain{}
			chain.UseC(xhandler.CloseHandler)
			chain.UseC(xhandler.TimeoutHandler(120 * time.Second))
			mux := xmux.New()
			mux.GET("/v1/digit/:index", xhandler.HandlerFuncC(getDigit))
			mux.GET("/healthz", xhandler.HandlerFuncC(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, "OK")
			}))
			err := http.ListenAndServe(address, chain.Handler(mux))
			if err != nil {
				logger.Error("Error returned from Serve",
					zap.Error(err),
				)
			}
		},
	}
)

func init() {
	serverCmd.PersistentFlags().StringVarP(&address, "address", "a", DEFAULT_LISTEN_ADDRESS, "Address to use to listen for HTTP connections")
	serverCmd.PersistentFlags().StringVarP(&sentinelAddress, "sentinel", "s", DEFAULT_SENTINEL_ADDRESS, "Address for Redis Sentinel instance")
	serverCmd.PersistentFlags().StringVarP(&masterName, "mastername", "m", DEFAULT_MASTER_NAME, "Name of master as configured in Redis Sentinels")
	serverCmd.PersistentFlags().StringVarP(&redisAddress, "redis", "r", DEFAULT_REDIS_ADDRESS, "Address for Redis instance")
	viper.BindPFlag("address", serverCmd.PersistentFlags().Lookup("address"))
	viper.BindPFlag("sentinel", serverCmd.PersistentFlags().Lookup("sentinel"))
	viper.BindPFlag("mastername", serverCmd.PersistentFlags().Lookup("mastername"))
	viper.BindPFlag("redisAddress", serverCmd.PersistentFlags().Lookup("redisAddress"))
	RootCmd.AddCommand(serverCmd)
}

func getDigit(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	index, err := strconv.ParseInt(xmux.Param(ctx, "index"), 10, 64)
	if err != nil {
		Logger.Error("Error parsing index from params",
			zap.Error(err),
		)
		transfer.MarshalError(ctx, w, http.StatusBadRequest)
		return
	}
	logger := Logger.With(
		zap.Int64("index", index),
	)
	logger.Debug("GetDigit: enter")
	digit, err := cachedDigit(ctx, index)
	if err != nil {
		logger.Error("Error retrieving digit from cache",
			zap.Error(err),
		)
		transfer.MarshalError(ctx, w, http.StatusInternalServerError)
		return
	}
	if digit == "" {
		calcIndex := int64(index/9) * 9
		digits := piDigits(calcIndex)
		err = writeDigits(ctx, calcIndex, digits)
		if err != nil {
			logger.Error("Error writing digits to cache",
				zap.Error(err),
			)
			transfer.MarshalError(ctx, w, http.StatusInternalServerError)
			return
		}
		digit = string(digits[index%9])
	}
	logger.Debug("GetDigit: exit",
		zap.String("digit", digit),
	)
	piResponse := transfer.PiResponse{
		Index: index,
		Digit: digit,
	}
	err = piResponse.MarshalResponse(ctx, w)
	if err != nil {
		logger.Error("Error marshalling response",
			zap.Error(err),
		)
	}
}

func cachedDigit(ctx context.Context, index int64) (string, error) {
	logger := Logger.With(
		zap.Int64("index", index),
	)
	logger.Debug("cachedDigit: enter")
	client, err := redis.Dial("tcp", redisAddress)
	if err != nil {
		logger.Error("Error connecting to Redis",
			zap.Error(err),
		)
		return "", err
	}
	defer client.Close()

	key := strconv.FormatInt(int64(index/9), 10)
	digits, err := client.Cmd("GET", key).Str()
	if err != nil && err == redis.ErrRespNil {
		logger.Info("Digits are not cached",
			zap.String("key", key),
			zap.Error(err),
		)
		return "", nil
	}
	if err != nil {
		logger.Error("Error returned from Redis cache",
			zap.String("key", key),
			zap.Error(err),
		)
		return "", err
	}
	logger.Debug("Cache lookup returned",
		zap.String("digits", digits),
	)
	digit := string(digits[index%9])
	logger.Debug("cachedDigit: exit",
		zap.String("result", digit),
	)
	return digit, nil
}

func writeDigits(ctx context.Context, index int64, digits string) error {
	logger := Logger.With(
		zap.Int64("index", index),
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
	key := strconv.FormatInt(int64(index/9), 10)
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
