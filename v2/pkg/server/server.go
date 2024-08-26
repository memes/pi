// Package server implements a gRPC server (and optional REST gateway) implementation
// that satisfies the PiServiceClient interface requirements, with optional
// OpenTelemetry metrics and traces.
package server

import (
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	pi "github.com/memes/pi/v2"
	cachepkg "github.com/memes/pi/v2/pkg/cache"
	"github.com/memes/pi/v2/pkg/generated"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
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
	"google.golang.org/grpc/xds"
)

const (
	// The default name to use when using OpenTelemetry components.
	OpenTelemetryPackageIdentifier = "pkg.server"
)

var ErrIndexTooLarge = fmt.Errorf("index is too large, must be <= %d", math.MaxInt64)

type PiServer struct {
	generated.UnimplementedPiServiceServer
	// The logr.Logger implementation to use
	logger logr.Logger
	// An optional cache implementation
	cache cachepkg.Cache
	// Holds the instance specific metadata that will be returned in PiService responses
	metadata *generated.GetDigitMetadata
	// A gauge for calculation durations
	calculationMs metric.Int64Histogram
	// A counter for the number of errors returned by cache
	cacheErrors metric.Int64Counter
	// A counter for cache hits
	cacheHits metric.Int64Counter
	// A counter for cache misses
	cacheMisses metric.Int64Counter
	// A set of gRPC ServerOptions to use
	serverOptions []grpc.ServerOption
	// A set of gRPC DialOptions to use with REST gateway gRPC client
	dialOptions []grpc.DialOption
}

// Defines the function signature for PiServer options.
type PiServerOption func(*PiServer)

// Create a new piServer and apply any options.
func NewPiServer(options ...PiServerOption) (*PiServer, error) {
	var hostname string
	if host, err := os.Hostname(); err == nil {
		hostname = host
	} else {
		hostname = "unknown"
	}
	server := &PiServer{
		logger: logr.Discard(),
		cache:  cachepkg.NewNoopCache(),
		metadata: &generated.GetDigitMetadata{
			Identity:    hostname,
			Tags:        []string{},
			Annotations: map[string]string{},
		},
		serverOptions: []grpc.ServerOption{
			grpc.StatsHandler(otelgrpc.NewServerHandler()),
		},
		dialOptions: []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		},
	}
	for _, option := range options {
		option(server)
	}
	var err error
	server.calculationMs, err = otel.Meter(OpenTelemetryPackageIdentifier).Int64Histogram(
		OpenTelemetryPackageIdentifier+".calc_duration_ms",
		metric.WithUnit("ms"),
		metric.WithDescription("The duration (ms) of calculations"),
	)
	if err != nil {
		return nil, fmt.Errorf("error returned while creating calculationMs Histogram: %w", err)
	}
	server.cacheErrors, err = otel.Meter(OpenTelemetryPackageIdentifier).Int64Counter(
		OpenTelemetryPackageIdentifier+".cache_errors",
		metric.WithDescription("The count of error responses from digit cache"),
	)
	if err != nil {
		return nil, fmt.Errorf("error returned while creating cacheErrors Counter: %w", err)
	}
	server.cacheHits, err = otel.Meter(OpenTelemetryPackageIdentifier).Int64Counter(
		OpenTelemetryPackageIdentifier+".cache_hits",
		metric.WithDescription("The count of cache hits"),
	)
	if err != nil {
		return nil, fmt.Errorf("error returned while creating cacheHits Counter: %w", err)
	}
	server.cacheMisses, err = otel.Meter(OpenTelemetryPackageIdentifier).Int64Counter(
		OpenTelemetryPackageIdentifier+".cache_misses",
		metric.WithDescription("The count of cache misses"),
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

// Add the string tags to the server's metadata.
func WithTags(tags []string) PiServerOption {
	return func(s *PiServer) {
		if tags != nil {
			s.metadata.Tags = append(s.metadata.Tags, tags...)
		}
	}
}

// Add the key-value annotations to the server's metadata.
func WithAnnotations(annotations map[string]string) PiServerOption {
	return func(s *PiServer) {
		for k, v := range annotations {
			s.metadata.Annotations[k] = v
		}
	}
}

// Set the TransportCredentials to use for Pi Service gRPC listener.
func WithGRPCServerTransportCredentials(serverCredentials credentials.TransportCredentials) PiServerOption {
	return func(s *PiServer) {
		s.serverOptions = append(s.serverOptions, grpc.Creds(serverCredentials))
	}
}

// Set the TransportCredentials to use for Pi Service REST-to-gRPC client.
func WithRestClientGRPCTransportCredentials(restClientGRPCCredentials credentials.TransportCredentials) PiServerOption {
	return func(s *PiServer) {
		if restClientGRPCCredentials != nil {
			s.dialOptions = append(s.dialOptions, grpc.WithTransportCredentials(restClientGRPCCredentials))
		}
	}
}

// Set the authority string to use for REST-gRPC gateway calls.
func WithRestClientAuthority(restClientAuthority string) PiServerOption {
	return func(s *PiServer) {
		if restClientAuthority != "" {
			s.dialOptions = append(s.dialOptions, grpc.WithAuthority(restClientAuthority))
		}
	}
}

// Implement the PiService GetDigit RPC method.
//
//nolint:funlen // OTEL options make this function appear longer than expected.
func (s *PiServer) GetDigit(ctx context.Context, in *generated.GetDigitRequest) (*generated.GetDigitResponse, error) {
	logger := s.logger.WithValues("index", in.Index)
	logger.Info("GetDigit: enter")
	index := int64(in.Index) //nolint:gosec // Check for potential overflow happens later in function
	cacheIndex := (index / 9) * 9
	key := strconv.FormatInt(cacheIndex, 16)
	attributes := []attribute.KeyValue{
		attribute.Int64(OpenTelemetryPackageIdentifier+".index", index),
		attribute.String(OpenTelemetryPackageIdentifier+".cacheKey", key),
	}
	ctx, span := otel.Tracer(OpenTelemetryPackageIdentifier).Start(ctx, OpenTelemetryPackageIdentifier+"/GetDigit")
	defer span.End()
	span.SetAttributes(attributes...)
	if in.Index > math.MaxInt64 {
		span.RecordError(ErrIndexTooLarge)
		span.SetStatus(otelcodes.Error, ErrIndexTooLarge.Error())
		return nil, status.Error(codes.Internal, fmt.Sprintf("Index is too large: %v", ErrIndexTooLarge)) //nolint:wrapcheck // Errors returned should be gRPC statuses
	}
	span.AddEvent("Checking cache")
	digits, err := s.cache.GetValue(ctx, key)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		s.cacheErrors.Add(ctx, 1, metric.WithAttributes(attributes...))
		return nil, status.Error(codes.Internal, fmt.Sprintf("cache %T GetValue method returned an error: %v", s.cache, err)) //nolint:wrapcheck // Errors returned should be gRPC statuses
	}
	if digits == "" {
		attributes := append(attributes, attribute.Bool(OpenTelemetryPackageIdentifier+".cache_hit", false))
		span.SetAttributes(attributes...)
		span.AddEvent("Calculating fractional digits")
		s.cacheMisses.Add(ctx, 1, metric.WithAttributes(attributes...))
		ts := time.Now()
		digits = pi.BBPDigits(cacheIndex)
		s.calculationMs.Record(ctx, time.Since(ts).Milliseconds(), metric.WithAttributes(attributes...))
		err = s.cache.SetValue(ctx, key, digits)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, err.Error())
			s.cacheErrors.Add(ctx, 1, metric.WithAttributes(attributes...))
			return nil, status.Error(codes.Internal, fmt.Sprintf("cache %T SetValue method returned an error: %v", s.cache, err)) //nolint:wrapcheck // Errors returned should be gRPC statuses
		}
	} else {
		attributes := append(attributes, attribute.Bool(OpenTelemetryPackageIdentifier+".cache_hit", true))
		span.SetAttributes(attributes...)
		s.cacheHits.Add(ctx, 1, metric.WithAttributes(attributes...))
	}
	offset := index % 9
	digit, err := strconv.ParseUint(digits[offset:offset+1], 10, 32)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return nil, status.Error(codes.Internal, fmt.Sprintf("method GetDigit failed to parse as uint: %v", err)) //nolint:wrapcheck // Errors returned should be gRPC statuses
	}
	logger.Info("GetDigit: exit", "digit", digit)
	return &generated.GetDigitResponse{
		Index:    in.Index,
		Digit:    uint32(digit), //nolint:gosec // digit will always be between 0 and 9 inclusive, no risk of overflow
		Metadata: s.metadata,
	}, nil
}

