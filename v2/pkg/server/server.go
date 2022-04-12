// Package server implements a gRPC server (and optional REST gateway) implementation
// that satisfies the PiServiceClient interface requirements, with optional
// OpenTelemetry metrics and traces.
package server

import (
	"fmt"
	"net/http"
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
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
	"go.opentelemetry.io/otel/metric/nonrecording"
	"go.opentelemetry.io/otel/metric/unit"
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
	"google.golang.org/grpc/xds"
)

const (
	// The default name to use when registering OpenTelemetry components.
	DefaultOpenTelemetryServerName = "pkg.server"
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
	// The prefix to use for metrics and spans
	prefix string
	// A gauge for calculation durations
	calculationMs syncint64.Histogram
	// A counter for the number of errors returned by cache
	cacheErrors syncint64.Counter
	// A counter for cache hits
	cacheHits syncint64.Counter
	// A counter for cache misses
	cacheMisses syncint64.Counter
	// Transport credentials to use with gRPC PiService listener.
	serverGRPCCredentials credentials.TransportCredentials
	// Transport credentials to use with REST gateway gRPC client.
	restClientGRPCCredentials credentials.TransportCredentials
	// Allow overriding of gRPC authority header through REST gateway.
	restClientAuthority string
}

// Defines the function signature for PiServer options.
type PiServerOption func(*PiServer)

// Create a new piServer and apply any options.
func NewPiServer(options ...PiServerOption) (*PiServer, error) {
	server := &PiServer{
		logger:                    logr.Discard(),
		cache:                     cachepkg.NewNoopCache(),
		tracer:                    trace.NewNoopTracerProvider().Tracer(DefaultOpenTelemetryServerName),
		meter:                     nonrecording.NewNoopMeterProvider().Meter(DefaultOpenTelemetryServerName),
		restClientGRPCCredentials: insecure.NewCredentials(),
	}
	for _, option := range options {
		option(server)
	}
	var err error
	server.calculationMs, err = server.meter.SyncInt64().Histogram(
		server.telemetryName("calc_duration_ms"),
		instrument.WithUnit(unit.Milliseconds),
		instrument.WithDescription("The duration (ms) of calculations"),
	)
	if err != nil {
		return nil, fmt.Errorf("error returned while creating calculationMs Histogram: %w", err)
	}
	server.cacheErrors, err = server.meter.SyncInt64().Counter(
		server.telemetryName("cache_errors"),
		instrument.WithDescription("The count of error responses from digit cache"),
	)
	if err != nil {
		return nil, fmt.Errorf("error returned while creating cacheErrors Counter: %w", err)
	}
	server.cacheHits, err = server.meter.SyncInt64().Counter(
		server.telemetryName("cache_hits"),
		instrument.WithDescription("The count of cache hits"),
	)
	if err != nil {
		return nil, fmt.Errorf("error returned while creating cacheHits Counter: %w", err)
	}
	server.cacheMisses, err = server.meter.SyncInt64().Counter(
		server.telemetryName("cache_misses"),
		instrument.WithDescription("The count of cache misses"),
	)
	if err != nil {
		return nil, fmt.Errorf("error returned while creating cacheMisses Counter: %w", err)
	}
	return server, nil
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
func WithMetadata(metadata *api.GetDigitMetadata) PiServerOption {
	return func(s *PiServer) {
		s.metadata = metadata
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

// Set the TransportCredentials to use for Pi Service gRPC listener.
func WithGRPCServerTransportCredentials(serverCredentials credentials.TransportCredentials) PiServerOption {
	return func(s *PiServer) {
		s.serverGRPCCredentials = serverCredentials
	}
}

// Set the TransportCredentials to use for Pi Service REST-to-gRPC client.
func WithRestClientGRPCTransportCredentials(restClientGRPCCredentials credentials.TransportCredentials) PiServerOption {
	return func(s *PiServer) {
		s.restClientGRPCCredentials = restClientGRPCCredentials
	}
}

// Set the authority string to use for REST-gRPC gateway calls.
func WithRestClientAuthority(restClientAuthority string) PiServerOption {
	return func(s *PiServer) {
		s.restClientAuthority = restClientAuthority
	}
}

// Generates a name for the metric or span.
func (s *PiServer) telemetryName(name string) string {
	if s.prefix == "" {
		return name
	}
	return s.prefix + "." + name
}

// Implement the PiService GetDigit RPC method.
func (s *PiServer) GetDigit(ctx context.Context, in *api.GetDigitRequest) (*api.GetDigitResponse, error) {
	logger := s.logger.WithValues("index", in.Index)
	logger.Info("GetDigit: enter")
	cacheIndex := (in.Index / 9) * 9
	key := strconv.FormatUint(cacheIndex, 16)
	attributes := []attribute.KeyValue{
		attribute.Int(s.telemetryName("index"), int(in.Index)),
		attribute.String(s.telemetryName("cacheKey"), key),
	}
	ctx, span := s.tracer.Start(ctx, DefaultOpenTelemetryServerName+"/GetDigit")
	defer span.End()
	span.SetAttributes(attributes...)
	span.AddEvent("Checking cache")
	digits, err := s.cache.GetValue(ctx, key)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		s.cacheErrors.Add(ctx, 1, attributes...)
		return nil, fmt.Errorf("cache %T GetValue method returned an error: %w", s.cache, err)
	}
	if digits == "" {
		attributes := append(attributes, attribute.Bool(s.telemetryName("cache_hit"), false))
		span.SetAttributes(attributes...)
		span.AddEvent("Calculating fractional digits")
		s.cacheMisses.Add(ctx, 1, attributes...)
		ts := time.Now()
		digits = pi.BBPDigits(cacheIndex)
		s.calculationMs.Record(ctx, time.Since(ts).Milliseconds(), attributes...)
		err = s.cache.SetValue(ctx, key, digits)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, err.Error())
			s.cacheErrors.Add(ctx, 1, attributes...)
			return nil, fmt.Errorf("cache %T SetValue method returned an error: %w", s.cache, err)
		}
	} else {
		attributes := append(attributes, attribute.Bool(s.telemetryName("cache_hit"), true))
		span.SetAttributes(attributes...)
		s.cacheHits.Add(ctx, 1, attributes...)
	}
	offset := in.Index % 9
	digit, err := strconv.ParseUint(digits[offset:offset+1], 10, 32)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())

		return nil, fmt.Errorf("method GetDigit failed to parse as uint: %w", err)
	}
	logger.Info("GetDigit: exit", "digit", digit)
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
	return status.Error(codes.Unimplemented, "unimplemented") //nolint:wrapcheck // This error is not received by application code
}

