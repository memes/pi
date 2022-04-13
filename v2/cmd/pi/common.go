package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/viper"
	gcpdetectors "go.opentelemetry.io/contrib/detectors/gcp"
	hostinstrumentation "go.opentelemetry.io/contrib/instrumentation/host"
	runtimeinstrumentation "go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/propagation"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"google.golang.org/grpc/credentials"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding/gzip"
)

const (
	metricReportingPeriod = 30 * time.Second
)

type shutdownFunction func(context.Context) error

func noopShutdownFunction(_ context.Context) error {
	return nil
}

// Create a new OpenTelemetry resource to describe the source of metrics and traces.
func newTelemetryResource(ctx context.Context, name string) (*resource.Resource, error) {
	logger := logger.V(1).WithValues("name", name)
	logger.Info("Creating new OpenTelemetry resource descriptor")
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("failed to generate UUID for telemetry resource: %w", err)
	}
	res, err := resource.New(ctx,
		resource.WithSchemaURL(semconv.SchemaURL),
		resource.WithAttributes(
			semconv.ServiceNamespaceKey.String(PackageName),
			semconv.ServiceNameKey.String(name),
			semconv.ServiceVersionKey.String(version),
			semconv.ServiceInstanceIDKey.String(id.String()),
		),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithOS(),
		// Some process information is unknown when running in a scratch
		// container. See https://github.com/memes/pi/issues/3
		resource.WithProcessPID(),
		resource.WithProcessExecutableName(),
		resource.WithProcessExecutablePath(),
		resource.WithProcessCommandArgs(),
		resource.WithProcessRuntimeName(),
		resource.WithProcessRuntimeVersion(),
		resource.WithProcessRuntimeDescription(),
		// These detectors place last to override the base service attributes with specifiers from GCP
		resource.WithDetectors(
			&gcpdetectors.GCE{},
			&gcpdetectors.GKE{},
			gcpdetectors.NewCloudRun(),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create new telemetry resource: %w", err)
	}
	logger.V(1).Info("OpenTelemetry resource created", "resource", res)
	return res, nil
}

// Initializes a pusher that will send OpenTelemetry metrics to the target
// provided, returning a shutdown function.
func initMetrics(ctx context.Context, target string, creds credentials.TransportCredentials, res *resource.Resource) ([]shutdownFunction, error) {
	logger := logger.V(1).WithValues("target", target, "creds", creds, "res", res)
	logger.V(1).Info("Creating OpenTelemetry metric handlers")
	if target == "" {
		logger.V(0).Info("OpenTelemetry endpoint is not set; no metrics will be sent to collector")
		return []shutdownFunction{
			noopShutdownFunction,
		}, nil
	}
	options := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(target),
		otlpmetricgrpc.WithCompressor(gzip.Name),
	}
	if creds != nil {
		options = append(options, otlpmetricgrpc.WithTLSCredentials(creds))
	}
	client := otlpmetricgrpc.NewClient(options...)
	exporter, err := otlpmetric.New(ctx, client)
	if err != nil {
		return []shutdownFunction{
			noopShutdownFunction,
		}, fmt.Errorf("failed to create new metric exporter: %w", err)
	}
	shutdownFuncs := []shutdownFunction{
		func(ctx context.Context) error {
			if err := exporter.Shutdown(ctx); err != nil {
				return fmt.Errorf("error during OpenTelemetry metric exporter shutdown: %w", err)
			}
			return nil
		},
	}
	pusher := controller.New(
		processor.NewFactory(simple.NewWithHistogramDistribution(), exporter),
		controller.WithExporter(exporter),
		controller.WithResource(res),
		controller.WithCollectPeriod(metricReportingPeriod),
	)
	if err = pusher.Start(ctx); err != nil {
		return shutdownFuncs, fmt.Errorf("failed to start metric pusher: %w", err)
	}
	shutdownFuncs = append([]shutdownFunction{
		func(ctx context.Context) error {
			if err := pusher.Stop(ctx); err != nil {
				return fmt.Errorf("error during OpenTelemetry metric pusher shutdown: %w", err)
			}
			return nil
		},
	}, shutdownFuncs...)
	if err = runtimeinstrumentation.Start(runtimeinstrumentation.WithMeterProvider(pusher)); err != nil {
		return shutdownFuncs, fmt.Errorf("failed to start runtime metrics: %w", err)
	}
	if err = hostinstrumentation.Start(hostinstrumentation.WithMeterProvider(pusher)); err != nil {
		return shutdownFuncs, fmt.Errorf("failed to start host metrics: %w", err)
	}

	global.SetMeterProvider(pusher)
	logger.V(1).Info("OpenTelemetry metric handlers created and started")
	return shutdownFuncs, nil
}

