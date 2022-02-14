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
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	// The default timeout that will be applied to connections.
	DEFAULT_CLIENT_TIMEOUT = 10 * time.Second
	// The default name to use when registering OpenTelemetry components
	DEFAULT_OPENTELEMETRY_CLIENT_NAME = "client"
)

// Implements the PiServiceClient interface.
type PiClient struct {
	// The logr.Logger instance to use
	logger logr.Logger
	// The client timeout/deadline to use when creating connections to a PiService.
	timeout time.Duration
	// The OpenTelemetry tracer to use for spans
	tracer trace.Tracer
	// The OpenTelemetry meter to use for metrics
	meter metric.Meter
	// The prefix to use for metrics
	prefix string
	// A counter for the number of connection errors
	connectionErrors metric.Int64Counter
	// A counter for the number of response errors
	responseErrors metric.Int64Counter
	// A gauge for request durations
	durationMs metric.Float64Histogram
}

// Defines a function signature for PiClient options.
type PiClientOption func(*PiClient)

// Create a new PiClient with optional settings.
func NewPiClient(options ...PiClientOption) *PiClient {
	client := &PiClient{
		logger:  logr.Discard(),
		timeout: DEFAULT_CLIENT_TIMEOUT,
		tracer:  trace.NewNoopTracerProvider().Tracer(DEFAULT_OPENTELEMETRY_CLIENT_NAME),
		meter:   metric.NewNoopMeterProvider().Meter(DEFAULT_OPENTELEMETRY_CLIENT_NAME),
		prefix:  DEFAULT_OPENTELEMETRY_CLIENT_NAME,
	}
	for _, option := range options {
		option(client)
	}
	client.connectionErrors = client.newInt64Counter("connection_errors", "The count of connection errors")
	client.responseErrors = client.newInt64Counter("response_errors", "The count of error responses")
	client.durationMs = client.newFloat64Histogram("request_duration_ms", "The duration (ms) of requests")
	return client
}

// Use the supplied logr.logger.
func WithLogger(logger logr.Logger) PiClientOption {
	return func(c *PiClient) {
		c.logger = logger
	}
}

// Set the client timeout.
func WithTimeout(timeout time.Duration) PiClientOption {
	return func(c *PiClient) {
		c.timeout = timeout
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

// Generates a name for the metric
func (c *PiClient) metricName(name string) string {
	if c.prefix == "" {
		return name
	}
	return fmt.Sprintf("%s_%s", c.prefix, name)
}

// Create a new Int64 OpenTelemetry metric counter
func (c *PiClient) newInt64Counter(name string, description string) metric.Int64Counter {
	return metric.Must(c.meter).NewInt64Counter(c.metricName(name), metric.WithDescription(description))
}

// Create a new floating point OpenTelemetry metric gauge
func (c *PiClient) newFloat64Histogram(name string, description string) metric.Float64Histogram {
	return metric.Must(c.meter).NewFloat64Histogram(c.metricName(name), metric.WithDescription(description))
}

// Initiate a gRPC connection to the endpoint and retrieve a single fractional
// decimal digit of pi at the zero-based index.
func (c *PiClient) FetchDigit(endpoint string, index uint64) (uint32, error) {
	logger := c.logger.V(1).WithValues("endpoint", endpoint, "index", index)
	logger.Info("Starting connection to service")
	attributes := []attribute.KeyValue{
		attribute.String("endpoint", endpoint),
		attribute.Int("index", int(index)),
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	startTs := time.Now()
	conn, err := grpc.DialContext(ctx, endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock(), grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()))
	if err != nil {
		c.meter.RecordBatch(
			ctx,
			attributes,
			c.connectionErrors.Measurement(1),
		)
		return 0, err
	}
	defer conn.Close()
	client := api.NewPiServiceClient(conn)
	response, err := client.GetDigit(ctx, &api.GetDigitRequest{
		Index: index,
	})
	duration := float64(time.Since(startTs) / time.Millisecond)
	if err != nil {
		c.meter.RecordBatch(
			ctx,
			attributes,
			c.responseErrors.Measurement(1),
			c.durationMs.Measurement(duration),
		)
		return 0, err
	}
	c.meter.RecordBatch(
		ctx,
		attributes,
		c.durationMs.Measurement(duration),
	)
	logger.Info("Response from remote", "result", response.Digit, "metadata", response.Metadata)

	return response.Digit, nil
}
