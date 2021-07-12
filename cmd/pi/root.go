package main

import (
	"log"
	// spell-checker: ignore mitchellh
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	APP_NAME = "pi"
)

var (
	logger  *zap.Logger = zap.NewNop()
	verbose bool
	cfgFile string
	rootCmd = &cobra.Command{
		Use:   APP_NAME,
		Short: "Utility to get the digits of pi at an arbitrary index",
		Long:  "Pi is a client/server application that will fetch 9 digits of pi from an arbitrary index.",
	}
)

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := homedir.Dir()
		if err != nil {
			log.Fatal("Error locating home dir", err)
		}
		viper.AddConfigPath(home)
		viper.SetConfigName("." + APP_NAME)
	}
	viper.SetEnvPrefix(APP_NAME)
	viper.AutomaticEnv()
	err := viper.ReadInConfig()
	if verbose {
		logger, _ = zap.NewDevelopment()
	} else {
		logger, _ = zap.NewProduction()
	}
	if logger == nil {
		log.Fatal("Error creating logger", err)
	}
	if err == nil {
		return
	}
	switch t := err.(type) {
	case viper.ConfigFileNotFoundError:
		logger.Debug("Error reading configuration file",
			zap.Error(t),
		)

	default:
		logger.Error("Error reading configuration file",
			zap.Error(t),
		)
	}
}
