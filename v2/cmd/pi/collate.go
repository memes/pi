package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Implements the collate sub-command which collects the resulting values into
// an output string.
func NewCollateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "collate target",
		Short: "Run a gRPC Pi Service client to request, collate, and print digits of pi",
		Long:  "Launches a gRPC client that will connect to Pi Service target and request the fractional digits of pi.",
		Args:  cobra.ExactArgs(1),
		RunE:  collateMain,
	}
}

// Collate sub-command entrypoint. This function will create a byte array to
// hold the results and a CollatorFunction to populate the byte array before
// calling the main client loop.
func collateMain(cmd *cobra.Command, endpoints []string) error {
	count := viper.GetInt(CountFlagName)
	logger := logger.V(1).WithValues(CountFlagName, count, "endpoints", endpoints)
	logger.V(0).Info("Preparing target buffer")
	digits := make([]byte, count)
	for i := range digits {
		digits[i] = '-'
	}
	// Set the global collator function to add digits to the array
	collator = func(index uint64, value uint32) error {
		digits[index] = '0' + byte(value)
		return nil
	}
	if err := clientMain(cmd, endpoints); err != nil {
		return err
	}
	fmt.Print("Result is: 3.") //nolint:forbidigo // This is a deliberate choice
	if _, err := os.Stdout.Write(digits); err != nil {
		return fmt.Errorf("failure writing result: %w", err)
	}
	fmt.Println() //nolint:forbidigo // This is a deliberate choice
	return nil
}
