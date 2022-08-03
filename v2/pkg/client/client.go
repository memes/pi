// Package client implements a gRPC client implementation that satisfies the
// PiServiceClient interface requirements with optional OpenTelemetry metrics and
// traces.
package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	api "github.com/memes/pi/v2/internal/api/v2"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
	"go.opentelemetry.io/otel/metric/unit"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"

	"google.golang.org/grpc/status"
	_ "google.golang.org/grpc/xds" // Importing this package injects xds://endpoint support into the client.
)

const (
	// The default maximum timeout that will be applied to requests.
	DefaultMaxTimeout = 10 * time.Second
	// The default name to use when using OpenTelemetry components.
	OpenTelemetryPackageIdentifier = "pkg.client"
	// A default round-robin configuration to use with client-side load balancer.
	DefaultRoundRobinConfig = `{"loadBalancingConfig": [{"round_robin":{}}]}`
)

// Implements the PiServiceClient interface.
type PiClient struct {
	// The logr.Logger instance to use.
	logger logr.Logger
	// The client maximum timeout/deadline to use when making requests to a PiService.
	maxTimeout time.Duration
	// A counter for the number of connection errors.
	connectionErrors syncint64.Counter
	// A counter for the number of errors returned by the service.
	serviceErrors syncint64.Counter
	// A gauge for request durations.
	durationMs syncint64.Histogram
	// gRPC metadata to add to service requests.
	metadata metadata.MD
	// gRPC endpoint; see https://github.com/grpc/grpc/blob/master/doc/naming.md
	// for full details.
	endpoint string
	// A set of gRPC DialOptions to apply to client connection.
	dialOptions []grpc.DialOption
	// Mutex to ensure the shared client connection is created on first call only.
	initConn sync.Once
	// The shared gRPC client connection; will be reused by each request.
	conn *grpc.ClientConn
}

// Defines a function signature for PiClient options.
type PiClientOption func(*PiClient)

// Create a new PiClient with optional settings.
func NewPiClient(options ...PiClientOption) (*PiClient, error) {
	client := &PiClient{
		logger:     logr.Discard(),
		maxTimeout: DefaultMaxTimeout,
		dialOptions: []grpc.DialOption{
			grpc.WithDefaultServiceConfig(DefaultRoundRobinConfig),
			grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
		},
	}
	for _, option := range options {
		option(client)
	}
	var err error
	client.connectionErrors, err = global.Meter(OpenTelemetryPackageIdentifier).SyncInt64().Counter(
		OpenTelemetryPackageIdentifier+".connection_errors",
		instrument.WithDescription("The count of connection errors seen by client"),
	)
	if err != nil {
		return nil, fmt.Errorf("error returned while creating connectionErrors Counter: %w", err)
	}
	client.serviceErrors, err = global.Meter(OpenTelemetryPackageIdentifier).SyncInt64().Counter(
		OpenTelemetryPackageIdentifier+".service_errors",
		instrument.WithDescription("The count of service errors received by client"),
	)
	if err != nil {
		return nil, fmt.Errorf("error returned while creating serviceErrors Counter: %w", err)
	}
	client.durationMs, err = global.Meter(OpenTelemetryPackageIdentifier).SyncInt64().Histogram(
		OpenTelemetryPackageIdentifier+".request_duration_ms",
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

// Add the headers values to the PiService client gRPC metadata.
func WithHeaders(headers map[string]string) PiClientOption {
	return func(c *PiClient) {
		c.metadata = metadata.Join(c.metadata, metadata.New(headers))
	}
}

// Set the endpoint to use for gRPC connections.
func WithEndpoint(endpoint string) PiClientOption {
	return func(c *PiClient) {
		c.endpoint = endpoint
	}
}

// Set the transport credentials to use for PiService gRPC connection.
func WithTransportCredentials(creds credentials.TransportCredentials) PiClientOption {
	return func(c *PiClient) {
		if creds != nil {
			c.dialOptions = append(c.dialOptions, grpc.WithTransportCredentials(creds))
		}
	}
}

// Set the authority string to use for gRPC connections.
func WithAuthority(authority string) PiClientOption {
	return func(c *PiClient) {
		if authority != "" {
			c.dialOptions = append(c.dialOptions, grpc.WithAuthority(authority))
		}
	}
}

// Set the user-agent string to use for gRPC connections.
func WithUserAgent(userAgent string) PiClientOption {
	return func(c *PiClient) {
		if userAgent != "" {
			c.dialOptions = append(c.dialOptions, grpc.WithUserAgent(userAgent))
		}
	}
}

// Initiate a gRPC request to Pi Service and retrieve a single fractional decimal
// digit of pi at the zero-based index.
func (c *PiClient) FetchDigit(ctx context.Context, index uint64) (uint32, error) {
	logger := c.logger.V(1).WithValues("index", index)
	logger.Info("Starting connection to service")
	attributes := []attribute.KeyValue{
		attribute.Int(OpenTelemetryPackageIdentifier+".index", int(index)),
	}
	ctx, span := otel.Tracer(OpenTelemetryPackageIdentifier).Start(ctx, OpenTelemetryPackageIdentifier+"/FetchDigit")
	defer span.End()
	span.SetAttributes(attributes...)
	var err error
	c.initConn.Do(func() {
		span.AddEvent("Building PiService client")
		c.conn, err = grpc.DialContext(ctx, c.endpoint, c.dialOptions...)
	})
	if err != nil {
		return 0, fmt.Errorf("failure establishing client dial context: %w", err)
	}

	span.AddEvent("Calling GetDigit")
	startTimestamp := time.Now()
	response, err := api.NewPiServiceClient(c.conn).GetDigit(metadata.NewOutgoingContext(ctx, c.metadata), &api.GetDigitRequest{
		Index: index,
	})
	durationMs := time.Since(startTimestamp).Milliseconds()
	if err == nil {
		attributes = append(attributes, attribute.Bool(OpenTelemetryPackageIdentifier+".success", true))
		c.durationMs.Record(ctx, durationMs, attributes...)
		logger.Info("Response from remote", "result", response.Digit, "metadata", response.Metadata)
		return response.Digit, nil
	}
	span.RecordError(err)
	span.SetStatus(otelcodes.Error, err.Error())
	attributes = append(attributes, attribute.Bool(OpenTelemetryPackageIdentifier+".success", false))
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

// Shutdown the gRPC client connection that is used by this PiClient. After calling
// this function FetchDigit will return an error on every call.
func (c *PiClient) Shutdown(_ context.Context) error {
	c.logger.V(1).Info("Shutting down PiClient gRPC connection")
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			return fmt.Errorf("error closing gRPC client connection: %w", err)
		}
	}
	return nil
}
