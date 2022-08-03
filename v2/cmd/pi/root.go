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
	VerboseFlagName                    = "verbose"
	StructuredLoggingFlagName          = "structured-logging"
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
	version = "v2-snapshot"
	// Failed to load CA cert.
	errFailedToAppendCACert = errors.New("failed to append CA cert to CA pool")
)

func NewRootCmd() (*cobra.Command, error) {
	cobra.OnInitialize(initConfig)
	rootCmd := &cobra.Command{
		Use:     AppName,
		Version: version,
		Short:   "Calculate and retrieve a fractional digit of pi at an arbitrary index",
		Long:    `Provides a gRPC client/server demo for distributed calculation of fractional digits of pi.`,
	}
	rootCmd.PersistentFlags().CountP(VerboseFlagName, "v", "Enable verbose logging; can be repeated to increase verbosity")
	rootCmd.PersistentFlags().Bool(StructuredLoggingFlagName, true, "Format logs as structured JSON records; set to false to output text logs")
	rootCmd.PersistentFlags().String(OpenTelemetryTargetFlagName, "", "An optional OpenTelemetry collection target that will receive metrics and traces")
	rootCmd.PersistentFlags().Bool(OpenTelemetryInsecureFlagName, false, "Disable remote TLS verification for OpenTelemetry target")
	rootCmd.PersistentFlags().String(OpenTelemetryAuthorityFlagName, "", "Set the authoritative name of the OpenTelemetry target for TLS verification, overriding hostname")
	rootCmd.PersistentFlags().Float64(OpenTelemetrySamplingRatioFlagName, DefaultOTLPTraceSamplingRatio, "Set the OpenTelemetry trace sampling ratio")
	rootCmd.PersistentFlags().StringArray(CACertFlagName, nil, "An optional CA certificate to use for TLS certificate verification; can be repeated")
	rootCmd.PersistentFlags().StringP(TLSCertFlagName, "E", "", "An optional TLS certificate to use")
	rootCmd.PersistentFlags().String(TLSKeyFlagName, "", "An optional TLS private key to use")
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
