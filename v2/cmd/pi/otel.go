package main

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"go.opentelemetry.io/contrib/detectors/gcp"
	runtimeinstrumentation "go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding/gzip"
)

const (
	OTELNamespace             = "github.com/memes/pi/v2"
	OTELMetricReportingPeriod = 30 * time.Second
)

// Defines a function signature that can be added to ShutdownFunctions.
type ShutdownFunction func(context.Context) error

// Defines a common shutdown helper that collects a list of ShutdownFunction
// functions that can be executed in last-in, first-out reverse order.
type ShutdownFunctions struct {
	functions []ShutdownFunction
}

// Add a slice of ShutdownFunction implementations to the LIFO collection.
func (s *ShutdownFunctions) AppendFunctions(fns []ShutdownFunction) {
	s.functions = append(s.functions, fns...)
}

// Helper to add a single ShutdownFunction to the LIFO collection.
func (s *ShutdownFunctions) AppendFunction(fn ShutdownFunction) {
	s.AppendFunctions([]ShutdownFunction{fn})
}

// Helper to append the ShutdownFunction entries from the provided instance onto
// the current object's list of functions.
func (s *ShutdownFunctions) Merge(shutdownFunctions ShutdownFunctions) {
	s.AppendFunctions(shutdownFunctions.functions)
}

// Helper to execute each registered ShutdownFunction in reverse-order of their
// addition.
func (s *ShutdownFunctions) Execute(ctx context.Context, logger logr.Logger) {
	for i := len(s.functions) - 1; i >= 0; i-- {
		if err := s.functions[i](ctx); err != nil {
			logger.Error(err, "Failure executing shutdown function; continuing")
		}
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
		resource.WithDetectors(gcp.NewDetector()),
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
		resource.WithAttributes(
			semconv.ServiceNamespace(OTELNamespace),
			semconv.ServiceName(name),
			semconv.ServiceVersion(version),
			semconv.ServiceInstanceID(id.String()),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create new telemetry resource: %w", err)
	}
	logger.V(1).Info("OpenTelemetry resource created", "resource", res)
	return res, nil
}

// Initializes a pusher that will send OpenTelemetry metrics to the target
// provided, returning a ShutdownFunctions object.
func initMetrics(ctx context.Context, target string, creds credentials.TransportCredentials, res *resource.Resource) (ShutdownFunctions, error) {
	logger := logger.V(1).WithValues("target", target, "creds", creds, "res", res)
	logger.V(1).Info("Creating OpenTelemetry metric handlers")
	if target == "" {
		logger.V(0).Info("OpenTelemetry endpoint is not set; no metrics will be sent to collector")
		return ShutdownFunctions{}, nil
	}
	options := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(target),
		otlpmetricgrpc.WithCompressor(gzip.Name),
	}
	if creds != nil {
		options = append(options, otlpmetricgrpc.WithTLSCredentials(creds))
	}
	exporter, err := otlpmetricgrpc.New(ctx, options...)
	if err != nil {
		return ShutdownFunctions{}, fmt.Errorf("failed to create new metric exporter: %w", err)
	}
	provider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(exporter, metric.WithInterval(OTELMetricReportingPeriod))),
	)

	shutdownFunctions := ShutdownFunctions{
		functions: []ShutdownFunction{
			func(ctx context.Context) error {
				if err := provider.Shutdown(ctx); err != nil {
					return fmt.Errorf("error during OpenTelemetry metric provider shutdown: %w", err)
				}
				return nil
			},
		},
	}
	if err = runtimeinstrumentation.Start(runtimeinstrumentation.WithMeterProvider(provider)); err != nil {
		return shutdownFunctions, fmt.Errorf("failed to start runtime metrics: %w", err)
	}

	otel.SetMeterProvider(provider)
	logger.V(1).Info("OpenTelemetry metric handlers created and started")
	return shutdownFunctions, nil
}

