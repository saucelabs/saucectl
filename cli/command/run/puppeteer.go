package run

import (
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/docker"
	"github.com/saucelabs/saucectl/internal/puppeteer"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/resto"
	"github.com/saucelabs/saucectl/internal/testcomposer"
	"github.com/spf13/cobra"
)

func runPuppeteer(cmd *cobra.Command, tc testcomposer.Client, rs resto.Client) (int, error) {
	p, err := puppeteer.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}
	p.Sauce.Metadata.ExpandEnv()
	applyDefaultValues(&p.Sauce)
	overrideCliParameters(cmd, &p.Sauce, &p.Artifacts)

	for k, v := range gFlags.env {
		for _, s := range p.Suites {
			if s.Env == nil {
				s.Env = map[string]string{}
			}
			s.Env[k] = v
		}
	}

	if gFlags.showConsoleLog {
		p.ShowConsoleLog = true
	}

	if cmd.Flags().Lookup("suite").Changed {
		if err := filterPuppeteerSuite(&p); err != nil {
			return 1, err
		}
	}

	regio := region.FromString(p.Sauce.Region)
	if regio == region.None {
		log.Error().Str("region", gFlags.regionFlag).Msg("Unable to determine sauce region.")
		return 1, errors.New("no sauce region set")
	}

	rs.URL = regio.APIBaseURL()
	tc.URL = regio.APIBaseURL()
	return runPuppeteerInDocker(p, tc, rs)
}

func runPuppeteerInDocker(p puppeteer.Project, testco testcomposer.Client, rs resto.Client) (int, error) {
	log.Info().Msg("Running puppeteer in Docker")
	printTestEnv("docker")

	cd, err := docker.NewPuppeteer(p, &testco, &testco, &rs)
	if err != nil {
		return 1, err
	}
	return cd.RunProject()
}

func filterPuppeteerSuite(c *puppeteer.Project) error {
	for _, s := range c.Suites {
		if s.Name == gFlags.suiteName {
			c.Suites = []puppeteer.Suite{s}
			return nil
		}
	}
	return fmt.Errorf("suite name '%s' is invalid", gFlags.suiteName)
}

