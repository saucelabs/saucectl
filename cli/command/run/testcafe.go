package run

import (
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/cli/flags"
	"github.com/saucelabs/saucectl/internal/appstore"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/docker"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/resto"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/sentry"
	"github.com/saucelabs/saucectl/internal/testcafe"
	"github.com/saucelabs/saucectl/internal/testcomposer"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"os"
)

type testcafeFlags struct {
	RootDir string
	// Simulator is set outside of Suite due to its special, flaggable type.
	Simulator flags.Simulator
	Suite     testcafe.Suite
	Testcafe  testcafe.Testcafe
	NPM       config.Npm
	FlagSet   *pflag.FlagSet
}

// NewTestcafeCmd creates the 'run' command for TestCafe.
func NewTestcafeCmd() *cobra.Command {
	lflags := testcafeFlags{}

	cmd := &cobra.Command{
		Use:              "testcafe",
		Short:            "Run testcafe tests",
		Hidden:           true, // TODO reveal command once ready
		TraverseChildren: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRun()
		},
		Run: func(cmd *cobra.Command, args []string) {
			// Test patterns are passed in via positional args.
			lflags.Suite.Src = args

			exitCode, err := runTestcafe(cmd, lflags, tcClient, restoClient, appsClient)
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

	f := cmd.Flags()
	lflags.FlagSet = f
	f.StringVar(&lflags.Suite.Name, "name", "", "Sets the name of the job as it will appear on Sauce Labs")

	// Browser & Platform
	f.StringVar(&lflags.Suite.BrowserName, "browserName", "", "Run tests against this browser")
	f.StringVar(&lflags.Suite.BrowserVersion, "browserVersion", "", "The browser version (default: latest)")
	f.StringVar(&lflags.Suite.PlatformName, "platformName", "", "Run tests against this platform")

	// Video & Screen(shots)
	f.BoolVar(&lflags.Suite.DisableScreenshots, "disableScreenshots", false, "Prevent TestCafe from taking screenshots")
	f.StringVar(&lflags.Suite.ScreenResolution, "screenResolution", "", "The screen resolution")
	f.BoolVar(&lflags.Suite.Screenshots.TakeOnFails, "screenshots.takeOnFails", false, "Take screenshot on test failure")
	f.BoolVar(&lflags.Suite.Screenshots.FullPage, "screenshots.fullPage", false, "Take screenshots of the entire page")

	// Error Handling
	f.BoolVar(&lflags.Suite.QuarantineMode, "quarantineMode", false, "Enable the quarantine mode for tests that fail")
	f.BoolVar(&lflags.Suite.SkipJsErrors, "skipJsErrors", false, "Ignore JavaScript errors that occur on a tested web page")
	f.BoolVar(&lflags.Suite.SkipUncaughtErrors, "skipUncaughtErrors", false, "Ignore uncaught errors or unhandled promise rejections on the server during test execution")
	f.BoolVar(&lflags.Suite.StopOnFirstFail, "stopOnFirstFail", false, "Stop an entire test run if any test fails")

	// Timeouts
	f.IntVar(&lflags.Suite.SelectorTimeout, "selectorTimeout", 10000, "milliseconds")
	f.IntVar(&lflags.Suite.AssertionTimeout, "assertionTimeout", 3000, "milliseconds")
	f.IntVar(&lflags.Suite.PageLoadTimeout, "pageLoadTimeout", 3000, "milliseconds")

	// Misc
	f.StringVar(&lflags.RootDir, "rootDir", ".", "Controls what files are available in the context of a test run, unless explicitly excluded by .sauceignore")
	f.StringVar(&lflags.Testcafe.Version, "testcafe.version", "", "The TestCafe version to use")
	f.StringSliceVar(&lflags.Suite.ClientScripts, "clientScripts", []string{}, "Inject scripts from the specified files into each page visited during the tests")
	f.Float64Var(&lflags.Suite.Speed, "speed", 1, "Specify the test execution speed")
	f.BoolVar(&lflags.Suite.DisablePageCaching, "disablePageCaching", false, "Prevent the browser from caching page content")

	// NPM
	f.StringVar(&lflags.NPM.Registry, "npm.registry", "", "Specify the npm registry URL")
	f.StringToStringVar(&lflags.NPM.Packages, "npm.packages", map[string]string{}, "Specify npm packages that are required to run tests")
	f.BoolVar(&lflags.NPM.StrictSSL, "npm.strictSSL", true, "Whether or not to do SSL key validation when making requests to the registry via https")

	// Simulators
	f.Var(&lflags.Simulator, "simulator", "Specifies the simulator to use for testing")

	return cmd
}

func runTestcafe(cmd *cobra.Command, flags testcafeFlags, tc testcomposer.Client, rs resto.Client, as appstore.AppStore) (int, error) {
	p, err := testcafe.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	p.Sauce.Metadata.ExpandEnv()
	applyGlobalFlags(cmd, &p.Sauce, &p.Artifacts)
	applyTestcafeFlags(&p, flags)

	for k, v := range gFlags.env {
		for _, s := range p.Suites {
			if s.Env == nil {
				s.Env = map[string]string{}
			}
			s.Env[k] = v
		}
	}

	if cmd.Flags().Lookup("suite").Changed {
		if err := filterTestcafeSuite(&p); err != nil {
			return 1, err
		}
	}
	if p.Defaults.Mode == "" {
		p.Defaults.Mode = "sauce"
	}
	for i, s := range p.Suites {
		if s.Mode == "" {
			s.Mode = p.Defaults.Mode
		}
		p.Suites[i] = s
	}
	if gFlags.testEnv != "" {
		for i, s := range p.Suites {
			s.Mode = gFlags.testEnv
			p.Suites[i] = s
		}
	}

	regio := region.FromString(p.Sauce.Region)
	if regio == region.None {
		log.Error().Str("region", gFlags.regionFlag).Msg("Unable to determine sauce region.")
		return 1, errors.New("no sauce region set")
	}

	tc.URL = regio.APIBaseURL()
	rs.URL = regio.APIBaseURL()
	as.URL = regio.APIBaseURL()

	rs.ArtifactConfig = p.Artifacts.Download

	if err := testcafe.Validate(&p); err != nil {
		return 1, err
	}

	dockerProject, sauceProject := testcafe.SplitSuites(p)
	if len(dockerProject.Suites) != 0 {
		exitCode, err := runTestcafeInDocker(dockerProject, tc, rs)
		if err != nil || exitCode != 0 {
			return exitCode, err
		}
	}
	if len(sauceProject.Suites) != 0 {
		return runTestcafeInCloud(sauceProject, regio, tc, rs, as)
	}

	return 0, nil
}

func runTestcafeInDocker(p testcafe.Project, testco testcomposer.Client, rs resto.Client) (int, error) {
	log.Info().Msg("Running Testcafe in Docker")
	printTestEnv("docker")

	cd, err := docker.NewTestcafe(p, &testco, &testco, &rs)
	if err != nil {
		return 1, err
	}
	return cd.RunProject()
}

func runTestcafeInCloud(p testcafe.Project, regio region.Region, tc testcomposer.Client, rs resto.Client, as appstore.AppStore) (int, error) {
	log.Info().Msg("Running Testcafe in Sauce Labs")
	printTestEnv("sauce")

	r := saucecloud.TestcafeRunner{
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
			DryRun:             gFlags.dryRun,
		},
	}
	return r.RunProject()
}

