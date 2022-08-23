package tunnel

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

type Filter string

const (
	NoneFilter    Filter = ""
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
