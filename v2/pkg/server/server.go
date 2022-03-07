// Package server implements a gRPC server (and optional REST gateway) implementation
// that satisfies the PiServiceClient interface requirements, with optional
// OpenTelemetry metrics and traces.
package server

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	pi "github.com/memes/pi/v2"
	api "github.com/memes/pi/v2/api/v2"
	cachepkg "github.com/memes/pi/v2/pkg/cache"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

const (
	// The default name to use when registering OpenTelemetry components.
	DefaultOpenTelemetryServerName = "server"
)

type PiServer struct {
	api.UnimplementedPiServiceServer
	// The logr.Logger implementation to use
	logger logr.Logger
	// An optional cache implementation
	cache cachepkg.Cache
	// Holds the instance specific metadata that will be returned in PiService responses
	metadata *api.GetDigitMetadata
	// The OpenTelemetry tracer to use for spans
	tracer trace.Tracer
	// The OpenTelemetry meter to use for metrics
	meter metric.Meter
	// The prefix to use for metrics
	prefix string
	// A gauge for calculation durations
	calculationMs metric.Float64Histogram
	// A counter for the number of errors returned by cache
	cacheErrors metric.Int64Counter
	// A counter for cache hits
	cacheHits metric.Int64Counter
	// A counter for cache misses
	cacheMisses metric.Int64Counter
	// gRPC transport credentials
	creds credentials.TransportCredentials
}

// Defines the function signature for PiServer options.
type PiServerOption func(*PiServer)

// Create a new piServer and apply any options.
func NewPiServer(options ...PiServerOption) *PiServer {
	server := &PiServer{
		logger: logr.Discard(),
		cache:  cachepkg.NewNoopCache(),
		tracer: trace.NewNoopTracerProvider().Tracer(DefaultOpenTelemetryServerName),
		meter:  metric.NewNoopMeterProvider().Meter(DefaultOpenTelemetryServerName),
	}
	for _, option := range options {
		option(server)
	}
	server.calculationMs = server.newFloat64Histogram("calc_duration_ms", "The duration (ms) of calculations")
	server.cacheErrors = server.newInt64Counter("cache_errors", "The count of cache errors")
	server.cacheHits = server.newInt64Counter("cache_hits", "The count of cache hits")
	server.cacheMisses = server.newInt64Counter("cache_misses", "The count of cache misses")
	return server
}

// Use the supplied logger for the server and pi packages.
func WithLogger(logger logr.Logger) PiServerOption {
	return func(s *PiServer) {
		s.logger = logger
		pi.Logger = logger
	}
}

// Use the Cache implementation to store BBPDigits results to avoid recalculation
// of a digit that has already been calculated.
func WithCache(cache cachepkg.Cache) PiServerOption {
	return func(s *PiServer) {
		if cache != nil {
			s.cache = cache
		}
	}
}

// Populate a metadata structure for this instance.
func WithMetadata(labels map[string]string) PiServerOption {
	return func(s *PiServer) {
		metadata := api.GetDigitMetadata{
			Labels: labels,
		}
		if hostname, err := os.Hostname(); err == nil {
			metadata.Identity = hostname
		}
		if addrs, err := net.InterfaceAddrs(); err == nil {
			addresses := make([]string, 0, len(addrs))
			for _, addr := range addrs {
				if ip, ok := addr.(*net.IPNet); ok && ip.IP.IsGlobalUnicast() {
					addresses = append(addresses, ip.IP.String())
				}
			}
			metadata.Addresses = addresses
		}
		s.metadata = &metadata
	}
}

// Add an OpenTelemetry tracer implementation to the PiService server.
func WithTracer(tracer trace.Tracer) PiServerOption {
	return func(s *PiServer) {
		if tracer != nil {
			s.tracer = tracer
		}
	}
}

// Add an OpenTelemetry metric meter implementation to the PiService server.
func WithMeter(meter metric.Meter) PiServerOption {
	return func(s *PiServer) {
		s.meter = meter
	}
}

// Set the prefix to use for OpenTelemetry metrics.
func WithPrefix(prefix string) PiServerOption {
	return func(s *PiServer) {
		s.prefix = prefix
	}
}

// Set the TransportCredentials to use for Pi Service connection.
func WithTransportCredentials(creds credentials.TransportCredentials) PiServerOption {
	return func(s *PiServer) {
		s.creds = creds
	}
}

// Generates a name for the metric.
func (s *PiServer) metricName(name string) string {
	if s.prefix == "" {
		return name
	}
	return fmt.Sprintf("%s_%s", s.prefix, name)
}

// Create a new Int64 OpenTelemetry metric counter.
func (s *PiServer) newInt64Counter(name, description string) metric.Int64Counter {
	return metric.Must(s.meter).NewInt64Counter(s.metricName(name), metric.WithDescription(description))
}