func filterTestcafeSuite(c *testcafe.Project) error {
	for _, s := range c.Suites {
		if s.Name == gFlags.suiteName {
			c.Suites = []testcafe.Suite{s}
			return nil
		}
	}
	return fmt.Errorf("suite name '%s' is invalid", gFlags.suiteName)
}

func applyTestcafeFlags(p *testcafe.Project, flags testcafeFlags) {
	if flags.Testcafe.Version != "" {
		p.Testcafe.Version = flags.Testcafe.Version
	}

	if flags.FlagSet.Changed("rootDir") || p.RootDir == "" {
		p.RootDir = flags.RootDir
	}

	if flags.NPM.Registry != "" {
		p.Npm.Registry = flags.NPM.Registry
	}

	if len(flags.NPM.Packages) != 0 {
		p.Npm.Packages = flags.NPM.Packages
	}

	if flags.FlagSet.Changed("npm.strictSSL") {
		p.Npm.StrictSSL = flags.NPM.StrictSSL
	}

	if gFlags.showConsoleLog {
		p.ShowConsoleLog = true
	}
	if gFlags.runnerVersion != "" {
		p.RunnerVersion = gFlags.runnerVersion
	}

	// No name, no adhoc suite.
	if flags.Suite.Name == "" {
		return
	}

	if flags.Simulator.Changed {
		flags.Suite.Simulators = []config.Simulator{flags.Simulator.Simulator}
	}
	p.Suites = []testcafe.Suite{flags.Suite}
}
