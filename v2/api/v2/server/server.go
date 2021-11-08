// Package server implements a gRPC server (and optional REST gateway) implementation
// that satisfies the PiServiceClient interface requirements, with optional
// OpenTelemetry metrics and traces.
package server

import (
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	pi "github.com/memes/pi/v2"
	api "github.com/memes/pi/v2/api/v2"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

type PiServer struct {
	api.UnimplementedPiServiceServer
	// The logr.Logger implementation to use
	logger logr.Logger
	// An instance of pi.Calculator that will be used to calculate fractional digits
	calculator *pi.Calculator
	// An optional cache implementation
	cache Cache
	// Holds the instance specific metadata that will be returned in PiService responses
	metadata *api.GetDigitMetadata
	// The OpenTelemetry tracer to use for spans
	tracer trace.Tracer
	// The OpenTelemetry meter to use for metrics
	meter metric.Meter
	// A counter for the number of errors returned by calculations
	calculationErrors metric.Int64Counter
	// A gauge for calculation durations
	calculationMs metric.Float64Histogram
}

// Defines the function signature for PiServer options.
type PiServerOption func(*PiServer)

// Create a new piServer and apply any options
func NewPiServer(options ...PiServerOption) *PiServer {
	server := &PiServer{
		logger:     logr.Discard(),
		calculator: pi.NewCalculator(),
		cache:      NewNoopCache(),
		tracer:     otel.Tracer(""),
	}
	WithMeter("server", global.Meter(""))(server)
	for _, option := range options {
		option(server)
	}
	return server
}

// Use the supplied logger for the server and calculator packages.
func WithLogger(logger logr.Logger) PiServerOption {
	return func(s *PiServer) {
		s.logger = logger
		pi.WithLogger(logger)(s.calculator)
	}
}

// Use the cache implementation to store BBPDigits results to avoid recalculation
// of a digit that has already been calculated.
func WithCache(cache Cache) PiServerOption {
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
				if net, ok := addr.(*net.IPNet); ok && net.IP.IsGlobalUnicast() {
					addresses = append(addresses, net.IP.String())
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
func WithMeter(prefix string, meter metric.Meter) PiServerOption {
	return func(s *PiServer) {
		if (prefix != "" && meter != metric.Meter{}) {
			s.meter = meter
			s.calculationErrors = metric.Must(meter).NewInt64Counter(prefix+"/calc_errors", metric.WithDescription("The count of calculation errors"))
			s.calculationMs = metric.Must(meter).NewFloat64Histogram(prefix+"/calc_duration_ms", metric.WithDescription("The duration (ms) of calculations"))
		}
	}
}

// Implement the PiService GetDigit RPC method
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
	cacheIndex := uint64(in.Index/9) * 9
	key := strconv.FormatUint(cacheIndex, 16)
	span.AddEvent("Checking cache")
	digits, err := s.cache.GetValue(ctx, key)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		s.meter.RecordBatch(
			ctx,
			attributes,
			s.calculationErrors.Measurement(1),
		)
		return nil, err
	}
	if digits == "" {
		attributes := append(attributes, attribute.Bool("cache_hit", false))
		span.SetAttributes(attributes...)
		ts := time.Now()
		digits = s.calculator.BBPDigits(cacheIndex)
		duration = float64(time.Since(ts) / time.Millisecond)
		err = s.cache.SetValue(ctx, key, digits)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, err.Error())
			s.meter.RecordBatch(
				ctx,
				attributes,
				s.calculationErrors.Measurement(1),
				s.calculationMs.Measurement(duration),
			)
			return nil, err
		}
	} else {
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
			s.calculationErrors.Measurement(1),
			s.calculationMs.Measurement(duration),
		)
		return nil, err
	}
	logger.Info("GetDigit: exit", "digit", digit)
	s.meter.RecordBatch(
		ctx,
		attributes,
		s.calculationMs.Measurement(duration),
	)
	return &api.GetDigitResponse{
		Index:    in.Index,
		Digit:    uint32(digit),
		Metadata: s.metadata,
	}, nil
}

// Implement the gRPC health service Check method
func (s *PiServer) Check(ctx context.Context, in *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
}

// Satisfy the gRPC health service Watch method - always returns an Unimplemented error
func (s *PiServer) Watch(in *grpc_health_v1.HealthCheckRequest, _ grpc_health_v1.Health_WatchServer) error {
	return status.Error(codes.Unimplemented, "unimplemented")
}

// Create a new grpc.Server that is ready to be attached to a net.Listener.
func (s *PiServer) NewGrpcServer() *grpc.Server {
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
	)
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
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
	}
	if err := api.RegisterPiServiceHandlerFromEndpoint(ctx, mux, grpcAddress, opts); err != nil {
		return nil, err
	}
	if err := mux.HandlePath("GET", "/v1/digit/{index}", func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		span := trace.SpanFromContext(r.Context())
		span.SetAttributes(attribute.String("index", pathParams["index"]))
		span.SetStatus(otelcodes.Error, "v1 API")
		w.WriteHeader(http.StatusGone)
	}); err != nil {
		return nil, err
	}
	return otelhttp.NewHandler(mux, "rest-gateway", otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents)), nil
}
