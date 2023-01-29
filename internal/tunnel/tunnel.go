package tunnel

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

// Filter filters tunnels by type when fetching a user's tunnels.
type Filter string

const (
	// NoneFilter is a noop and does no filtering.
	NoneFilter    Filter = ""
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

func ValidateTunnel(service Service, name string, owner string, filter Filter, dryRun bool) error {
	if name == "" {
		return nil
	}

	if dryRun {
		log.Info().Msg("Skipping tunnel validation in dry run.")
		return nil
	}

	// This wait value is deliberately not configurable.
	wait := 30 * time.Second
	log.Info().Str("timeout", wait.String()).Msg("Performing tunnel readiness check...")
	if err := service.IsTunnelRunning(context.Background(), name, owner, filter, wait); err != nil {
		return err
	}

	log.Info().Msg("Tunnel is ready!")
	return nil
}
