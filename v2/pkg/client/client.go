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
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/unit"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	// Importing this package injects xds://endpoint support into the client.
	_ "google.golang.org/grpc/xds"
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
	connectionErrors metric.Int64Counter
	// A counter for the number of response errors.
	responseErrors metric.Int64Counter
	// A gauge for request durations.
	durationMs metric.Int64Histogram
	// gRPC transport credentials for Pi Service gRPC client.
	creds credentials.TransportCredentials
	// gRPC server authority to specify for TLS verification.
	authority string
	// Specifies the gRPC user-agent.
	userAgent string
}

// Defines a function signature for PiClient options.
type PiClientOption func(*PiClient)

// Create a new PiClient with optional settings.
func NewPiClient(options ...PiClientOption) *PiClient {
	client := &PiClient{
		logger:     logr.Discard(),
		maxTimeout: DefaultMaxTimeout,
		tracer:     trace.NewNoopTracerProvider().Tracer(DefaultOpenTelemetryClientName),
		meter:      metric.NewNoopMeterProvider().Meter(DefaultOpenTelemetryClientName),
		prefix:     DefaultOpenTelemetryClientName,
		userAgent:  DefaultOpenTelemetryClientName + "/unspecified",
	}
	for _, option := range options {
		option(client)
	}
	client.connectionErrors = metric.Must(client.meter).NewInt64Counter(
		client.telemetryName("connection_errors"),
		metric.WithDescription("The count of connection errors seen by client"),
	)
	client.responseErrors = metric.Must(client.meter).NewInt64Counter(
		client.telemetryName("response_errors"),
		metric.WithDescription("The count of error responses received by client"),
	)
	client.durationMs = metric.Must(client.meter).NewInt64Histogram(
		client.telemetryName("request_duration_ms"),
		metric.WithUnit(unit.Milliseconds),
		metric.WithDescription("The duration (ms) of requests"),
	)
	return client
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

// Set the TransportCredentials to use for Pi Service connection.
func WithTransportCredentials(creds credentials.TransportCredentials) PiClientOption {
	return func(c *PiClient) {
		c.creds = creds
	}
}

// Set the authority to use for Pi Service client calls.
func WithAuthority(authority string) PiClientOption {
	return func(c *PiClient) {
		c.authority = authority
	}
}

// Set the user-agent used by Pi Service client.
func WithUserAgent(userAgent string) PiClientOption {
	return func(c *PiClient) {
		c.userAgent = userAgent
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
func (c *PiClient) FetchDigit(endpoint string, index uint64) (uint32, error) {
	logger := c.logger.V(1).WithValues("endpoint", endpoint, "index", index)
	logger.Info("Starting connection to service")
	attributes := []attribute.KeyValue{
		attribute.String(c.telemetryName("endpoint"), endpoint),
		attribute.Int(c.telemetryName("index"), int(index)),
		attribute.String(c.telemetryName("user-agent"), c.userAgent),
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.maxTimeout)
	defer cancel()
	ctx, span := c.tracer.Start(ctx, DefaultOpenTelemetryClientName+"/FetchDigit")
	defer span.End()
	span.SetAttributes(attributes...)
	span.AddEvent("Building gRPC client")
	startTimestamp := time.Now()
	options := []grpc.DialOption{
		grpc.WithUserAgent(c.userAgent),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
	}
	if c.creds != nil {
		options = append(options, grpc.WithTransportCredentials(c.creds))
	}
	if c.authority != "" {
		options = append(options, grpc.WithAuthority(c.authority))
	}
	conn, err := grpc.DialContext(ctx, endpoint, options...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		c.meter.RecordBatch(
			ctx,
			attributes,
			c.connectionErrors.Measurement(1),
		)
		return 0, fmt.Errorf("failure establishing client dial context: %w", err)
	}
	defer conn.Close()
	client := api.NewPiServiceClient(conn)
	span.AddEvent("Calling GetDigit")
	response, err := client.GetDigit(ctx, &api.GetDigitRequest{
		Index: index,
	})
	duration := time.Since(startTimestamp)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		attributes = append(attributes, attribute.Bool(c.telemetryName("success"), false))
		c.meter.RecordBatch(
			ctx,
			attributes,
			c.responseErrors.Measurement(1),
			c.durationMs.Measurement(duration.Milliseconds()),
		)
		return 0, fmt.Errorf("failure calling GetDigit: %w", err)
	}
	attributes = append(attributes, attribute.Bool(c.telemetryName("success"), true))
	c.meter.RecordBatch(
		ctx,
		attributes,
		c.durationMs.Measurement(duration.Milliseconds()),
	)
	logger.Info("Response from remote", "result", response.Digit, "metadata", response.Metadata)
	return response.Digit, nil
}