// Initializes a pipeline handler that will send OpenTelemetry spans to the target
// provided, returning a ShutdownFunctions object.
func initTrace(ctx context.Context, target string, creds credentials.TransportCredentials, res *resource.Resource, sampler trace.Sampler) (ShutdownFunctions, error) {
	logger := logger.V(1).WithValues("target", target, "creds", creds, "res", res, "sampler", sampler.Description())
	logger.V(1).Info("Creating new OpenTelemetry trace exporter")
	if target == "" {
		logger.V(0).Info("OpenTelemetry endpoint is not set; no traces will be sent to collector")
		return ShutdownFunctions{}, nil
	}
	options := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(target),
		otlptracegrpc.WithCompressor(gzip.Name),
	}
	if creds != nil {
		options = append(options, otlptracegrpc.WithTLSCredentials(creds))
	}
	exporter, err := otlptrace.New(ctx, otlptracegrpc.NewClient(options...))
	if err != nil {
		return ShutdownFunctions{}, fmt.Errorf("failed to create new trace exporter: %w", err)
	}
	shutdownFunctions := ShutdownFunctions{
		functions: []ShutdownFunction{
			func(ctx context.Context) error {
				if err := exporter.Shutdown(ctx); err != nil {
					return fmt.Errorf("error during OpenTelemetry trace exporter shutdown: %w", err)
				}
				return nil
			},
		},
	}

	// NOTE: provider.Shutdown will shutdown every registered span processor
	// so don't add an explicit shutdown function.
	spanProcessor := trace.NewBatchSpanProcessor(exporter)

	provider := trace.NewTracerProvider(
		trace.WithSampler(sampler),
		trace.WithSpanProcessor(spanProcessor),
		trace.WithResource(res),
	)
	shutdownFunctions.AppendFunction(func(ctx context.Context) error {
		if err := provider.Shutdown(ctx); err != nil {
			return fmt.Errorf("error during OpenTelemetry trace provider shutdown: %w", err)
		}
		return nil
	})
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	otel.SetTracerProvider(provider)
	logger.V(1).Info("OpenTelemetry trace handlers created and started")
	return shutdownFunctions, nil
}

// Initializes OpenTelemetry metric and trace processing and deliver to a collector
// target, returning a ShutdownFunctions object that can be used to cleanup the
// background pipeline processes.
func initTelemetry(ctx context.Context, name string, sampler trace.Sampler) (ShutdownFunctions, error) {
	otel.SetLogger(logger)
	target := viper.GetString(OpenTelemetryTargetFlagName)
	logger := logger.V(1).WithValues(
		"name", name,
		"target", target,
		"sampler", sampler.Description(),
	)
	logger.Info("Initializing OpenTelemetry")

	res, err := newTelemetryResource(ctx, name)
	if err != nil {
		return ShutdownFunctions{}, err
	}

	creds, err := buildOTELClientTransportCredentials()
	if err != nil {
		return ShutdownFunctions{}, err
	}

	shutdownFunctions, err := initMetrics(ctx, target, creds, res)
	if err != nil {
		return shutdownFunctions, err
	}
	shutdownTraces, err := initTrace(ctx, target, creds, res, sampler)
	shutdownFunctions.Merge(shutdownTraces)
	logger.Info("OpenTelemetry initialization complete, returning shutdown functions")
	return shutdownFunctions, err
}

// Creates the gRPC transport credentials that are appropriate for a remote OTEL
// gRPC collector, based on command line flags.
func buildOTELClientTransportCredentials() (credentials.TransportCredentials, error) {
	if viper.GetBool(OpenTelemetryInsecureFlagName) {
		return insecure.NewCredentials(), nil
	}
	certPool, err := newCACertPool(viper.GetStringSlice(CACertFlagName))
	if err != nil {
		return nil, err
	}
	tlsConfig, err := newTLSConfig(viper.GetString(OpenTelemetryTLSCertFlagName), viper.GetString(OpenTelemetryTLSKeyFlagName), nil, certPool)
	if err != nil {
		return nil, err
	}
	return credentials.NewTLS(tlsConfig), nil
}
