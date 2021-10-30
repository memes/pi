package pi

import (
	"context"
)

// Cache defines an interface for a cache implementation that can be used to
// store the results of CalcDigits for subsequent lookup requests.
type Cache interface {
	// Return the string that was set for key (or "" if unset) and an Error
	// if the implementation failed.
	// NOTE: a cache miss *should not* return an error.
	GetValue(ctx context.Context, key string) (string, error)
	// Store the value string with the provided key, returning an error if
	// the implementation failed.
	SetValue(ctx context.Context, key string, value string) error
}

// noopCache implements Cache interface without any real cacheing.
type noopCache struct{}

// Always returns an empty string and no error for every key.
func (n *noopCache) GetValue(ctx context.Context, key string) (string, error) {
	return "", nil
}

// Ignores the value and returns nil error.
func (n *noopCache) SetValue(ctx context.Context, key string, value string) error {
	return nil
}

// Creates a no-operation Cache implementation that satisfies the interface
// requirements without performing any real caching. All values are silently
// dropped by SetValue and calls to GetValue always return an empty string.
func NewNoopCache() *noopCache {
	return &noopCache{}
}
