package tunnel

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog/log"
)

// Filter filters tunnels by type when fetching a user's tunnels.
type Filter string

const (
	// NoneFilter is a noop and does no filtering.
	NoneFilter Filter = ""
	// V2AlphaFilter filters down to only tunnels with vm-version: v2alpha when requesting a user's tunnels.
	// NOTE: We use this filter when checking tunnel readiness for api tests. The response when
	// requesting a user's tunnel does not include vm-version metadata so we need to use the
	// filter to ensure the correct type of tunnel has been started for an api test.
	V2AlphaFilter Filter = "v2alpha"
)

// Service represents an interface for interacting with Sauce Connect tunnels.
type Service interface {
	IsTunnelRunning(ctx context.Context, id, parent string, filter Filter, wait time.Duration) error
}

func Validate(service Service, name string, owner string, filter Filter, dryRun bool, timeout time.Duration) error {
	if name == "" {
		return nil
	}

	if dryRun {
		log.Info().Msg("Skipping tunnel validation in dry run.")
		return nil
	}

	if timeout <= 0 {
		return errors.New("tunnel timeout must be greater than 0")
	}

	log.Info().Str("timeout", timeout.String()).Str("tunnel", name).Msg("Performing tunnel readiness check...")
	if err := service.IsTunnelRunning(context.Background(), name, owner, filter, timeout); err != nil {
		return err
	}

	log.Info().Str("tunnel", name).Msg("Tunnel is ready!")
	return nil
}
