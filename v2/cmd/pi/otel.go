package main

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
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
	logger.V(1).Info("OpenTelemetry resource created", "resource", res)
	return res, nil
}

// Returns a new metric pusher that will send OpenTelemetry metrics to the target
// provided as a configuration flag.
func newMetricPusher(ctx context.Context, target string, creds credentials.TransportCredentials) (*controller.Controller, error) {
	logger := logger.V(1).WithValues("target", target, "creds", creds)
	logger.V(1).Info("Creating new OpenTelemetry metric pusher")
	if target == "" {
		logger.V(0).Info("OpenTelemetry endpoint is not set; no metrics will be sent to collector")
		return nil, nil
	}
	options := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(target),
	}
	if creds != nil {
		options = append(options, otlpmetricgrpc.WithTLSCredentials(creds))
	}
	client := otlpmetricgrpc.NewClient(options...)
	exporter, err := otlpmetric.New(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create new metric exporter: %w", err)
	}
	pusher := controller.New(processor.NewFactory(simple.NewWithInexpensiveDistribution(), exporter), controller.WithExporter(exporter), controller.WithCollectPeriod(2*time.Second))
	global.SetMeterProvider(pusher)
	err = pusher.Start(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start metric pusher: %w", err)
	}
	logger.V(1).Info("OpenTelemetry metric pusher created and started", "pusher", pusher)
	return pusher, nil
}

// Returns a new trace exporter that will send OpenTelemetry traces to the target
// provided as a configuration flag.
func newTraceExporter(ctx context.Context, target string, creds credentials.TransportCredentials) (*otlptrace.Exporter, error) {
	logger := logger.V(1).WithValues("target", target, "creds", creds)
	logger.V(1).Info("Creating new OpenTelemetry trace exporter")
	if target == "" {
		logger.V(0).Info("OpenTelemetry endpoint is not set; no traces will be sent to collector")
		return nil, nil
	}
	options := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(target),
		otlptracegrpc.WithDialOption(grpc.WithBlock()),
	}
	if creds != nil {
		options = append(options, otlptracegrpc.WithTLSCredentials(creds))
	}
	client := otlptracegrpc.NewClient(options...)
	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create new trace exporter: %w", err)
	}
	logger.V(1).Info("OpenTelemetry trace exporter created and started", "exporter", exporter)
	return exporter, nil
}

// Initializes OpenTelemetry metric and trace processing and deliver to a collector
// target, returning a function that can be called to shutdown the background
// pipeline processes.
func initTelemetry(ctx context.Context, name, target string, creds credentials.TransportCredentials, sampler sdktrace.Sampler) (func(context.Context), error) {
	logger := logger.V(1).WithValues("name", name, "target", target, "creds", creds, "sampler", sampler.Description())
	logger.Info("Initializing OpenTelemetry")
	metrics, err := newMetricPusher(ctx, target, creds)
	if err != nil {
		return nil, err
	}
	exporter, err := newTraceExporter(ctx, target, creds)
	if err != nil {
		return nil, err
	}
	res, err := newTelemetryResource(ctx, name)
	if err != nil {
		return nil, err
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
	}, nil
}
