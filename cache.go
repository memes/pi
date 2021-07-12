package pi

import (
	"context"
)

type Cache interface {
	GetValue(ctx context.Context, key string) (string, error)
	SetValue(ctx context.Context, key string, value string) error
}

// noopCache implements Cache interface without any real cacheing.
type noopCache struct{}

func (n *noopCache) GetValue(ctx context.Context, key string) (string, error) {
	return "", nil
}

func (n *noopCache) SetValue(ctx context.Context, key string, value string) error {
	return nil
}

// Creates a NoopCache which ignores all
func NewNoopCache() *noopCache {
	return &noopCache{}
}
