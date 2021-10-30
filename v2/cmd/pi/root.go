package main

import (
	"os"
	"strings"

	"github.com/go-logr/zerologr"
	// spell-checker: ignore mitchellh
	homedir "github.com/mitchellh/go-homedir"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	APP_NAME = "pi"
)

var (
	version = "unknown"
	rootCmd = &cobra.Command{
		Use:     APP_NAME,
		Version: version,
		Short:   "Utility to get the digit of pi at an arbitrary index",
		Long:    "Pi is a client/server application that will fetch one of the decimal digits of pi from an arbitrary index.",
	}
)

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().CountP("verbose", "v", "Enable more verbose logging")
	rootCmd.PersistentFlags().BoolP("pretty", "p", false, "Enable pretty console logging")
	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	_ = viper.BindPFlag("pretty", rootCmd.PersistentFlags().Lookup("pretty"))
}

func initConfig() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	zl := zerolog.New(os.Stderr).With().Caller().Timestamp().Logger()
	viper.AddConfigPath(".")
	if home, err := homedir.Dir(); err == nil {
		viper.AddConfigPath(home)
	}
	viper.SetConfigName("." + APP_NAME)
	viper.SetEnvPrefix(APP_NAME)
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
	err := viper.ReadInConfig()
	verbosity := viper.GetInt("verbose")
	switch {
	case verbosity >= 2:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)

	case verbosity == 1:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	}
	if viper.GetBool("pretty") {
		zl = zl.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	}
	logger = zerologr.New(&zl)
	if err == nil {
		return
	}
	switch t := err.(type) {
	case viper.ConfigFileNotFoundError:
		logger.V(1).Info("Configuration file not found", "err", t)

	default:
		logger.Error(t, "Error reading configuration file")
	}
}
