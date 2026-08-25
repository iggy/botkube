package reloader

import (
	"context"
)

// Reloader is an interface for reloading configuration.
type Reloader interface {
	Do(ctx context.Context) error
}
