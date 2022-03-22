package main

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	gcpdetectors "go.opentelemetry.io/contrib/detectors/gcp"
	hostMetrics "go.opentelemetry.io/contrib/instrumentation/host"
	runtimeMetrics "go.opentelemetry.io/contrib/instrumentation/runtime"
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
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/encoding/gzip"
)

const (
	metricReportingPeriod = 30 * time.Second
)

// Create a new OpenTelemetry resource to describe the source of metrics and traces.
func newTelemetryResource(ctx context.Context, name string) (*resource.Resource, error) {
	logger := logger.V(1).WithValues("name", name)
	logger.Info("Creating new OpenTelemetry resource descriptor")
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("failed to generate UUID for telemetry resource: %w", err)
	}
	res, err := resource.New(ctx,
		resource.WithSchemaURL(semconv.SchemaURL),
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
	logger.V(1).Info("OpenTelemetry resource created", "resource", res)
	return res, nil
}

// Initialises a pusher that will send OpenTelemetry metrics to the target
// provided, returning a shutdown function.
func initMetrics(ctx context.Context, target string, creds credentials.TransportCredentials, res *resource.Resource) (func(context.Context) error, error) {
	logger := logger.V(1).WithValues("target", target, "creds", creds, "res", res)
	logger.V(1).Info("Creating OpenTelemetry metric handlers")
	if target == "" {
		logger.V(0).Info("OpenTelemetry endpoint is not set; no metrics will be sent to collector")
		return func(_ context.Context) error {
			return nil
		}, nil
	}
	options := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(target),
		otlpmetricgrpc.WithCompressor(gzip.Name),
	}
	if creds != nil {
		options = append(options, otlpmetricgrpc.WithTLSCredentials(creds))
	}
	client := otlpmetricgrpc.NewClient(options...)
	exporter, err := otlpmetric.New(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create new metric exporter: %w", err)
	}
	pusher := controller.New(
		processor.NewFactory(simple.NewWithHistogramDistribution(), exporter),
		controller.WithExporter(exporter),
		controller.WithResource(res),
		controller.WithCollectPeriod(metricReportingPeriod),
	)
	if err = pusher.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start metric pusher: %w", err)
	}
	if err = runtimeMetrics.Start(runtimeMetrics.WithMeterProvider(pusher)); err != nil {
		return nil, fmt.Errorf("failed to start runtime metrics: %w", err)
	}
	if err = hostMetrics.Start(hostMetrics.WithMeterProvider(pusher)); err != nil {
		return nil, fmt.Errorf("failed to start host metrics: %w", err)
	}

	global.SetMeterProvider(pusher)
	logger.V(1).Info("OpenTelemetry metric handlers created and started")
	return func(ctx context.Context) error {
		if err := pusher.Stop(ctx); err != nil {
			logger.Error(err, "Error raised while stopping metric pusher; continuing")
		}
		if err := exporter.Shutdown(ctx); err != nil {
			return fmt.Errorf("failure to shutdown metric exporter: %w", err)
		}
		return nil
	}, nil
}

// Initialises a pipeline handler that will send OpenTelemetry spans to the target
// provided, returning a shutdown function.
func initTrace(ctx context.Context, target string, creds credentials.TransportCredentials, res *resource.Resource, sampler trace.Sampler) (func(context.Context) error, error) {
	logger := logger.V(1).WithValues("target", target, "creds", creds, "res", res, "sampler", sampler.Description())
	logger.V(1).Info("Creating new OpenTelemetry trace exporter")
	if target == "" {
		logger.V(0).Info("OpenTelemetry endpoint is not set; no traces will be sent to collector")
		return func(_ context.Context) error {
			return nil
		}, nil
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
		return nil, fmt.Errorf("failed to create new trace exporter: %w", err)
	}
	spanProcessor := trace.NewBatchSpanProcessor(exporter)
	provider := trace.NewTracerProvider(
		trace.WithSampler(sampler),
		trace.WithSpanProcessor(spanProcessor),
		trace.WithResource(res),
	)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	otel.SetTracerProvider(provider)
	logger.V(1).Info("OpenTelemetry trace handlers created and started")
	return func(ctx context.Context) error {
		if err := spanProcessor.Shutdown(ctx); err != nil {
			logger.Error(err, "Error raised while shutting down trace processor; continuing")
		}
		if err := exporter.Shutdown(ctx); err != nil {
			return fmt.Errorf("failure to shutdown trace exporter: %w", err)
		}
		return nil
	}, nil
}

// Initializes OpenTelemetry metric and trace processing and deliver to a collector
// target, returning a function that can be called to shutdown the background
// pipeline processes.
func initTelemetry(ctx context.Context, name, target string, creds credentials.TransportCredentials, sampler trace.Sampler) (func(context.Context), error) {
	logger := logger.V(1).WithValues("name", name, "target", target, "creds", creds, "sampler", sampler.Description())
	logger.Info("Initializing OpenTelemetry")
	res, err := newTelemetryResource(ctx, name)
	if err != nil {
		return nil, err
	}
	shutdownMetrics, err := initMetrics(ctx, target, creds, res)
	if err != nil {
		return nil, err
	}
	shutdownTraces, err := initTrace(ctx, target, creds, res, sampler)
	if err != nil {
		return nil, err
	}

	logger.Info("OpenTelemetry initialization complete, returning shutdown function")
	// Return a function that will cancel OpenTelemetry processes when called.
	return func(ctx context.Context) {
		if shutdownTraces != nil {
			if err := shutdownTraces(ctx); err != nil {
				logger.Error(err, "Error raised while shutting down tracing; continuing")
			}
		}
		if shutdownMetrics != nil {
			if err := shutdownMetrics(ctx); err != nil {
				logger.Error(err, "Error raised while shutting down metrics; continuing")
			}
		}
	}, nil
}
