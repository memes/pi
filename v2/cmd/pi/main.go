// This package contains a gRPC client and server to demonstrate use of library
// in a distributed environment.
package main

import (
	"github.com/go-logr/logr"
)

var (
	// The default logr sink; this will be changed as command options are processed
	logger = logr.Discard()
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		logger.Error(err, "Error executing command")
	}
}
