package main

import (
	"go.uber.org/zap"
)

func main() {
	defer func() {
		if logger != nil {
			_ = logger.Sync()
		}
	}()
	if err := rootCmd.Execute(); err != nil {
		logger.Fatal("Error executing command",
			zap.Error(err),
		)
	}
}
