package retry

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/job"
)

type BasicRetrier struct{}

func (b *BasicRetrier) Retry(_ context.Context, jobOpts chan<- job.StartOptions, opt job.StartOptions, _ job.Job) {
	log.Info().Str("suite", opt.DisplayName).
		Str("attempt", fmt.Sprintf("%d of %d", opt.Attempt+1, opt.Retries+1)).
		Msg("Retrying suite.")
	jobOpts <- opt
}
