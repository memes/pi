// Package client implements a gRPC client implementation that satisfies the
// PiServiceClient interface requirements with optional OpenTelemetry metrics and
// traces.
package client

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	api "github.com/memes/pi/v2/api/v2"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
	"go.opentelemetry.io/otel/metric/unit"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"

	"google.golang.org/grpc/status"
	_ "google.golang.org/grpc/xds" // Importing this package injects xds://endpoint support into the client.
)

const (
	// The default maximum timeout that will be applied to requests.
	DefaultMaxTimeout = 10 * time.Second
	// The default name to use when registering OpenTelemetry components.
	DefaultOpenTelemetryClientName = "pkg.client"
)

// Implements the PiServiceClient interface.
type PiClient struct {
	// The logr.Logger instance to use.
	logger logr.Logger
	// The client maximum timeout/deadline to use when making requests to a PiService.
	maxTimeout time.Duration
	// The OpenTelemetry tracer to use for spans.
	tracer trace.Tracer
	// The OpenTelemetry meter to use for metrics.
	meter metric.Meter
	// The prefix to use for metrics.
	prefix string
	// A counter for the number of connection errors.
	connectionErrors syncint64.Counter
	// A counter for the number of errors returned by the service.
	serviceErrors syncint64.Counter
	// A gauge for request durations.
	durationMs syncint64.Histogram
}

// Defines a function signature for PiClient options.
type PiClientOption func(*PiClient)

// Create a new PiClient with optional settings.
func NewPiClient(options ...PiClientOption) (*PiClient, error) {
	client := &PiClient{
		logger:     logr.Discard(),
		maxTimeout: DefaultMaxTimeout,
		tracer:     trace.NewNoopTracerProvider().Tracer(DefaultOpenTelemetryClientName),
		meter:      metric.NewNoopMeterProvider().Meter(DefaultOpenTelemetryClientName),
		prefix:     DefaultOpenTelemetryClientName,
	}
	for _, option := range options {
		option(client)
	}
	var err error
	client.connectionErrors, err = client.meter.SyncInt64().Counter(
		client.telemetryName("connection_errors"),
		instrument.WithDescription("The count of connection errors seen by client"),
	)
	if err != nil {
		return nil, fmt.Errorf("error returned while creating connectionErrors Counter: %w", err)
	}
	client.serviceErrors, err = client.meter.SyncInt64().Counter(
		client.telemetryName("service_errors"),
		instrument.WithDescription("The count of service errors received by client"),
	)
	if err != nil {
		return nil, fmt.Errorf("error returned while creating serviceErrors Counter: %w", err)
	}
	client.durationMs, err = client.meter.SyncInt64().Histogram(
		client.telemetryName("request_duration_ms"),
		instrument.WithUnit(unit.Milliseconds),
		instrument.WithDescription("The duration (ms) of requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("error returned while creating durationMs Histogram: %w", err)
	}
	return client, nil
}

// Use the supplied logr.logger.
func WithLogger(logger logr.Logger) PiClientOption {
	return func(c *PiClient) {
		c.logger = logger
	}
}

// Set the maximum timeout for client requests to a PiService.
func WithMaxTimeout(maxTimeout time.Duration) PiClientOption {
	return func(c *PiClient) {
		c.maxTimeout = maxTimeout
	}
}

// Add an OpenTelemetry tracer implementation to the PiService client.
func WithTracer(tracer trace.Tracer) PiClientOption {
	return func(c *PiClient) {
		c.tracer = tracer
	}
}

// Add an OpenTelemetry metric meter implementation to the PiService client.
func WithMeter(meter metric.Meter) PiClientOption {
	return func(c *PiClient) {
		c.meter = meter
	}
}

// Set the prefix to use for OpenTelemetry metrics.
func WithPrefix(prefix string) PiClientOption {
	return func(c *PiClient) {
		c.prefix = prefix
	}
}

// Generates a name for the metric or span.
func (c *PiClient) telemetryName(name string) string {
	if c.prefix == "" {
		return name
	}
	return c.prefix + "." + name
}

// Initiate a gRPC connection to the endpoint and retrieve a single fractional
// decimal digit of pi at the zero-based index.
func (c *PiClient) FetchDigit(ctx context.Context, conn *grpc.ClientConn, index uint64) (uint32, error) {
	logger := c.logger.V(1).WithValues("index", index)
	logger.Info("Starting connection to service")
	attributes := []attribute.KeyValue{
		attribute.Int(c.telemetryName("index"), int(index)),
	}
	ctx, span := c.tracer.Start(ctx, DefaultOpenTelemetryClientName+"/FetchDigit")
	defer span.End()
	span.SetAttributes(attributes...)
	startTimestamp := time.Now()
	span.AddEvent("Building gRPC client")
	client := api.NewPiServiceClient(conn)
	span.AddEvent("Calling GetDigit")
	response, err := client.GetDigit(ctx, &api.GetDigitRequest{
		Index: index,
	})
	durationMs := time.Since(startTimestamp).Milliseconds()
	if err == nil {
		attributes = append(attributes, attribute.Bool(c.telemetryName("success"), true))
		c.durationMs.Record(ctx, durationMs, attributes...)
		logger.Info("Response from remote", "result", response.Digit, "metadata", response.Metadata)
		return response.Digit, nil
	}
	span.RecordError(err)
	span.SetStatus(otelcodes.Error, err.Error())
	attributes = append(attributes, attribute.Bool(c.telemetryName("success"), false))
	c.durationMs.Record(ctx, durationMs, attributes...)
	// Simple but dumb way to determine if the error is a connection error or
	// service error; if the error can be marshaled to a gRPC status, then it
	// is an error raised by the service implementation, not related to
	// connectivity.
	if _, ok := status.FromError(err); ok {
		c.serviceErrors.Add(ctx, 1, attributes...)
	} else {
		c.connectionErrors.Add(ctx, 1, attributes...)
	}
	return 0, fmt.Errorf("failure calling GetDigit: %w", err)
}