// Create a new floating point OpenTelemetry metric gauge.
func (s *PiServer) newFloat64Histogram(name, description string) metric.Float64Histogram {
	return metric.Must(s.meter).NewFloat64Histogram(s.metricName(name), metric.WithDescription(description))
}

// Implement the PiService GetDigit RPC method.
func (s *PiServer) GetDigit(ctx context.Context, in *api.GetDigitRequest) (*api.GetDigitResponse, error) {
	logger := s.logger.WithValues("index", in.Index)
	logger.Info("GetDigit: enter")
	attributes := []attribute.KeyValue{
		attribute.Int("index", int(in.Index)),
	}
	ctx, span := s.tracer.Start(ctx, "GetDigit")
	defer span.End()
	span.SetAttributes(attributes...)
	var duration float64
	cacheIndex := (in.Index / 9) * 9
	key := strconv.FormatUint(cacheIndex, 16)
	span.AddEvent("Checking cache")
	digits, err := s.cache.GetValue(ctx, key)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		s.meter.RecordBatch(
			ctx,
			attributes,
			s.cacheErrors.Measurement(1),
		)
		return nil, fmt.Errorf("cache %T GetValue method returned an error: %w", s.cache, err)
	}
	measurements := []metric.Measurement{}
	if digits == "" {
		measurements = append(measurements, s.cacheMisses.Measurement(1))
		attributes := append(attributes, attribute.Bool("cache_hit", false))
		span.SetAttributes(attributes...)
		span.AddEvent("Calculating fractional digits")
		ts := time.Now()
		digits = pi.BBPDigits(cacheIndex)
		duration = float64(time.Since(ts) / time.Millisecond)
		measurements = append(measurements, s.calculationMs.Measurement(duration))
		err = s.cache.SetValue(ctx, key, digits)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, err.Error())
			measurements = append(measurements, s.cacheErrors.Measurement(1))
			s.meter.RecordBatch(
				ctx,
				attributes,
				measurements...,
			)
			return nil, fmt.Errorf("cache %T SetValue method returned an error: %w", s.cache, err)
		}
	} else {
		measurements = append(measurements, s.cacheHits.Measurement(1))
		attributes := append(attributes, attribute.Bool("cache_hit", true))
		span.SetAttributes(attributes...)
	}
	offset := in.Index % 9
	digit, err := strconv.ParseUint(digits[offset:offset+1], 10, 32)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		s.meter.RecordBatch(
			ctx,
			attributes,
			measurements...,
		)
		return nil, fmt.Errorf("method GetDigit failed to parse as uint: %w", err)
	}
	logger.Info("GetDigit: exit", "digit", digit)
	s.meter.RecordBatch(
		ctx,
		attributes,
		measurements...,
	)
	return &api.GetDigitResponse{
		Index:    in.Index,
		Digit:    uint32(digit),
		Metadata: s.metadata,
	}, nil
}

// Implement the gRPC health service Check method.
func (s *PiServer) Check(ctx context.Context, in *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
}

// Satisfy the gRPC health service Watch method - always returns an Unimplemented error.
func (s *PiServer) Watch(in *grpc_health_v1.HealthCheckRequest, _ grpc_health_v1.Health_WatchServer) error {
	return status.Error(codes.Unimplemented, "unimplemented") // nolint:wrapcheck
}

// Create a new grpc.Server that is ready to be attached to a net.Listener.
func (s *PiServer) NewGrpcServer() *grpc.Server {
	options := []grpc.ServerOption{
		grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
	}
	if s.creds != nil {
		options = append(options, grpc.Creds(s.creds))
	}
	grpcServer := grpc.NewServer(options...)
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	api.RegisterPiServiceServer(grpcServer, s)
	reflection.Register(grpcServer)
	return grpcServer
}

// Create a new REST gateway handler that translates and forwards incoming REST
// requests to the specified gRPC endpoint address.
func (s *PiServer) NewRestGatewayHandler(ctx context.Context, grpcAddress string) (http.Handler, error) {
	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
	}
	if err := api.RegisterPiServiceHandlerFromEndpoint(ctx, mux, grpcAddress, opts); err != nil {
		return nil, fmt.Errorf("failed to register PiService handler for REST gateway: %w", err)
	}
	if err := mux.HandlePath("GET", "/v1/digit/{index}", func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		span := trace.SpanFromContext(r.Context())
		span.SetAttributes(attribute.String("index", pathParams["index"]))
		span.SetStatus(otelcodes.Error, "v1 API")
		w.WriteHeader(http.StatusGone)
	}); err != nil {
		return nil, fmt.Errorf("failed to register /v1 handler for REST gateway: %w", err)
	}
	return otelhttp.NewHandler(mux, "rest-gateway", otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents)), nil
}
