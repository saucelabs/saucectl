package run

import (
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/saucelabs/saucectl/internal/appstore"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/docker"
	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report/captor"
	"github.com/saucelabs/saucectl/internal/resto"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/sentry"
	"github.com/saucelabs/saucectl/internal/testcafe"
	"github.com/saucelabs/saucectl/internal/testcomposer"
)

type testcafeFlags struct {
	Simulator flags.Simulator
}

// NewTestcafeCmd creates the 'run' command for TestCafe.
func NewTestcafeCmd() *cobra.Command {
	sc := flags.SnakeCharmer{Fmap: map[string]*pflag.Flag{}}
	lflags := testcafeFlags{}

	cmd := &cobra.Command{
		Use:              "testcafe",
		Short:            "Run testcafe tests",
		Hidden:           true, // TODO reveal command once ready
		TraverseChildren: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			sc.BindAll()
			return preRun()
		},
		Run: func(cmd *cobra.Command, args []string) {
			// Test patterns are passed in via positional args.
			viper.Set("suite.src", args)

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
	sc.Fset = cmd.Flags()
	sc.String("name", "suite.name", "", "Set the name of the job as it will appear on Sauce Labs")

	// Browser & Platform
	sc.String("browser", "suite.browserName", "", "Run tests against this browser")
	sc.String("browserVersion", "suite.browserVersion", "", "The browser version (default: latest)")
	sc.StringSlice("browserArgs", "suite.browserArgs", []string{}, "Set browser args")
	sc.String("platform", "suite.platformName", "", "Run tests against this platform")

	// Video & Screen(shots)
	sc.Bool("disableScreenshots", "suite.disableScreenshots", false, "Prevent TestCafe from taking screenshots")
	sc.String("screenResolution", "suite.screenResolution", "", "The screen resolution")
	sc.Bool("screenshots.takeOnFails", "suite.screenshots.takeOnFails", false, "Take screenshot on test failure")
	sc.Bool("screenshots.fullPage", "suite.screenshots.fullPage", false, "Take screenshots of the entire page")

	// Error Handling
	sc.Bool("quarantineMode", "suite.quarantineMode", false, "Enable the quarantine mode for tests that fail")
	sc.Bool("skipJsErrors", "suite.skipJsErrors", false, "Ignore JavaScript errors that occur on a tested web page")
	sc.Bool("skipUncaughtErrors", "suite.skipUncaughtErrors", false, "Ignore uncaught errors or unhandled promise rejections on the server during test execution")
	sc.Bool("stopOnFirstFail", "suite.stopOnFirstFail", false, "Stop an entire test run if any test fails")

	// Timeouts
	sc.Int("selectorTimeout", "suite.selectorTimeout", 10000, "Specify the time (in milliseconds) within which selectors attempt to return a node")
	sc.Int("assertionTimeout", "suite.assertionTimeout", 3000, "Specify the time (in milliseconds) TestCafe attempts to successfully execute an assertion")
	sc.Int("pageLoadTimeout", "suite.pageLoadTimeout", 3000, "Specify the time (in milliseconds) passed after the DOMContentLoaded event, within which TestCafe waits for the window.load event to fire")

	// Misc
	sc.String("rootDir", "rootDir", ".", "Control what files are available in the context of a test run, unless explicitly excluded by .sauceignore")
	sc.String("testcafe.version", "testcafe.version", "", "The TestCafe version to use")
	sc.StringSlice("clientScripts", "suite.clientScripts", []string{}, "Inject scripts from the specified files into each page visited during the tests")
	sc.Float64("speed", "suite.speed", 1, "Specify the test execution speed")
	sc.Bool("disablePageCaching", "suite.disablePageCaching", false, "Prevent the browser from caching page content")

	// NPM
	sc.String("npm.registry", "npm.registry", "", "Specify the npm registry URL")
	sc.StringToString("npm.packages", "npm.packages", map[string]string{}, "Specify npm packages that are required to run tests")
	sc.Bool("npm.strictSSL", "npm.strictSSL", true, "Whether or not to do SSL key validation when making requests to the registry via https")

	// Simulators
	f.Var(&lflags.Simulator, "simulator", "Specifies the simulator to use for testing")

	return cmd
}

func runTestcafe(cmd *cobra.Command, tcFlags testcafeFlags, tc testcomposer.Client, rs resto.Client, as appstore.AppStore) (int, error) {
	p, err := testcafe.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	p.CLIFlags = flags.CaptureCommandLineFlags(cmd.Flags())
	p.Sauce.Metadata.ExpandEnv()

	if err := applyTestcafeFlags(&p, tcFlags); err != nil {
		return 1, err
	}
	testcafe.SetDefaults(&p)

	if err := testcafe.Validate(&p); err != nil {
		return 1, err
	}

	regio := region.FromString(p.Sauce.Region)
	tc.URL = regio.APIBaseURL()
	rs.URL = regio.APIBaseURL()
	as.URL = regio.APIBaseURL()

	rs.ArtifactConfig = p.Artifacts.Download

	tracker := segment.New()

	defer func() {
		props := usage.Properties{}
		props.SetFramework("testcafe").SetFVersion(p.Testcafe.Version).SetFlags(cmd.Flags()).SetSauceConfig(p.Sauce).
			SetArtifacts(p.Artifacts).SetDocker(p.Docker).SetNPM(p.Npm).SetNumSuites(len(p.Suites)).SetJobs(captor.Default.TestResults)
		tracker.Collect(strings.Title(fullCommandName(cmd)), props)
		_ = tracker.Close()
	}()

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

	reporters := createReporters(p.Reporters)
	reporters = append(reporters, createSlackReporter(p.Notifications, p.Sauce.Metadata, &testco,
		"testcafe", "docker"))

	cd, err := docker.NewTestcafe(p, &testco, &testco, &rs, &rs, reporters)
	if err != nil {
		return 1, err
	}

	cleanTestCafePackages(&p)
	return cd.RunProject()
}

func runTestcafeInCloud(p testcafe.Project, regio region.Region, tc testcomposer.Client, rs resto.Client, as appstore.AppStore) (int, error) {
	log.Info().Msg("Running Testcafe in Sauce Labs")
	printTestEnv("sauce")

	reporters := createReporters(p.Reporters)
	reporters = append(reporters, createSlackReporter(p.Notifications, p.Sauce.Metadata, &tc,
		"testcafe", "sauce"))

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
			Reporters:          reporters,
		},
	}
	cleanTestCafePackages(&p)
	return r.RunProject()
}

func applyTestcafeFlags(p *testcafe.Project, flags testcafeFlags) error {
	if gFlags.selectedSuite != "" {
		if err := testcafe.FilterSuites(p, gFlags.selectedSuite); err != nil {
			return err
		}
	}

	if p.Suite.Name == "" {
		return nil
	}

	if flags.Simulator.Changed {
		p.Suite.Simulators = []config.Simulator{flags.Simulator.Simulator}
	}

	p.Suites = []testcafe.Suite{p.Suite}

	return nil
}

func cleanTestCafePackages(p *testcafe.Project) {
	version, hasFramework := p.Npm.Packages["testcafe"]
	if hasFramework {
		log.Warn().Msg(msg.IgnoredNpmPackagesMsg("testcafe", p.Testcafe.Version, []string{fmt.Sprintf("testcafe@%s", version)}))
		p.Npm.Packages = config.CleanNpmPackages(p.Npm.Packages, []string{"testcafe"})
	}
}