// Return a set of gRPC options for this instance.
func (s *PiServer) grpcOptions() []grpc.ServerOption {
	options := []grpc.ServerOption{
		grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
	}
	if s.serverGRPCCredentials != nil {
		options = append(options, grpc.Creds(s.serverGRPCCredentials))
	}
	return options
}

// Create a new grpc.Server that is ready to be attached to a net.Listener.
func (s *PiServer) NewGrpcServer() *grpc.Server {
	s.logger.V(1).Info("Building a standard gRPC server")
	options := s.grpcOptions()
	grpcServer := grpc.NewServer(options...)
	grpc_health_v1.RegisterHealthServer(grpcServer, health.NewServer())
	api.RegisterPiServiceServer(grpcServer, s)
	reflection.Register(grpcServer)
	return grpcServer
}

// Create a new xds.GRPCServer that is ready to be attached to a net.Listener.
func (s *PiServer) NewXDSServer() *xds.GRPCServer {
	s.logger.V(1).Info("xDS is enabled; building an xDS aware gRPC server")
	options := s.grpcOptions()
	xdsServer := xds.NewGRPCServer(options...)
	grpc_health_v1.RegisterHealthServer(xdsServer, health.NewServer())
	api.RegisterPiServiceServer(xdsServer, s)
	reflection.Register(xdsServer)
	return xdsServer
}

// Registers the gRPC services to the
// Create a new REST gateway handler that translates and forwards incoming REST
// requests to the specified gRPC endpoint address.
func (s *PiServer) NewRestGatewayHandler(ctx context.Context, grpcAddress string) (http.Handler, error) {
	mux := runtime.NewServeMux()
	options := []grpc.DialOption{
		grpc.WithTransportCredentials(s.restClientGRPCCredentials),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
	}
	if s.restClientAuthority != "" {
		options = append(options, grpc.WithAuthority(s.restClientAuthority))
	}
	if err := api.RegisterPiServiceHandlerFromEndpoint(ctx, mux, grpcAddress, options); err != nil {
		return nil, fmt.Errorf("failed to register PiService handler for REST gateway: %w", err)
	}
	if err := mux.HandlePath("GET", "/api/v2/swagger.json",
		func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
			w.Header().Add("Content-Type", "application/json")
			if _, err := w.Write(api.SwaggerJSON); err != nil {
				s.logger.Error(err, "Writing swagger JSON to response raised an error; continuing")
			}
		},
	); err != nil {
		return nil, fmt.Errorf("failed to register /api/v2 handler for swagger definition: %w", err)
	}
	if err := mux.HandlePath("GET", "/v1/digit/{index}",
		func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
			span := trace.SpanFromContext(r.Context())
			span.SetAttributes(attribute.String(s.telemetryName("index"), pathParams["index"]))
			span.SetStatus(otelcodes.Error, "v1 API")
			w.WriteHeader(http.StatusGone)
		},
	); err != nil {
		return nil, fmt.Errorf("failed to register /v1 handler for REST gateway: %w", err)
	}
	return otelhttp.NewHandler(mux,
		s.telemetryName("REST-GRPC Gateway"),
		otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents),
	), nil
}
