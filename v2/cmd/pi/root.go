package main

import (
	"errors"
	"fmt"
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
	DefaultOTLPTraceSamplingRatio      = 0.5
	AnnotationFlagName                 = "annotation"
	AuthorityFlagName                  = "authority"
	CACertFlagName                     = "cacert"
	CountFlagName                      = "count"
	HeaderFlagName                     = "header"
	InsecureFlagName                   = "insecure"
	MaxTimeoutFlagName                 = "max-timeout"
	MutualTLSFlagName                  = "mtls"
	OpenTelemetryAuthorityFlagName     = "otlp-authority"
	OpenTelemetryInsecureFlagName      = "otlp-insecure"
	OpenTelemetrySamplingRatioFlagName = "otlp-sampling-ratio"
	OpenTelemetryTargetFlagName        = "otlp-target"
	OpenTelemetryTLSCertFlagName       = "otlp-cert"
	OpenTelemetryTLSKeyFlagName        = "otlp-key"
	RedisTargetFlagName                = "redis-target"
	RESTAddressFlagName                = "rest-address"
	RESTAuthorityFlagName              = "rest-authority"
	StructuredLoggingFlagName          = "structured-logging"
	TagFlagName                        = "tag"
	TLSCertFlagName                    = "cert"
	TLSKeyFlagName                     = "key"
	VerboseFlagName                    = "verbose"
	XDSFlagName                        = "xds"
)

// Version is updated from git tags during build.
var version = "v2-snapshot"

func NewRootCmd() (*cobra.Command, error) {
	cobra.OnInitialize(initConfig)
	rootCmd := &cobra.Command{
		Use:     AppName,
		Version: version,
		Short:   "Calculate and retrieve a fractional digit of pi at an arbitrary index",
		Long:    `Provides a gRPC client/server demo for distributed calculation of fractional digits of pi.`,
	}
	rootCmd.PersistentFlags().Count(VerboseFlagName, "Enable verbose logging; can be repeated to increase verbosity")
	rootCmd.PersistentFlags().Bool(StructuredLoggingFlagName, true, "Format logs as structured JSON records; set to false to output text logs")
	rootCmd.PersistentFlags().String(OpenTelemetryTargetFlagName, "", "An optional OpenTelemetry collection target that will receive metrics and traces")
	rootCmd.PersistentFlags().Bool(OpenTelemetryInsecureFlagName, false, "Disable remote TLS verification for OpenTelemetry target")
	rootCmd.PersistentFlags().String(OpenTelemetryAuthorityFlagName, "", "Set the authoritative name of the OpenTelemetry target for TLS verification, overriding hostname")
	rootCmd.PersistentFlags().Float64(OpenTelemetrySamplingRatioFlagName, DefaultOTLPTraceSamplingRatio, "Set the OpenTelemetry trace sampling ratio")
	rootCmd.PersistentFlags().StringArray(CACertFlagName, nil, "An optional CA certificate to use for TLS certificate verification; can be repeated")
	rootCmd.PersistentFlags().String(TLSCertFlagName, "", "An optional TLS certificate to secure this communication")
	rootCmd.PersistentFlags().String(TLSKeyFlagName, "", "An optional private key to use with TLS certificate")
	rootCmd.PersistentFlags().String(OpenTelemetryTLSCertFlagName, "", "An optional TLS certificate to use with OpenTelemetry gRPC collector")
	rootCmd.PersistentFlags().String(OpenTelemetryTLSKeyFlagName, "", "An optional private key to use with OpenTelemetry TLS certificate")
	if err := viper.BindPFlag(VerboseFlagName, rootCmd.PersistentFlags().Lookup(VerboseFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind %s pflag: %w", VerboseFlagName, err)
	}
	if err := viper.BindPFlag(StructuredLoggingFlagName, rootCmd.PersistentFlags().Lookup(StructuredLoggingFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind %s pflag: %w", StructuredLoggingFlagName, err)
	}
	if err := viper.BindPFlag(OpenTelemetryTargetFlagName, rootCmd.PersistentFlags().Lookup(OpenTelemetryTargetFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind %s pflag: %w", OpenTelemetryTargetFlagName, err)
	}
	if err := viper.BindPFlag(OpenTelemetryInsecureFlagName, rootCmd.PersistentFlags().Lookup(OpenTelemetryInsecureFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind %s pflag: %w", OpenTelemetryInsecureFlagName, err)
	}
	if err := viper.BindPFlag(OpenTelemetryAuthorityFlagName, rootCmd.PersistentFlags().Lookup(OpenTelemetryAuthorityFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind %s pflag: %w", OpenTelemetryAuthorityFlagName, err)
	}
	if err := viper.BindPFlag(OpenTelemetrySamplingRatioFlagName, rootCmd.PersistentFlags().Lookup(OpenTelemetrySamplingRatioFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind %s pflag: %w", OpenTelemetrySamplingRatioFlagName, err)
	}
	if err := viper.BindPFlag(CACertFlagName, rootCmd.PersistentFlags().Lookup(CACertFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind %s pflag: %w", CACertFlagName, err)
	}
	if err := viper.BindPFlag(TLSCertFlagName, rootCmd.PersistentFlags().Lookup(TLSCertFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind %s pflag: %w", TLSCertFlagName, err)
	}
	if err := viper.BindPFlag(TLSKeyFlagName, rootCmd.PersistentFlags().Lookup(TLSKeyFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind %s pflag: %w", TLSKeyFlagName, err)
	}
	if err := viper.BindPFlag(OpenTelemetryTLSCertFlagName, rootCmd.PersistentFlags().Lookup(OpenTelemetryTLSCertFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind %s pflag: %w", OpenTelemetryTLSCertFlagName, err)
	}
	if err := viper.BindPFlag(OpenTelemetryTLSKeyFlagName, rootCmd.PersistentFlags().Lookup(OpenTelemetryTLSKeyFlagName)); err != nil {
		return nil, fmt.Errorf("failed to bind %s pflag: %w", OpenTelemetryTLSKeyFlagName, err)
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
	if !viper.GetBool(StructuredLoggingFlagName) {
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
