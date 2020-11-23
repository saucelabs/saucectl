package sauce

import (
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/cypress"
)

// Runner represents the Sauce Labs cloud implementation for cypress.
type Runner struct {
	Project         cypress.Project
}

func (r *Runner) RunProject() (int, error) {
	log.Warn().Msg("Not yet implemented.")
	return 0, nil
}
