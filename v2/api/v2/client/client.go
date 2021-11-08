package client

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	api "github.com/memes/pi/v2/api/v2"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

type PiClient struct {
	logger  logr.Logger
	timeout time.Duration
	// Holds the instance specific metadata that will be returned in PiService responses
	metadata *api.GetDigitMetadata
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

type PiClientOption func(*PiClient)

func NewPiClient(options ...PiClientOption) *PiClient {
	client := &PiClient{
		logger: logr.Discard(),
		tracer: otel.Tracer(""),
	}
	// Set a default set of metrics
	WithMeter("client", global.Meter(""))(client)
	for _, option := range options {
		option(client)
	}
	return client
}

// Use the supplied logger.
func WithLogger(logger logr.Logger) PiClientOption {
	return func(c *PiClient) {
		c.logger = logger
	}
}

func WithTimeout(timeout time.Duration) PiClientOption {
	return func(c *PiClient) {
		c.timeout = timeout
	}
}

func WithTracer(tracer trace.Tracer) PiClientOption {
	return func(c *PiClient) {
		c.tracer = tracer
	}
}

func WithMeter(prefix string, meter metric.Meter) PiClientOption {
	return func(c *PiClient) {
		c.meter = meter
		c.connectionErrors = metric.Must(meter).NewInt64Counter(prefix+"/connection_errors", metric.WithDescription("The count of connection errors"))
		c.responseErrors = metric.Must(meter).NewInt64Counter(prefix+"/response_errors", metric.WithDescription("The count of error responses"))
		c.durationMs = metric.Must(meter).NewFloat64Histogram(prefix+"/request_duration_ms", metric.WithDescription("The duration (ms) of requests"))
	}
}

// Initiate a gRPC connect to endpoint and retrieve a single fractional digit of
// pi at the zero-based index.
func (c *PiClient) FetchDigit(endpoint string, index uint64) (uint32, error) {
	logger := c.logger.V(0).WithValues("endpoint", endpoint, "index", index)
	logger.Info("Starting connection to service")
	attributes := []attribute.KeyValue{
		attribute.String("endpoint", endpoint),
		attribute.Int("index", int(index)),
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
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
	ts := time.Now()
	response, err := client.GetDigit(ctx, &api.GetDigitRequest{
		Index: index,
	})
	duration := float64(time.Since(ts) / time.Millisecond)
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
