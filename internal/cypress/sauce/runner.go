package sauce

import (
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/cypress"
)

// Runner represents the Sauce Labs cloud implementation for cypress.
type Runner struct {
	Project         cypress.Project
}

// RunProject runs the tests defined in cypress.Project.
func (r *Runner) RunProject() (int, error) {
	log.Error().Msg("Not yet implemented.")
	return 1, nil
}
