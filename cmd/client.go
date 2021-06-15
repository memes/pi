// Implements a JSON client that requests a set of digits of pi starting at a
// specified index.
package cmd

import (
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/memes/pi/transfer"
	"github.com/sethgrid/pester"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

const (
	DEFAULT_REMOTE_URL = "http://pi-service:8080/"
	DEFAULT_COUNT      = 1000
)

var (
	url         string
	count       int
	waitaminute bool
	client      *pester.Client
	clientCmd   = &cobra.Command{
		Use:   "client",
		Short: "Run a JSON client to request pi digits",
		Long:  "Launch a client that attempts to connect to servers and return a subset of the mantissa of pi.",
		Run: func(cmd *cobra.Command, args []string) {
			logger := Logger.With(
				zap.Int("count", count),
				zap.String("url", url),
				zap.Bool("waitaminute", waitaminute),
			)
			if waitaminute {
				logger.Info("Sleeping as requested")
				time.Sleep(60 * time.Second)
			}
			logger.Debug("Running client")
			// Randomize the retrieval of numbers
			indices := rand.Perm(count)
			digits := make([]string, count)
			var wg sync.WaitGroup
			for _, index := range indices {
				wg.Add(1)
				go func(index int) {
					defer wg.Done()
					log := logger.With(
						zap.Int("index", index),
					)
					log.Debug("In goroutine")
					digit, err := fetchDigit(int64(index))
					if err != nil {
						log.Error("Error getting digit",
							zap.Error(err),
						)
						digit = "#"
					}
					digits[index] = digit
				}(index)
			}
			wg.Wait()
			fmt.Printf("Result is: 3.%s\n", strings.Join(digits, ""))
		},
	}
)

func init() {
	clientCmd.PersistentFlags().StringVarP(&url, "url", "u", DEFAULT_REMOTE_URL, "Remote URL to connect to")
	clientCmd.PersistentFlags().IntVarP(&count, "count", "c", DEFAULT_COUNT, "Number of digits of pi to return ")
	clientCmd.PersistentFlags().BoolVarP(&waitaminute, "waitaminute", "w", false, "Wait 60s before initiating connections")
	_ = viper.BindPFlag("url", clientCmd.PersistentFlags().Lookup("url"))
	_ = viper.BindPFlag("count", clientCmd.PersistentFlags().Lookup("count"))
	_ = viper.BindPFlag("waitaminute", clientCmd.PersistentFlags().Lookup("waitaminute"))
	RootCmd.AddCommand(clientCmd)
}

func fetchDigit(index int64) (string, error) {
	ctx := context.Background()
	logger := Logger.With(
		zap.String("url", url),
		zap.Int64("index", index),
	)
	logger.Debug("Starting connection to service")
	api := fmt.Sprintf("%s/v1/digit/%d", url, index)
	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		logger.Error("Error createing a new request",
			zap.String("api", api),
			zap.Error(err),
		)
		return "", err
	}
	req.Header.Set("User-Agent", "pi-client")
	client = pester.New()
	client.KeepLog = false
	client.LogHook = func(e pester.ErrEntry) {
		Logger.Debug("HTTP request",
			zap.String("method", e.Method),
			zap.String("verb", e.Verb),
			zap.String("url", e.URL),
			zap.Int("attempt", e.Attempt),
			zap.Error(e.Err),
		)
	}
	response, err := client.Do(req)
	if err != nil {
		logger.Error("Error received from remote",
			zap.Error(err),
		)
		return "", err
	}
	piResponse := transfer.PiResponse{}
	err = piResponse.UnmarshalRequest(ctx, response)
	if err != nil {
		logger.Error("Error unmarshaling response body",
			zap.Error(err),
		)
		return "", err
	}
	logger.Debug("Response from remote",
		zap.String("result", piResponse.Digit),
	)

	return piResponse.Digit, nil
}
