package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-logr/zerologr"
	"github.com/google/uuid"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	gcpdetectors "go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/propagation"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	AppName     = "pi"
	PackageName = "github.com/memes/pi/v2/cmd/pi"
)

var (
	// Version is updated from git tags during build.
	version = "unspecified"
	// Failed to load CA cert.
	errFailedToAppendCACert = errors.New("failed to append CA cert to CA pool")
)

func NewRootCmd() (*cobra.Command, error) {
	cobra.OnInitialize(initConfig)
	rootCmd := &cobra.Command{
		Use:     AppName,
		Version: version,
		Short:   "Get a fractional digit of pi at an arbitrary index",
		Long:    `Provides a gRPC client/server demo for distributed calculation of fractional digits of pi.`,
	}
	rootCmd.PersistentFlags().CountP("verbose", "v", "Enable verbose logging; can be repeated to increase verbosity")
	rootCmd.PersistentFlags().BoolP("pretty", "p", false, "Disables structured JSON logging to stdout, making it easier to read")
	rootCmd.PersistentFlags().String("otlp-target", "", "An optional OpenTelemetry collection target that will receive metrics and traces")
	rootCmd.PersistentFlags().String("otlp-cacert", "", "An optional path to a CA certificate file to use to validate OpenTelemetry target endpoint.")
	rootCmd.PersistentFlags().Bool("otlp-insecure", false, "Disable TLS verification of OpenTelemetry target endpoint")
	if err := viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose")); err != nil {
		return nil, fmt.Errorf("failed to bind verbose pflag: %w", err)
	}
	if err := viper.BindPFlag("pretty", rootCmd.PersistentFlags().Lookup("pretty")); err != nil {
		return nil, fmt.Errorf("failed to bind pretty pflag: %w", err)
	}
	if err := viper.BindPFlag("otlp-target", rootCmd.PersistentFlags().Lookup("otlp-target")); err != nil {
		return nil, fmt.Errorf("failed to bind otlp-target pflag: %w", err)
	}
	if err := viper.BindPFlag("otlp-cacert", rootCmd.PersistentFlags().Lookup("otlp-cacert")); err != nil {
		return nil, fmt.Errorf("failed to bind otlp-cacert pflag: %w", err)
	}
	if err := viper.BindPFlag("otlp-insecure", rootCmd.PersistentFlags().Lookup("otlp-insecure")); err != nil {
		return nil, fmt.Errorf("failed to bind otlp-insecure pflag: %w", err)
	}
	serverCmd, err := NewServerCmd()
	if err != nil {
		return nil, err
	}
	clientCmd, err := NewClientCmd()
	if err != nil {
		return nil, err
	}
	rootCmd.AddCommand(serverCmd, clientCmd)
	return rootCmd, nil
}

// Determine the outcome of command line flags, environment variables, and an
// optional configuration file to perform initialization of the application. An
// appropriate zerolog will be assigned as the default logr sink.
func initConfig() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	zl := zerolog.New(os.Stderr).With().Caller().Timestamp().Logger()
	viper.AddConfigPath(".")
	if home, err := homedir.Dir(); err == nil {
		viper.AddConfigPath(home)
	}
	viper.SetConfigName("." + AppName)
	viper.SetEnvPrefix(AppName)
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
	err := viper.ReadInConfig()
	verbosity := viper.GetInt("verbose")
	switch {
	case verbosity > 2:
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	case verbosity == 2:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case verbosity == 1:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	}
	if viper.GetBool("pretty") {
		zl = zl.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	}
	logger = zerologr.New(&zl)
	if err == nil {
		return
	}
	var cfgNotFound viper.ConfigFileNotFoundError
	if !errors.As(err, &cfgNotFound) {
		logger.Error(err, "Error reading configuration file")
	}
}

// Create a new OpenTelemetry resource to describe the source of metrics and traces.
func newTelemetryResource(ctx context.Context, name string) (*resource.Resource, error) {
	logger := logger.V(1).WithValues("name", name)
	logger.Info("Creating new OpenTelemetry resource descriptor")
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("failed to generate UUID for telemetry resource: %w", err)
	}
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNamespaceKey.String(PackageName),
			semconv.ServiceNameKey.String(name),
			semconv.ServiceVersionKey.String(version),
			semconv.ServiceInstanceIDKey.String(id.String()),
		),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithOS(),
		// Some process information is unknown when running in a scratch
		// container. See https://github.com/memes/pi/issues/3
		resource.WithProcessPID(),
		resource.WithProcessExecutableName(),
		resource.WithProcessExecutablePath(),
		resource.WithProcessCommandArgs(),
		resource.WithProcessRuntimeName(),
		resource.WithProcessRuntimeVersion(),
		resource.WithProcessRuntimeDescription(),
		// These detectors place last to override the base service attributes with specifiers from GCP
		resource.WithDetectors(
			&gcpdetectors.GCE{},
			&gcpdetectors.GKE{},
			gcpdetectors.NewCloudRun(),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create new telemetry resource: %w", err)
	}
	logger.V(2).Info("Resource created", "resource", res)
	return res, nil
}

