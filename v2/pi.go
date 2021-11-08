// Package pi calculates the nth fractional digit of pi using a Bailey-Borwein-Plouffe
// algorithm (see https://wikipedia.org/wiki/Bailey%E2%80%93Borwein%E2%80%93Plouffe_formula).
// This allows any arbitrary fractional digit of pi to be calculated independently
// of the preceding digits albeit with longer calculation times as the value
// of n increases because of the need to calculate prime numbers of
// increasing value.
//
//
//
// NOTE: This package is intended to be used in distributed computing and cloud
// scaling demos, and does not guarantee accuracy or efficiency of calculated
// fractional digits.
package pi

import (
	"github.com/go-logr/logr"
)

var (
	// Logger to use in this package; default is a no-op logger.
	logger = logr.Discard()
)

// Change the logger instance used by this package.
func WithLogger(l logr.Logger) {
	logger = l
}
