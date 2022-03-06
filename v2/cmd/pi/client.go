package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/memes/pi/v2/pkg/client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric/global"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc/credentials"
)

const (
	CLIENT_SERVICE_NAME = "client"
	DEFAULT_DIGIT_COUNT = 100
	DEFAULT_MAX_TIMEOUT = 10 * time.Second
)

// Implements the client sub-command which attempts to connect to one or
// more pi server instances and build up the digits of pi through multiple
// requests.
var clientCmd = &cobra.Command{
	Use:   CLIENT_SERVICE_NAME + " target [target]",
	Short: "Run a gRPC Pi Service client to request fractional digits of pi",
	Long: `Launches a gRPC client that will connect to Pi Service target(s) and request the fractional digits of pi.

At least one target endpoint must be provided. Metrics and traces will be sent to an OpenTelemetry collection endpoint, if specified.`,
	Args: cobra.MinimumNArgs(1),
	RunE: clientMain,
}

func init() {
	clientCmd.PersistentFlags().UintP("count", "c", DEFAULT_DIGIT_COUNT, "The number of decimal digits of pi to request")
	clientCmd.PersistentFlags().DurationP("max-timeout", "m", DEFAULT_MAX_TIMEOUT, "The maximum timeout for a Pi Service request")
	clientCmd.PersistentFlags().String("cacert", "", "An optional CA certificate to use for Pi Service TLS verification")
	clientCmd.PersistentFlags().String("cert", "", "An optional client TLS certificate to use with Pi Service")
	clientCmd.PersistentFlags().String("key", "", "An optional client TLS private key to use with Pi Service")
	clientCmd.PersistentFlags().Bool("insecure", false, "Disable TLS verification of Pi Service")
	clientCmd.PersistentFlags().String("authority", "", "Set the authoritative name of the Pi Service target for TLS verification, overriding hostname")
	_ = viper.BindPFlag("count", clientCmd.PersistentFlags().Lookup("count"))
	_ = viper.BindPFlag("max-timeout", clientCmd.PersistentFlags().Lookup("max-timeout"))
	_ = viper.BindPFlag("cacert", clientCmd.PersistentFlags().Lookup("cacert"))
	_ = viper.BindPFlag("cert", clientCmd.PersistentFlags().Lookup("cert"))
	_ = viper.BindPFlag("key", clientCmd.PersistentFlags().Lookup("key"))
	_ = viper.BindPFlag("insecure", clientCmd.PersistentFlags().Lookup("insecure"))
	_ = viper.BindPFlag("authority", clientCmd.PersistentFlags().Lookup("authority"))
	rootCmd.AddCommand(clientCmd)
}

// Client sub-command entrypoint. This function will launch gRPC requests for
// each of the fractional digits requested.
func clientMain(cmd *cobra.Command, endpoints []string) error {
	count := viper.GetInt("count")
	logger := logger.V(1).WithValues("count", count, "endpoints", endpoints)
	logger.V(0).Info("Preparing telemetry")
	ctx := context.Background()
	shutdown := initTelemetry(ctx, CLIENT_SERVICE_NAME, sdktrace.AlwaysSample())
	defer shutdown(ctx)
	logger.V(0).Info("Preparing client TLS config")
	tlsCreds, err := newClientTLSCredentials()
	if err != nil {
		return err
	}
	logger.V(0).Info("Building client")
	options := []client.PiClientOption{
		client.WithLogger(logger),
		client.WithMaxTimeout(viper.GetDuration("max-timeout")),
		client.WithTracer(otel.Tracer(CLIENT_SERVICE_NAME)),
		client.WithMeter(global.Meter(CLIENT_SERVICE_NAME)),
		client.WithPrefix(CLIENT_SERVICE_NAME),
		client.WithTransportCredentials(tlsCreds),
		client.WithAuthority(viper.GetString("authority")),
	}
	client := client.NewPiClient(options...)
	// Randomize the retrieval of numbers
	indices := rand.Perm(count)
	digits := make([]byte, count)
	var wg sync.WaitGroup
	for i, index := range indices {
		endpoint := endpoints[i%len(endpoints)]
		wg.Add(1)
		go func(endpoint string, index uint64) {
			defer wg.Done()
			digit, err := client.FetchDigit(endpoint, index)
			if err != nil {
				logger.Error(err, "Error fetching digit", "index", index)
				digits[index] = '-'
			} else {
				digits[index] = '0' + byte(digit)
			}
		}(endpoint, uint64(index))
	}
	wg.Wait()
	fmt.Print("Result is: 3.")
	if _, err := os.Stdout.Write(digits[:]); err != nil {
		return err
	}
	fmt.Println()
	return nil
}

// Creates the gRPC transport credentials to use with PiService client from the
// various configuration options provided.
func newClientTLSCredentials() (credentials.TransportCredentials, error) {
	var tlsConf tls.Config
	certFile := viper.GetString("cert")
	keyFile := viper.GetString("key")
	cacertFile := viper.GetString("cacert")
	insecure := viper.GetBool("insecure")
	logger := logger.V(1).WithValues("certFile", certFile, "keyFile", keyFile, "cacertFile", cacertFile, "insecure", insecure)
	logger.V(0).Info("Preparing client TLS credentials")
	if certFile != "" {
		logger.V(1).Info("Loading x509 certificate and key")
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, err
		}
		tlsConf.Certificates = []tls.Certificate{cert}
	}

	if insecure {
		logger.V(1).Info("Skipping TLS verification")
		tlsConf.InsecureSkipVerify = true
	} else if cacertFile != "" {
		logger.V(1).Info("Loading CA from file")
		ca, err := ioutil.ReadFile(cacertFile)
		if err != nil {
			return nil, err
		}
		certPool := x509.NewCertPool()
		if ok := certPool.AppendCertsFromPEM(ca); !ok {
			return nil, fmt.Errorf("failed to append CA cert %s to CA pool", cacertFile)
		}
		tlsConf.RootCAs = certPool
	}
	return credentials.NewTLS(&tlsConf), nil
}
