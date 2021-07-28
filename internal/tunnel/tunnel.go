package tunnel

import (
	"context"
	"time"
)

// Service represents an interface for interacting with Sauce Connect tunnels.
type Service interface {
	IsTunnelRunning(ctx context.Context, id, parent string, wait time.Duration) error
}
