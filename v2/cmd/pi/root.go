package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/go-logr/zerologr"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	AppName                            = "pi"
	PackageName                        = "github.com/memes/pi/v2/cmd/pi"
	DefaultOTLPTraceSamplingRatio      = 0.5
	VerboseFlagName                    = "verbose"
	PrettyFlagName                     = "pretty"
	OpenTelemetryTargetFlagName        = "otlp-target"
	OpenTelemetryInsecureFlagName      = "otlp-insecure"
	OpenTelemetryAuthorityFlagName     = "otlp-authority"
	OpenTelemetrySamplingRatioFlagName = "otlp-sampling-ratio"
	CACertFlagName                     = "cacert"
	TLSCertFlagName                    = "cert"
	TLSKeyFlagName                     = "key"
)

var (
	// Version is updated from git tags during build.
	version = "unspecified"
	// Failed to load CA cert.
	errFailedToAppendCACert = errors.New("failed to append CA cert to CA pool")
)

func NewRootCmd() (*cobra.Command, error) {
	cobra.OnInitialize(initConfig)
	rootCmd := &cobra.Command{
		Use:     AppName,
		Version: version,
		Short:   "Get a fractional digit of pi at an arbitrary index",
		Long:    `Provides a gRPC client/server demo for distributed calculation of fractional digits of pi.`,
	}
	rootCmd.PersistentFlags().CountP(VerboseFlagName, "v", "Enable verbose logging; can be repeated to increase verbosity")
	rootCmd.PersistentFlags().BoolP(PrettyFlagName, "p", false, "Disables structured JSON logging to stdout, making it easier to read")
	rootCmd.PersistentFlags().String(OpenTelemetryTargetFlagName, "", "An optional OpenTelemetry collection target that will receive metrics and traces")
	rootCmd.PersistentFlags().Bool(OpenTelemetryInsecureFlagName, false, "Disable remote TLS verification for OpenTelemetry target")
	rootCmd.PersistentFlags().String(OpenTelemetryAuthorityFlagName, "", "Set the authoritative name of the OpenTelemetry target for TLS verification, overriding hostname")
	rootCmd.PersistentFlags().Float64(OpenTelemetrySamplingRatioFlagName, DefaultOTLPTraceSamplingRatio, "Set the OpenTelemetry trace sampling ratio")
	rootCmd.PersistentFlags().StringArray(CACertFlagName, nil, "An optional CA certificate to use for remote TLS verification; can be repeated")
	rootCmd.PersistentFlags().String(TLSCertFlagName, "", "An optional TLS certificate to use")
	rootCmd.PersistentFlags().String(TLSKeyFlagName, "", "An optional TLS private key to use")
	if err := viper.BindPFlag(VerboseFlagName, rootCmd.PersistentFlags().Lookup(VerboseFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind verbose pflag: %w", err)
	}
	if err := viper.BindPFlag(PrettyFlagName, rootCmd.PersistentFlags().Lookup(PrettyFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind pretty pflag: %w", err)
	}
	if err := viper.BindPFlag(OpenTelemetryTargetFlagName, rootCmd.PersistentFlags().Lookup(OpenTelemetryTargetFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind otlp-target pflag: %w", err)
	}
	if err := viper.BindPFlag(OpenTelemetryInsecureFlagName, rootCmd.PersistentFlags().Lookup(OpenTelemetryInsecureFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind otlp-insecure pflag: %w", err)
	}
	if err := viper.BindPFlag(OpenTelemetryAuthorityFlagName, rootCmd.PersistentFlags().Lookup(OpenTelemetryAuthorityFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind otlp-authority pflag: %w", err)
	}
	if err := viper.BindPFlag(OpenTelemetrySamplingRatioFlagName, rootCmd.PersistentFlags().Lookup(OpenTelemetrySamplingRatioFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind otlp-sampling-ratio pflag: %w", err)
	}
	if err := viper.BindPFlag(CACertFlagName, rootCmd.PersistentFlags().Lookup(CACertFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind cacert pflag: %w", err)
	}
	if err := viper.BindPFlag(TLSCertFlagName, rootCmd.PersistentFlags().Lookup(TLSCertFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind cert pflag: %w", err)
	}
	if err := viper.BindPFlag(TLSKeyFlagName, rootCmd.PersistentFlags().Lookup(TLSKeyFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind key pflag: %w", err)
	}
	serverCmd, err := NewServerCmd()
	if err != nil {
		return nil, err
	}
	clientCmd, err := NewClientCmd()
	if err != nil {
		return nil, err
	}
	rootCmd.AddCommand(serverCmd, clientCmd)
	return rootCmd, nil
}

// Determine the outcome of command line flags, environment variables, and an
// optional configuration file to perform initialization of the application. An
// appropriate zerolog will be assigned as the default logr sink.
func initConfig() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	zl := zerolog.New(os.Stderr).With().Caller().Timestamp().Logger()
	viper.AddConfigPath(".")
	if home, err := homedir.Dir(); err == nil {
		viper.AddConfigPath(home)
	}
	viper.SetConfigName("." + AppName)
	viper.SetEnvPrefix(AppName)
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
	err := viper.ReadInConfig()
	verbosity := viper.GetInt(VerboseFlagName)
	switch {
	case verbosity > 2:
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	case verbosity == 2:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case verbosity == 1:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	}
	if viper.GetBool(PrettyFlagName) {
		zl = zl.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	}
	logger = zerologr.New(&zl)
	if err == nil {
		return
	}
	var cfgNotFound viper.ConfigFileNotFoundError
	if !errors.As(err, &cfgNotFound) {
		logger.Error(err, "Error reading configuration file")
	}
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