// Create a new grpc.Server that is ready to be attached to a net.Listener.
func (s *PiServer) NewGrpcServer() *grpc.Server {
	s.logger.V(1).Info("Building a standard gRPC server")
	grpcServer := grpc.NewServer(s.serverOptions...)
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	generated.RegisterPiServiceServer(grpcServer, s)
	reflection.Register(grpcServer)
	return grpcServer
}

// Create a new xds.GRPCServer that is ready to be attached to a net.Listener.
func (s *PiServer) NewXDSServer() (*xds.GRPCServer, error) {
	s.logger.V(1).Info("xDS is enabled; building an xDS aware gRPC server")
	xdsServer, err := xds.NewGRPCServer(s.serverOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create new xDS gRPC server: %w", err)
	}
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(xdsServer, healthServer)
	reflection.Register(xdsServer)
	return xdsServer, nil
}

// Create a new REST gateway handler that translates and forwards incoming REST
// requests to the specified gRPC endpoint address.
func (s *PiServer) NewRestGatewayHandler(ctx context.Context, grpcAddress string) (http.Handler, error) {
	mux := runtime.NewServeMux()
	if err := generated.RegisterPiServiceHandlerFromEndpoint(ctx, mux, grpcAddress, s.dialOptions); err != nil {
		return nil, fmt.Errorf("failed to register PiService handler for REST gateway: %w", err)
	}
	if err := mux.HandlePath("GET", "/api/v2/swagger.json",
		func(w http.ResponseWriter, _ *http.Request, _ map[string]string) {
			w.Header().Add("Content-Type", "application/json")
			if _, err := w.Write(generated.SwaggerJSON); err != nil {
				s.logger.Error(err, "Writing swagger JSON to response raised an error; continuing")
			}
		},
	); err != nil {
		return nil, fmt.Errorf("failed to register /api/v2 handler for swagger definition: %w", err)
	}
	if err := mux.HandlePath("GET", "/v1/digit/{index}",
		func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
			span := trace.SpanFromContext(r.Context())
			span.SetAttributes(attribute.String(OpenTelemetryPackageIdentifier+".index", pathParams["index"]))
			span.SetStatus(otelcodes.Error, "v1 API")
			w.WriteHeader(http.StatusGone)
		},
	); err != nil {
		return nil, fmt.Errorf("failed to register /v1 handler for REST gateway: %w", err)
	}
	return otelhttp.NewHandler(mux,
		OpenTelemetryPackageIdentifier+"/RestGatewayHandler",
		otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents),
	), nil
}
