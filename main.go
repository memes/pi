package main

import (
	"github.com/memes/pi/cmd"
	"go.uber.org/zap"
)

func main() {
	defer func() {
		if cmd.Logger != nil {
			_ = cmd.Logger.Sync()
		}
	}()
	if err := cmd.RootCmd.Execute(); err != nil {
		cmd.Logger.Fatal("Error executing command",
			zap.Error(err),
		)
	}
}
