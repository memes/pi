package main

import (
	"crypto/tls"
	"crypto/x509"
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
	SERVER_SERVICE_NAME         = "server"
	DEFAULT_GRPC_LISTEN_ADDRESS = ":443"
)

// Implements the server sub-command
var serverCmd = &cobra.Command{
	Use:   SERVER_SERVICE_NAME,
	Short: "Run gRPC service to return fractional digits of pi",
	Long: `Launches a gRPC Pi Service server that can calculate the decimal digits of pi.

A single decimal digit of pi will be returned per request. An optional Redis DB can be used to cache the calculated digits. Metrics and traces will be sent to an OpenTelemetry collection endpoint, if specified.`,
	RunE: serverMain,
}

func init() {
	serverCmd.PersistentFlags().StringP("address", "a", DEFAULT_GRPC_LISTEN_ADDRESS, "Address to listen for gRPC PiService requests")
	serverCmd.PersistentFlags().String("rest-address", "", "An optional listen address to launch a REST/gRPC gateway process")
	serverCmd.PersistentFlags().String("redis-target", "", "An optional Redis endpoint to use as a PiService cache")
	serverCmd.PersistentFlags().StringToStringP("label", "l", nil, "An optional label key=value to add to PiService response metadata; can be repeated")
	serverCmd.PersistentFlags().String("cacert", "", "An optional CA certificate to use for PiService client TLS verification")
	serverCmd.PersistentFlags().String("cert", "", "An optional client TLS certificate to use with PiService")
	serverCmd.PersistentFlags().String("key", "", "An optional client TLS private key to use with PiService")
	serverCmd.PersistentFlags().Bool("tls-client-auth", false, "Require PiService clients to provide a valid TLS client certificate")
	_ = viper.BindPFlag("address", serverCmd.PersistentFlags().Lookup("address"))
	_ = viper.BindPFlag("rest-address", serverCmd.PersistentFlags().Lookup("rest-address"))
	_ = viper.BindPFlag("redis-target", serverCmd.PersistentFlags().Lookup("redis-target"))
	_ = viper.BindPFlag("label", serverCmd.PersistentFlags().Lookup("label"))
	_ = viper.BindPFlag("cacert", serverCmd.PersistentFlags().Lookup("cacert"))
	_ = viper.BindPFlag("cert", serverCmd.PersistentFlags().Lookup("cert"))
	_ = viper.BindPFlag("key", serverCmd.PersistentFlags().Lookup("key"))
	_ = viper.BindPFlag("tls-client-auth", serverCmd.PersistentFlags().Lookup("tls-client-auth"))
	rootCmd.AddCommand(serverCmd)
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
	telemetryShutdown := initTelemetry(ctx, SERVER_SERVICE_NAME, sdktrace.AlwaysSample())

	logger.V(0).Info("Preparing services")
	options := []server.PiServerOption{
		server.WithLogger(logger),
		server.WithMetadata(viper.GetStringMapString("label")),
		server.WithTracer(otel.Tracer(SERVER_SERVICE_NAME)),
		server.WithMeter(global.Meter(SERVER_SERVICE_NAME)),
		server.WithPrefix(SERVER_SERVICE_NAME),
	}
	if redisTarget != "" {
		options = append(options, server.WithCache(cache.NewRedisCache(ctx, redisTarget)))
	}
	tlsCreds, err := newServerTLSCredentials()
	if err != nil {
		return err
	}
	options = append(options, server.WithTransportCredentials(tlsCreds))
	server := server.NewPiServer(options...)
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
		grpcServer = server.NewGrpcServer()
		listener, err := net.Listen("tcp", address)
		if err != nil {
			return err
		}
		return grpcServer.Serve(listener)
	})
	if restAddress != "" {
		g.Go(func() error {
			logger.V(0).Info("Starting REST/gRPC gateway")
			restHandler, err := server.NewRestGatewayHandler(ctx, address)
			if err != nil {
				return err
			}
			restServer = &http.Server{
				Addr:    restAddress,
				Handler: restHandler,
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
	logger.V(0).Info("Shutting down on signal")
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
			return nil, err
		}
		tlsConf.Certificates = []tls.Certificate{cert}
	}

	if cacertFile != "" {
		logger.V(1).Info("Loading CA from file")
		ca, err := ioutil.ReadFile(cacertFile)
		if err != nil {
			return nil, err
		}
		certPool := x509.NewCertPool()
		if ok := certPool.AppendCertsFromPEM(ca); !ok {
			return nil, fmt.Errorf("failed to append CA cert %s to CA pool", cacertFile)
		}
		tlsConf.ClientCAs = certPool
	}

	if tlsClientAuth {
		tlsConf.ClientAuth = tls.RequireAndVerifyClientCert
	} else if cacertFile != "" {
		tlsConf.ClientAuth = tls.VerifyClientCertIfGiven
	} else {
		tlsConf.ClientAuth = tls.NoClientCert
	}
	return credentials.NewTLS(&tlsConf), nil
}
