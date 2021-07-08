package run

import (
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/appstore"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/docker"
	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/resto"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/sentry"
	"github.com/saucelabs/saucectl/internal/testcomposer"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"os"
)

// NewPlaywrightCmd creates the 'run' command for Playwright.
func NewPlaywrightCmd() *cobra.Command {
	sc := flags.SnakeCharmer{Fmap: map[string]*pflag.Flag{}}

	cmd := &cobra.Command{
		Use:              "playwright",
		Short:            "Run playwright tests",
		Hidden:           true, // TODO reveal command once ready
		TraverseChildren: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			sc.BindAll()
			return preRun()
		},
		Run: func(cmd *cobra.Command, args []string) {
			// Test patterns are passed in via positional args.
			viper.Set("suite.testMatch", args)

			exitCode, err := runPlaywright(cmd, tcClient, restoClient, appsClient)
			if err != nil {
				log.Err(err).Msg("failed to execute run command")
				sentry.CaptureError(err, sentry.Scope{
					Username:   credentials.Get().Username,
					ConfigFile: gFlags.cfgFilePath,
				})
			}
			os.Exit(exitCode)
		},
	}

	sc.Fset = cmd.Flags()

	sc.String("name", "suite.name", "", "Set the name of the job as it will appear on Sauce Labs")

	// Browser & Platform
	sc.String("browser", "suite.params.browserName", "", "Run tests against this browser")
	sc.String("platformName", "suite.platformName", "", "Run tests against this platform")

	// Playwright
	sc.String("playwright.version", "playwright.version", "", "The Playwright version to use")

	// Playwright Test Options
	sc.Bool("headed", "suite.params.headed", false, "Run tests in headed browsers")
	sc.Int("globalTimeout", "suite.params.globalTimeout", 0, "Total timeout for the whole test run in milliseconds")
	sc.Int("testTimeout", "suite.params.timeout", 0, "Maximum timeout in milliseconds for each test")
	sc.String("grep", "suite.params.grep", "", "Only run tests matching this regular expression")
	sc.Int("repeatEach", "suite.params.repeatEach", 0, "Run each test N times")
	sc.Int("retries", "suite.params.retries", 0, "The maximum number of retries for flaky tests")
	sc.Int("maxFailures", "suite.params.maxFailures", 0, "Stop after the first N test failures")
	// TODO sharding

	// Misc
	sc.String("rootDir", "rootDir", ".", "Control what files are available in the context of a test run, unless explicitly excluded by .sauceignore")

	// NPM
	sc.String("npm.registry", "npm.registry", "", "Specify the npm registry URL")
	sc.StringToString("npm.packages", "npm.packages", map[string]string{}, "Specify npm packages that are required to run tests")
	sc.Bool("npm.strictSSL", "npm.strictSSL", true, "Whether or not to do SSL key validation when making requests to the registry via https")

	return cmd
}

func runPlaywright(cmd *cobra.Command, tc testcomposer.Client, rs resto.Client, as appstore.AppStore) (int, error) {
	p, err := playwright.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	p.Sauce.Metadata.ExpandEnv()
	applyGlobalFlags(cmd, &p.Sauce, &p.Artifacts)
	if err := applyPlaywrightFlags(&p); err != nil {
		return 1, err
	}
	playwright.SetDefaults(&p)

	if err := playwright.Validate(&p); err != nil {
		return 1, err
	}

	regio := region.FromString(p.Sauce.Region)

	tc.URL = regio.APIBaseURL()
	rs.URL = regio.APIBaseURL()
	as.URL = regio.APIBaseURL()

	rs.ArtifactConfig = p.Artifacts.Download

	dockerProject, sauceProject := playwright.SplitSuites(p)
	if len(dockerProject.Suites) != 0 {
		exitCode, err := runPlaywrightInDocker(dockerProject, tc, rs)
		if err != nil || exitCode != 0 {
			return exitCode, err
		}
	}
	if len(sauceProject.Suites) != 0 {
		return runPlaywrightInSauce(sauceProject, regio, tc, rs, as)
	}

	return 0, nil
}

func runPlaywrightInDocker(p playwright.Project, testco testcomposer.Client, rs resto.Client) (int, error) {
	log.Info().Msg("Running Playwright in Docker")
	printTestEnv("docker")

	cd, err := docker.NewPlaywright(p, &testco, &testco, &rs)
	if err != nil {
		return 1, err
	}
	return cd.RunProject()
}

func runPlaywrightInSauce(p playwright.Project, regio region.Region, tc testcomposer.Client, rs resto.Client, as appstore.AppStore) (int, error) {
	log.Info().Msg("Running Playwright in Sauce Labs")
	printTestEnv("sauce")

	r := saucecloud.PlaywrightRunner{
		Project: p,
		CloudRunner: saucecloud.CloudRunner{
			ProjectUploader:    &as,
			JobStarter:         &tc,
			JobReader:          &rs,
			JobStopper:         &rs,
			JobWriter:          &tc,
			CCYReader:          &rs,
			TunnelService:      &rs,
			Region:             regio,
			ShowConsoleLog:     p.ShowConsoleLog,
			ArtifactDownloader: &rs,
		},
	}
	return r.RunProject()
}

func applyPlaywrightFlags(p *playwright.Project) error {
	if gFlags.selectedSuite != "" {
		if err := playwright.FilterSuites(p, gFlags.selectedSuite); err != nil {
			return err
		}
	}

	// Use the adhoc suite instead, if one is provided
	if p.Suite.Name != "" {
		p.Suites = []playwright.Suite{p.Suite}
	}

	return nil
}
