package main

// spell-checker: ignore mitchellh gcpdetectors otel otlp otlpmetric otlptrace sdktrace semconv otlpmetricgrpc otlptracegrpc
import (
	"context"
	"errors"
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
	errNoTelemetryEndpoint = errors.New("OpenTelemetry endpoint is not set")
	version                = "unknown"
	rootCmd                = &cobra.Command{
		Use:     APP_NAME,
		Version: version,
		Short:   "Utility to get the digit of pi at an arbitrary index",
		Long:    "Pi is a client/server application that will fetch one of the decimal digits of pi from an arbitrary index.",
	}
)

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().CountP("verbose", "v", "Enable more verbose logging")
	rootCmd.PersistentFlags().BoolP("pretty", "p", false, "Enable pretty console logging")
	rootCmd.PersistentFlags().StringP("otlp-endpoint", "o", "", "The OpenTelemetry endpoint for export of metrics and traces")
	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	_ = viper.BindPFlag("pretty", rootCmd.PersistentFlags().Lookup("pretty"))
	_ = viper.BindPFlag("otlp-endpoint", rootCmd.PersistentFlags().Lookup("otlp-endpoint"))
}

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
	case verbosity >= 2:
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
		logger.V(1).Info("Configuration file not found", "err", t)

	default:
		logger.Error(t, "Error reading configuration file")
	}
}

// Create a new OpenTelemetry resource with the named service
func newTelemetryResource(ctx context.Context, name string) (*resource.Resource, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	return resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNamespaceKey.String(PACKAGE_NAME),
			semconv.ServiceNameKey.String(name),
			semconv.ServiceVersionKey.String(version),
			semconv.ServiceInstanceIDKey.String(id.String()),
		),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithOS(),
		resource.WithProcess(),
		resource.WithDetectors(&gcpdetectors.GCE{}, &gcpdetectors.GKE{}, gcpdetectors.NewCloudRun()),
	)
}

func newMetricController(ctx context.Context) (*controller.Controller, error) {
	endpoint := viper.GetString("otlp-endpoint")
	if endpoint == "" {
		return nil, errNoTelemetryEndpoint
	}
	client := otlpmetricgrpc.NewClient(otlpmetricgrpc.WithInsecure(), otlpmetricgrpc.WithEndpoint(endpoint))
	exporter, err := otlpmetric.New(ctx, client)
	if err != nil {
		return nil, err
	}
	pusher := controller.New(processor.NewFactory(simple.NewWithExactDistribution(), exporter), controller.WithExporter(exporter), controller.WithCollectPeriod(2*time.Second))
	global.SetMeterProvider(pusher)
	err = pusher.Start(ctx)
	return pusher, err
}

func newTraceExporter(ctx context.Context) (*otlptrace.Exporter, error) {
	endpoint := viper.GetString("otlp-endpoint")
	if endpoint == "" {
		return nil, errNoTelemetryEndpoint
	}
	client := otlptracegrpc.NewClient(otlptracegrpc.WithInsecure(), otlptracegrpc.WithEndpoint(endpoint), otlptracegrpc.WithDialOption(grpc.WithBlock()))
	return otlptrace.New(ctx, client)
}

func newTracerProvider(ctx context.Context, name string, sampler sdktrace.Sampler, exporter *otlptrace.Exporter) (*sdktrace.TracerProvider, error) {
	var provider *sdktrace.TracerProvider
	resource, err := newTelemetryResource(ctx, name)
	if err != nil {
		return nil, err
	}
	if exporter != nil {
		provider = sdktrace.NewTracerProvider(sdktrace.WithSampler(sampler), sdktrace.WithSpanProcessor(sdktrace.NewBatchSpanProcessor(exporter)), sdktrace.WithResource(resource))
	} else {
		provider = sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.NeverSample()), sdktrace.WithResource(resource))
	}
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	otel.SetTracerProvider(provider)
	return provider, nil
}

func initTelemetry(ctx context.Context, name string, sampler sdktrace.Sampler) func(context.Context) {
	metrics, err := newMetricController(ctx)
	if err != nil {
		logger.Error(err, "Failed to create new OpenTelemetry metric controller")
	}
	exporter, err := newTraceExporter(ctx)
	if err != nil {
		logger.Error(err, "Failed to create OpenTelemetry trace exporter")
	}
	provider, err := newTracerProvider(ctx, name, sampler, exporter)
	if err != nil {
		logger.Error(err, "Failed to create OpenTelemetry tracer provider")
	}
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
