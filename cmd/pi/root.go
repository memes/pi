package main

import (
	"log"
	"os"
	"strings"

	// spell-checker: ignore mitchellh
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	APP_NAME = "pi"
)

var (
	logger  *zap.Logger = zap.NewNop()
	rootCmd             = &cobra.Command{
		Use:   APP_NAME,
		Short: "Utility to get the digits of pi at an arbitrary index",
		Long:  "Pi is a client/server application that will fetch 9 digits of pi from an arbitrary index.",
	}
)

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose logging")
	rootCmd.PersistentFlags().BoolP("quiet", "q", false, "Enable quiet logging")
	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	_ = viper.BindPFlag("quiet", rootCmd.PersistentFlags().Lookup("quiet"))
}

func initConfig() {
	viper.AddConfigPath(".")
	home, err := homedir.Dir()
	if err != nil {
		log.Printf("Error locating home dir: %+v", err)
	} else {
		viper.AddConfigPath(home)
	}
	viper.SetConfigName("." + APP_NAME)
	viper.SetEnvPrefix(APP_NAME)
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
	err = viper.ReadInConfig()
	config := zap.NewProductionEncoderConfig()
	encoder := zapcore.NewJSONEncoder(config)
	level := zap.NewAtomicLevel()
	logger = zap.New(zapcore.NewCore(encoder, zapcore.Lock(os.Stdout), level))
	if viper.GetBool("verbose") {
		level.SetLevel(zapcore.DebugLevel)
	}
	if viper.GetBool("quiet") {
		level.SetLevel(zapcore.ErrorLevel)
	}
	if logger == nil {
		log.Fatal("Error creating logger", err)
	}
	if err == nil {
		return
	}
	switch t := err.(type) {
	case viper.ConfigFileNotFoundError:
		logger.Debug("Configuration file not found",
			zap.Error(t),
		)

	default:
		logger.Error("Error reading configuration file",
			zap.Error(t),
		)
	}
}
