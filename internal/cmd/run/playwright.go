package run

import (
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/saucelabs/saucectl/internal/appstore"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/docker"
	"github.com/saucelabs/saucectl/internal/download"
	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report/captor"
	"github.com/saucelabs/saucectl/internal/resto"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/sentry"
	"github.com/saucelabs/saucectl/internal/testcomposer"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/saucelabs/saucectl/internal/viper"
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
			viper.Set("suite::testMatch", args)

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

	sc.String("name", "suite::name", "", "Set the name of the job as it will appear on Sauce Labs")

	// Browser & Platform
	sc.String("browser", "suite::params::browserName", "", "Run tests against this browser")
	sc.String("platform", "suite::platformName", "", "Run tests against this platform")

	// Playwright
	sc.String("playwright.version", "playwright::version", "", "The Playwright version to use")
	sc.String("playwright.configFile", "playwright::configFile", "", "The path to playwright config file")

	// Playwright Test Options
	sc.Bool("headed", "suite::params::headed", false, "Run tests in headed browsers")
	sc.Bool("headless", "suite::params::headless", false, "Run tests in headless mode")
	sc.Int("globalTimeout", "suite::params::globalTimeout", 0, "Total timeout for the whole test run in milliseconds")
	sc.Int("testTimeout", "suite::params::timeout", 0, "Maximum timeout in milliseconds for each test")
	sc.String("grep", "suite::params::grep", "", "Only run tests matching this regular expression")
	sc.Int("repeatEach", "suite::params::repeatEach", 0, "Run each test N times")
	sc.Int("retries", "suite::params::retries", 0, "The maximum number of retries for flaky tests")
	sc.Int("maxFailures", "suite::params::maxFailures", 0, "Stop after the first N test failures")
	sc.Int("numShards", "suite::numShards", 0, "Split tests across N number of shards")
	sc.String("project", "suite::params::project", "", "Specify playwright project")
	sc.Bool("headless", "suite::params::headless", false, "Run tests in headless mode")

	// Misc
	sc.String("rootDir", "rootDir", ".", "Control what files are available in the context of a test run, unless explicitly excluded by .sauceignore")
	sc.String("shard", "suite.shard", "", "Controls whether or not (and how) tests are sharded across multiple machines")

	// NPM
	sc.String("npm.registry", "npm::registry", "", "Specify the npm registry URL")
	sc.StringToString("npm.packages", "npm::packages", map[string]string{}, "Specify npm packages that are required to run tests")
	sc.Bool("npm.strictSSL", "npm::strictSSL", true, "Whether or not to do SSL key validation when making requests to the registry via https")

	return cmd
}

func runPlaywright(cmd *cobra.Command, tc testcomposer.Client, rs resto.Client, as appstore.AppStore) (int, error) {
	p, err := playwright.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	p.CLIFlags = flags.CaptureCommandLineFlags(cmd.Flags())
	p.Sauce.Metadata.ExpandEnv()

	if err := applyPlaywrightFlags(&p); err != nil {
		return 1, err
	}
	playwright.SetDefaults(&p)

	if err := playwright.Validate(&p); err != nil {
		return 1, err
	}

	if err := playwright.ShardSuites(&p); err != nil {
		return 1, err
	}

	regio := region.FromString(p.Sauce.Region)

	tc.URL = regio.APIBaseURL()
	rs.URL = regio.APIBaseURL()
	as.URL = regio.APIBaseURL()

	rs.ArtifactConfig = p.Artifacts.Download

	tracker := segment.New(!gFlags.disableUsageMetrics)

	defer func() {
		props := usage.Properties{}
		props.SetFramework("playwright").SetFVersion(p.Playwright.Version).SetFlags(cmd.Flags()).SetSauceConfig(p.Sauce).
			SetArtifacts(p.Artifacts).SetDocker(p.Docker).SetNPM(p.Npm).SetNumSuites(len(p.Suites)).SetJobs(captor.Default.TestResults).
			SetSlack(p.Notifications.Slack)
		tracker.Collect(strings.Title(fullCommandName(cmd)), props)
		_ = tracker.Close()
	}()

	if p.Artifacts.Cleanup {
		download.Cleanup(p.Artifacts.Download.Directory)
	}

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

	cd, err := docker.NewPlaywright(p, &testco, &testco, &rs, &rs, createReporters(p.Reporters, p.Notifications, p.Sauce.Metadata, &testco,
		"playwright", "docker"))
	if err != nil {
		return 1, err
	}

	cleanPlaywrightPackages(&p)
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
			MetadataService:    &tc,
			Region:             regio,
			ShowConsoleLog:     p.ShowConsoleLog,
			ArtifactDownloader: &rs,
			Reporters: createReporters(p.Reporters, p.Notifications, p.Sauce.Metadata, &tc,
				"playwright", "sauce"),
			Async:    gFlags.async,
			FailFast: gFlags.failFast,
		},
	}

	cleanPlaywrightPackages(&p)
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

func cleanPlaywrightPackages(p *playwright.Project) {
	// Don't allow framework installation, it is provided by the runner
	ignoredPackages := []string{}
	playwrightVersion, hasPlaywright := p.Npm.Packages["playwright"]
	playwrightTestVersion, hasPlaywrightTest := p.Npm.Packages["@playwright/test"]
	if hasPlaywright {
		ignoredPackages = append(ignoredPackages, fmt.Sprintf("playwright@%s", playwrightVersion))
	}
	if hasPlaywrightTest {
		ignoredPackages = append(ignoredPackages, fmt.Sprintf("@playwright/test@%s", playwrightTestVersion))
	}
	if hasPlaywright || hasPlaywrightTest {
		log.Warn().Msg(msg.IgnoredNpmPackagesMsg("playwright", p.Playwright.Version, ignoredPackages))
		p.Npm.Packages = config.CleanNpmPackages(p.Npm.Packages, []string{"playwright", "@playwright/test"})
	}
}
