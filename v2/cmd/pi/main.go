// This package contains a gRPC client and server to demonstrate use of library
// in a distributed environment.
package main

import (
	"os"

	"github.com/go-logr/logr"
)

// The default logr sink; this will be changed as command options are processed.
var logger = logr.Discard()

func main() {
	rootCmd, err := NewRootCmd()
	if err != nil {
		logger.Error(err, "Error building commands")
		os.Exit(1)
	}
	if err := rootCmd.Execute(); err != nil {
		logger.Error(err, "Error executing command")
	}
}