// Returns a new metric pusher that will send OpenTelemetry metrics to the endpoint
// provided as a configuration flag.
func newMetricPusher(ctx context.Context) (*controller.Controller, error) {
	logger.V(1).Info("Creating new OpenTelemetry metric pusher")
	target := viper.GetString("otlp-target")
	if target == "" {
		logger.V(0).Info("OpenTelemetry endpoint is not set; no metrics will be sent to collector")
		return nil, nil
	}
	options := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(target),
	}
	if viper.GetBool("otlp-insecure") {
		options = append(options, otlpmetricgrpc.WithInsecure())
	} else {
		cacert := viper.GetString("otlp-cacert")
		if cacert != "" {
			tlsCreds, err := credentials.NewClientTLSFromFile(cacert, "")
			if err != nil {
				return nil, fmt.Errorf("failed to create new transport credentials for metric pusher: %w", err)
			}
			options = append(options, otlpmetricgrpc.WithTLSCredentials(tlsCreds))
		}
	}
	client := otlpmetricgrpc.NewClient(options...)
	exporter, err := otlpmetric.New(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create new metric exporter: %w", err)
	}
	pusher := controller.New(processor.NewFactory(simple.NewWithInexpensiveDistribution(), exporter), controller.WithExporter(exporter), controller.WithCollectPeriod(2*time.Second))
	global.SetMeterProvider(pusher)
	err = pusher.Start(ctx)
	return pusher, fmt.Errorf("failed to start metric pusher: %w", err)
}

// Returns a new trace exporter that will send OpenTelemetry traces to the endpoint
// provided as a configuration flag.
func newTraceExporter(ctx context.Context) (*otlptrace.Exporter, error) {
	logger.V(1).Info("Creating new OpenTelemetry trace exporter")
	target := viper.GetString("otlp-target")
	if target == "" {
		logger.V(0).Info("OpenTelemetry endpoint is not set; no traces will be sent to collector")
		return nil, nil
	}
	options := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(target),
		otlptracegrpc.WithDialOption(grpc.WithBlock()),
	}
	if viper.GetBool("otlp-insecure") {
		options = append(options, otlptracegrpc.WithInsecure())
	} else {
		cacert := viper.GetString("otlp-cacert")
		if cacert != "" {
			tlsCreds, err := credentials.NewClientTLSFromFile(cacert, "")
			if err != nil {
				return nil, fmt.Errorf("failed to create new transport credentials for trace exporter: %w", err)
			}
			options = append(options, otlptracegrpc.WithTLSCredentials(tlsCreds))
		}
	}
	client := otlptracegrpc.NewClient(options...)
	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create new trace exporter: %w", err)
	}
	return exporter, nil
}

// Initializes OpenTelemetry metric and trace processing and deliver to a collector
// endpoint, returning a function that can be called to shutdown the background
// pipeline processes.
func initTelemetry(ctx context.Context, name string, sampler sdktrace.Sampler) func(context.Context) {
	logger := logger.V(1).WithValues("name", name, "sampler", sampler.Description())
	logger.Info("Initializing OpenTelemetry")
	metrics, err := newMetricPusher(ctx)
	if err != nil {
		logger.Error(err, "Failed to create new OpenTelemetry metric controller")
	}
	exporter, err := newTraceExporter(ctx)
	if err != nil {
		logger.Error(err, "Failed to create OpenTelemetry trace exporter")
	}
	res, err := newTelemetryResource(ctx, name)
	if err != nil {
		logger.Error(err, "Failed to create OpenTelemetry resource")
	}

	// Without an exporter the traces cannot be processed and sent to a collector,
	// so create a provider that does not ever sample or attempt to deliver traces.
	var provider *sdktrace.TracerProvider
	if exporter != nil {
		provider = sdktrace.NewTracerProvider(sdktrace.WithSampler(sampler), sdktrace.WithSpanProcessor(sdktrace.NewBatchSpanProcessor(exporter)), sdktrace.WithResource(res))
		otel.SetTracerProvider(provider)
	}
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	logger.Info("OpenTelemetry initialization complete, returning shutdown function")
	// Return a function that will cancel OpenTelemetry processes when called.
	return func(ctx context.Context) {
		if provider != nil {
			if err := provider.Shutdown(ctx); err != nil {
				logger.Error(err, "Error raised while stopping tracer provider")
			}
		}
		if exporter != nil {
			if err := exporter.Shutdown(ctx); err != nil {
				logger.Error(err, "Error raised while stopping trace exporter")
			}
		}
		if metrics != nil {
			if err := metrics.Stop(ctx); err != nil {
				logger.Error(err, "Error raised while stopping metric contoller")
			}
		}
	}
}
