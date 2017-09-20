package cmd

import (
	"log"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	APP_NAME = "pi"
)

var (
	Logger  *zap.Logger
	verbose bool
	cfgFile string
	RootCmd = &cobra.Command{
		Use:   APP_NAME,
		Short: "Utility to get the digits of pi at an arbitrary index",
		Long:  "Pi is a client/server application that will fetch 9 digits of pi from an arbitrary index.",
	}
)

func init() {
	cobra.OnInitialize(initConfig)
	RootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	viper.BindPFlag("verbose", RootCmd.PersistentFlags().Lookup("verbose"))
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
		Logger, _ = zap.NewDevelopment()
	} else {
		Logger, _ = zap.NewProduction()
	}
	if Logger == nil {
		log.Fatal("Error creating logger", err)
	}
	if err == nil {
		return
	}
	switch t := err.(type) {
	case viper.ConfigFileNotFoundError:
		Logger.Debug("Error reading configuration file",
			zap.Error(t),
		)

	default:
		Logger.Error("Error reading configuration file",
			zap.Error(t),
		)
	}
}
