// Package client implements a gRPC client implementation that satisfies the
// PiServiceClient interface requirements with optional OpenTelemetry metrics and
// traces.
package client

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	api "github.com/memes/pi/v2/api/v2"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
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
	}
	// Set a default set of metrics agains a noop meter provider
	WithMeter(DEFAULT_OPENTELEMETRY_CLIENT_NAME, metric.NewNoopMeterProvider().Meter(DEFAULT_OPENTELEMETRY_CLIENT_NAME))(client)
	for _, option := range options {
		option(client)
	}
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
func WithMeter(prefix string, meter metric.Meter) PiClientOption {
	return func(c *PiClient) {
		if prefix == "" {
			prefix = DEFAULT_OPENTELEMETRY_CLIENT_NAME
		}
		c.meter = meter
		c.connectionErrors = metric.Must(meter).NewInt64Counter(prefix+"_connection_errors", metric.WithDescription("The count of connection errors"))
		c.responseErrors = metric.Must(meter).NewInt64Counter(prefix+"_response_errors", metric.WithDescription("The count of error responses"))
		c.durationMs = metric.Must(meter).NewFloat64Histogram(prefix+"_request_duration_ms", metric.WithDescription("The duration (ms) of requests"))
	}
}

// Initiate a gRPC connection to the endpoint and retrieve a single fractional
// decimal digit of pi at the zero-based index.
func (c *PiClient) FetchDigit(endpoint string, index uint64) (uint32, error) {
	logger := c.logger.V(0).WithValues("endpoint", endpoint, "index", index)
	logger.Info("Starting connection to service")
	attributes := []attribute.KeyValue{
		attribute.String("endpoint", endpoint),
		attribute.Int("index", int(index)),
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	startTs := time.Now()
	conn, err := grpc.DialContext(ctx, endpoint, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()))
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
