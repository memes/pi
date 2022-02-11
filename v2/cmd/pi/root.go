package main

import (
	"context"
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
)

const (
	APP_NAME     = "pi"
	PACKAGE_NAME = "github.com/memes/pi/v2/cmd/pi"
)

var (
	// Version is updated from git tags during build
	version = "unspecified"
	rootCmd = &cobra.Command{
		Use:     APP_NAME,
		Version: version,
		Short:   "Get a fractional digit of pi at an arbitrary index",
		Long: `Provides a client/server application that can be used to demonstrate distributed calculation of fractional digits of pi.
The application supports OpenTelemetry metric and trace generation and will optionally send these to a specified OpenTelemetry collector.`,
	}
)

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().CountP("verbose", "v", "Enable more verbose logging")
	rootCmd.PersistentFlags().BoolP("pretty", "p", false, "Enable prettier logging to console")
	rootCmd.PersistentFlags().StringP("otlp-endpoint", "o", "", "An OpenTelemetry collection endpoint to receive metrics and traces")
	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	_ = viper.BindPFlag("pretty", rootCmd.PersistentFlags().Lookup("pretty"))
	_ = viper.BindPFlag("otlp-endpoint", rootCmd.PersistentFlags().Lookup("otlp-endpoint"))
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
	viper.SetConfigName("." + APP_NAME)
	viper.SetEnvPrefix(APP_NAME)
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
	switch t := err.(type) {
	case viper.ConfigFileNotFoundError:
		logger.V(0).Info("Configuration file not found", "err", t)

	default:
		logger.Error(t, "Error reading configuration file")
	}
}

// Create a new OpenTelemetry resource to describe the source of metrics and traces.
func newTelemetryResource(ctx context.Context, name string) (*resource.Resource, error) {
	logger := logger.V(1).WithValues("name", name)
	logger.Info("Creating new OpenTelemetry resource descriptor")
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	resource, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNamespaceKey.String(PACKAGE_NAME),
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
		return nil, err
	}
	logger.V(2).Info("Resource created", "resource", resource)
	return resource, err
}

// Returns a new metric pusher that will send OpenTelemetry metrics to the endpoint
// provided as a configuration flag.
func newMetricPusher(ctx context.Context) (*controller.Controller, error) {
	logger.V(1).Info("Creating new OpenTelemetry metric pusher")
	endpoint := viper.GetString("otlp-endpoint")
	if endpoint == "" {
		logger.V(0).Info("OpenTelemetry endpoint is not set; no metrics will be sent to collector")
		return nil, nil
	}
	client := otlpmetricgrpc.NewClient(otlpmetricgrpc.WithInsecure(), otlpmetricgrpc.WithEndpoint(endpoint))
	exporter, err := otlpmetric.New(ctx, client)
	if err != nil {
		return nil, err
	}
	pusher := controller.New(processor.NewFactory(simple.NewWithInexpensiveDistribution(), exporter), controller.WithExporter(exporter), controller.WithCollectPeriod(2*time.Second))
	global.SetMeterProvider(pusher)
	err = pusher.Start(ctx)
	return pusher, err
}

// Returns a new trace exporter that will send OpenTelemetry traces to the endpoint
// provided as a configuration flag.
func newTraceExporter(ctx context.Context) (*otlptrace.Exporter, error) {
	logger.V(1).Info("Creating new OpenTelemetry trace exporter")
	endpoint := viper.GetString("otlp-endpoint")
	if endpoint == "" {
		logger.V(0).Info("OpenTelemetry endpoint is not set; no traces will be sent to collector")
		return nil, nil
	}
	client := otlptracegrpc.NewClient(otlptracegrpc.WithInsecure(), otlptracegrpc.WithEndpoint(endpoint), otlptracegrpc.WithDialOption(grpc.WithBlock()))
	return otlptrace.New(ctx, client)
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
	resource, err := newTelemetryResource(ctx, name)
	if err != nil {
		logger.Error(err, "Failed to create OpenTelemetry resource")
	}

	// Without an exporter the traces cannot be processed and sent to a collector,
	// so create a provider that does not ever sample or attempt to deliver traces.
	var provider *sdktrace.TracerProvider
	if exporter != nil {
		provider = sdktrace.NewTracerProvider(sdktrace.WithSampler(sampler), sdktrace.WithSpanProcessor(sdktrace.NewBatchSpanProcessor(exporter)), sdktrace.WithResource(resource))
		otel.SetTracerProvider(provider)
	}
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	logger.Info("OpenTelemetry initialization complete, returning shutdown function")
	// Return a function that will cancel OpenTelemetry processes
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
