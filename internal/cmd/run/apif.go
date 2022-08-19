package run

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/apif"
	"github.com/saucelabs/saucectl/internal/region"
)

func runApif() (int, error) {
	p, err := apif.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	log.Info().Str("kind", p.Kind).Msg("Running apif tests")

	regio := region.FromString(p.Sauce.Region)	

	// testcompClient.URL = regio.APIBaseURL()
	// webdriverClient.URL = regio.WebDriverBaseURL()
	// restoClient.URL = regio.APIBaseURL()
	// appsClient.URL = regio.APIBaseURL()
	// rdcClient.URL = regio.APIBaseURL()
	// insightsClient.URL = regio.APIBaseURL()
	// iamClient.URL = regio.APIBaseURL()

	apifClient.URL = regio.APIBaseURL()

	// TODO: Set defaults
	// TODO: Validate

	// TODO: Run suites
	// runSuites(p.Suites)
	r := apif.ApifRunner{
		Project: p,
		Client: apifClient,
	}

	r.RunSuites()
	return 0, nil
}

func runSuites(suites []apif.Suite) bool {
	var failureCount int
	for _, s := range suites {
		resp, err := apifClient.RunAllSync(context.Background(), s.Project, "json", "")
		if err != nil {
			log.Error().Err(err).Msg("Failed to run")
		}

		for _, r := range resp {
			failureCount += r.FailuresCount
		}
	}

	log.Info().Int("failures", failureCount).Msg("Finished running")
	return true
}