// Initializes a pipeline handler that will send OpenTelemetry spans to the target
// provided, returning a shutdown function.
func initTrace(ctx context.Context, target string, creds credentials.TransportCredentials, res *resource.Resource, sampler trace.Sampler) ([]shutdownFunction, error) {
	logger := logger.V(1).WithValues("target", target, "creds", creds, "res", res, "sampler", sampler.Description())
	logger.V(1).Info("Creating new OpenTelemetry trace exporter")
	if target == "" {
		logger.V(0).Info("OpenTelemetry endpoint is not set; no traces will be sent to collector")
		return []shutdownFunction{
			noopShutdownFunction,
		}, nil
	}
	options := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(target),
		otlptracegrpc.WithCompressor(gzip.Name),
	}
	if creds != nil {
		options = append(options, otlptracegrpc.WithTLSCredentials(creds))
	}
	exporter, err := otlptrace.New(ctx, otlptracegrpc.NewClient(options...))
	if err != nil {
		return nil, fmt.Errorf("failed to create new trace exporter: %w", err)
	}
	shutdownFuncs := []shutdownFunction{
		func(ctx context.Context) error {
			if err := exporter.Shutdown(ctx); err != nil {
				return fmt.Errorf("error during OpenTelemetry trace exporter shutdown: %w", err)
			}
			return nil
		},
	}

	// NOTE: provider.Shutdown will shutdown every registered span processor
	// so don't add an explicit shutdown function.
	spanProcessor := trace.NewBatchSpanProcessor(exporter)

	provider := trace.NewTracerProvider(
		trace.WithSampler(sampler),
		trace.WithSpanProcessor(spanProcessor),
		trace.WithResource(res),
	)
	shutdownFuncs = append([]shutdownFunction{
		func(ctx context.Context) error {
			if err := provider.Shutdown(ctx); err != nil {
				return fmt.Errorf("error during OpenTelemetry trace provider shutdown: %w", err)
			}
			return nil
		},
	}, shutdownFuncs...)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	otel.SetTracerProvider(provider)
	logger.V(1).Info("OpenTelemetry trace handlers created and started")
	return shutdownFuncs, nil
}

// Initializes OpenTelemetry metric and trace processing and deliver to a collector
// target, returning a list of function that can be called to shutdown the background
// pipeline processes.
func initTelemetry(ctx context.Context, name string, sampler trace.Sampler) ([]shutdownFunction, error) {
	otel.SetLogger(logger)
	target := viper.GetString(OpenTelemetryTargetFlagName)
	cacerts := viper.GetStringSlice(CACertFlagName)
	cert := viper.GetString(TLSCertFlagName)
	key := viper.GetString(TLSKeyFlagName)
	insecure := viper.GetBool(InsecureFlagName)
	logger := logger.V(1).WithValues(
		"name", name,
		"target", target,
		"cacerts", cacerts,
		"cert", cert,
		"key", key,
		"insecure", insecure,
		"sampler", sampler.Description(),
	)
	logger.Info("Initializing OpenTelemetry")
	shutdownFunctions := []shutdownFunction{}

	res, err := newTelemetryResource(ctx, name)
	if err != nil {
		return nil, err
	}

	var creds credentials.TransportCredentials
	if insecure {
		creds = grpcinsecure.NewCredentials()
	} else {
		certPool, err := newCACertPool(cacerts)
		if err != nil {
			return nil, err
		}
		tlsConfig, err := newTLSConfig(cert, key, nil, certPool)
		if err != nil {
			return nil, err
		}
		creds = credentials.NewTLS(tlsConfig)
	}

	shutdownMetrics, err := initMetrics(ctx, target, creds, res)
	if err != nil {
		return shutdownMetrics, err
	}
	shutdownFunctions = append(shutdownMetrics, shutdownFunctions...)
	shutdownTraces, err := initTrace(ctx, target, creds, res, sampler)
	shutdownFunctions = append(shutdownTraces, shutdownFunctions...)
	if err != nil {
		return shutdownFunctions, err
	}
	logger.Info("OpenTelemetry initialization complete, returning shutdown functions")
	return shutdownFunctions, nil
}

// Creates a new pool of x509 certificates from the list of file paths provided,
// appended to any system installed certificates.
func newCACertPool(cacerts []string) (*x509.CertPool, error) {
	logger := logger.V(1).WithValues("cacerts", cacerts)
	if len(cacerts) == 0 {
		logger.V(0).Info("No CA certificate paths provided; returning nil for CA cert pool")
		return nil, nil
	}
	logger.V(0).Info("Building certificate pool from file(s)")
	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("failed to build new CA cert pool from SystemCertPool: %w", err)
	}
	for _, cacert := range cacerts {
		ca, err := ioutil.ReadFile(cacert)
		if err != nil {
			return nil, fmt.Errorf("failed to read from certificate file %s: %w", cacert, err)
		}
		if ok := pool.AppendCertsFromPEM(ca); !ok {
			return nil, fmt.Errorf("failed to process CA cert %s: %w", cacert, errFailedToAppendCACert)
		}
	}
	return pool, nil
}

// Creates a new TLS configuration from supplied arguments. If a certificate and
// key are provided, the loaded x509 certificate will be added as the certificate
// to present to remote side of TLS connections. An optional pool of CA certificates
// can be provided as ClientCA and/or RootCA verification.
func newTLSConfig(certFile, keyFile string, clientCAs, rootCAs *x509.CertPool) (*tls.Config, error) {
	logger := logger.V(1).WithValues(TLSCertFlagName, certFile, TLSKeyFlagName, keyFile, "hasClientCAs", clientCAs != nil, "hasRootCAs", rootCAs != nil)
	logger.V(0).Info("Preparing TLS configuration")
	tlsConf := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	if certFile != "" && keyFile != "" {
		logger.V(1).Info("Loading x509 certificate and key")
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load certificate %s and key %s: %w", certFile, keyFile, err)
		}
		tlsConf.Certificates = []tls.Certificate{cert}
	}
	if clientCAs != nil {
		logger.V(1).Info("Add x509 certificate pool to ClientCAs")
		tlsConf.ClientCAs = clientCAs
	}
	if rootCAs != nil {
		logger.V(1).Info("Add x509 certificate pool to RootCAs")
		tlsConf.RootCAs = rootCAs
	}
	return tlsConf, nil
}
