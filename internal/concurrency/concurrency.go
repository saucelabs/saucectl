package concurrency

import (
	"context"
	"github.com/rs/zerolog/log"
)

// Min reads the allowed concurrency from r, compares it against ccy and returns the smaller value of the two.
// A value of 1 is returned if r is unable to provide one.
func Min(r Reader, ccy int) int {
	allowed, err := r.ReadAllowedCCY(context.Background())
	if err != nil {
		log.Warn().Err(err).Msg("Unable to determine allowed concurrency. Using concurrency of 1.")
		return 1
	}

	if ccy > allowed {
		log.Warn().Msgf("Allowed concurrency is %d. Overriding configured value of %d.",
			allowed, ccy)
		return allowed
	}

	return ccy
}