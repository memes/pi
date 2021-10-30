package main

import (
	"github.com/go-logr/logr"
)

var (
	logger = logr.Discard()
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		logger.Error(err, "Error executing command")
	}
}
