package run

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/saucelabs/saucectl/internal/appstore"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/espresso"
	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/github"
	"github.com/saucelabs/saucectl/internal/junit"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/notification/slack"
	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/saucelabs/saucectl/internal/puppeteer"
	"github.com/saucelabs/saucectl/internal/rdc"
	"github.com/saucelabs/saucectl/internal/report"
	"github.com/saucelabs/saucectl/internal/report/captor"
	"github.com/saucelabs/saucectl/internal/report/table"
	"github.com/saucelabs/saucectl/internal/resto"
	"github.com/saucelabs/saucectl/internal/sentry"
	"github.com/saucelabs/saucectl/internal/testcafe"
	"github.com/saucelabs/saucectl/internal/testcomposer"
	"github.com/saucelabs/saucectl/internal/version"
	"github.com/saucelabs/saucectl/internal/xcuitest"
)

var (
	runUse   = "run"
	runShort = "Runs tests on Sauce Labs"

	// General Request Timeouts
	appStoreTimeout     = 300 * time.Second
	testComposerTimeout = 300 * time.Second
	restoTimeout        = 60 * time.Second
	rdcTimeout          = 15 * time.Second
	githubTimeout       = 2 * time.Second

	typeDef config.TypeDef

	tcClient    testcomposer.Client
	restoClient resto.Client
	appsClient  appstore.AppStore
	rdcClient   rdc.Client

	// ErrEmptySuiteName is thrown when a flag is specified that has a dependency on the --name flag.
	ErrEmptySuiteName = errors.New(msg.EmptyAdhocSuiteName)
)

// gFlags contains all global flags that are set when 'run' is invoked.
var gFlags = globalFlags{}

type globalFlags struct {
	cfgFilePath         string
	globalTimeout       time.Duration
	sauceAPI            string
	selectedSuite       string
	testEnvSilent       bool
	disableUsageMetrics bool
	async               bool
	failFast            bool
}

// Command creates the `run` command
func Command() *cobra.Command {
	sc := flags.SnakeCharmer{Fmap: map[string]*pflag.Flag{}}

	cmd := &cobra.Command{
		Use:              runUse,
		Short:            runShort,
		TraverseChildren: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRun()
		},
		Run: func(cmd *cobra.Command, args []string) {
			exitCode, err := Run(cmd)
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

	sc.Fset = cmd.PersistentFlags()

	defaultCfgPath := filepath.Join(".sauce", "config.yml")
	cmd.PersistentFlags().StringVarP(&gFlags.cfgFilePath, "config", "c", defaultCfgPath, "Specifies which config file to use")
	cmd.PersistentFlags().DurationVarP(&gFlags.globalTimeout, "timeout", "t", 0, "Global timeout that limits how long saucectl can run in total. Supports duration values like '10s', '30m' etc. (default: no timeout)")
	cmd.PersistentFlags().BoolVar(&gFlags.async, "async", false, "Launches tests without waiting for test results (sauce mode only)")
	cmd.PersistentFlags().BoolVar(&gFlags.failFast, "fail-fast", false, "Stops suites after the first failure (sauce mode only)")
	sc.StringP("region", "r", "sauce::region", "us-west-1", "The sauce labs region.")
	sc.StringToStringP("env", "e", "env", map[string]string{}, "Set environment variables, e.g. -e foo=bar. Not supported when running espresso/xcuitest!")
	sc.Bool("show-console-log", "showConsoleLog", false, "Shows suites console.log locally. By default console.log is only shown on failures.")
	sc.Int("ccy", "sauce::concurrency", 2, "Concurrency specifies how many suites are run at the same time.")
	sc.String("tunnel-id", "sauce::tunnel::id", "", "Sets the sauce-connect tunnel ID to be used for the run.")
	sc.String("tunnel-name", "sauce::tunnel::name", "", "Sets the sauce-connect tunnel name to be used for the run.")
	sc.String("tunnel-parent", "sauce::tunnel:;parent", "", "Sets the sauce-connect tunnel parent to be used for the run.")
	sc.String("tunnel-owner", "sauce::tunnel::owner", "", "Sets the sauce-connect tunnel owner to be used for the run.")
	sc.String("runner-version", "runnerVersion", "", "Overrides the automatically determined runner version.")
	sc.String("sauceignore", "sauce::sauceignore", ".sauceignore", "Specifies the path to the .sauceignore file.")
	sc.StringToString("experiment", "sauce::experiment", map[string]string{}, "Specifies a list of experimental flags and values")
	sc.Bool("dry-run", "dryRun", false, "Simulate a test run without actually running any tests.")
	sc.Int("retries", "sauce::retries", 0, "Retries specifies the number of times to retry a failed suite (sauce mode only)")

	// Metadata
	sc.StringSlice("tags", "sauce::metadata::tags", []string{}, "Adds tags to tests")
	sc.String("build", "sauce::metadata::build", "", "Associates tests with a build")

	// Artifacts
	sc.String("artifacts.download.when", "artifacts::download::when", "never", "Specifies when to download test artifacts")
	sc.StringSlice("artifacts.download.match", "artifacts::download::match", []string{}, "Specifies which test artifacts to download")
	sc.String("artifacts.download.directory", "artifacts::download::directory", "", "Specifies the location where to download test artifacts to")
	sc.Bool("artifacts.cleanup", "artifacts::cleanup", false, "Specifies whether to remove all contents of artifacts directory")

	// Reporters
	sc.Bool("reporters.junit.enabled", "reporters::junit::enabled", false, "Toggle saucectl's own junit reporting on/off. This only affects the reports that saucectl itself generates as a summary of your tests. Each Job in Sauce Labs has an independent report regardless.")
	sc.String("reporters.junit.filename", "reporters::junit::filename", "saucectl-report.xml", "Specifies the report filename.")

	// Hide undocumented flags that the user does not need to care about.
	// FIXME sauce-api is actually not implemented, but probably should
	cmd.PersistentFlags().StringVar(&gFlags.sauceAPI, "sauce-api", "", "Overrides the region specific sauce API URL. (e.g. https://api.us-west-1.saucelabs.com)")
	cmd.PersistentFlags().StringVar(&gFlags.selectedSuite, "select-suite", "", "Run specified test suite.")
	cmd.PersistentFlags().BoolVar(&gFlags.testEnvSilent, "test-env-silent", false, "Skips the test environment announcement.")
	cmd.PersistentFlags().BoolVar(&gFlags.disableUsageMetrics, "disable-usage-metrics", false, "Disable usage metrics collection.")
	_ = cmd.PersistentFlags().MarkHidden("sauce-api")
	_ = cmd.PersistentFlags().MarkHidden("runner-version")
	_ = cmd.PersistentFlags().MarkHidden("experiment")

	// Deprecated flags
	sc.Fset.MarkDeprecated("tunnel-id", "please use --tunnel-name instead")
	sc.Fset.MarkDeprecated("tunnel-parent", "please use --tunnel-owner instead")

	sc.BindAll()

	cmd.AddCommand(
		NewCypressCmd(),
		NewEspressoCmd(),
		NewPlaywrightCmd(),
		NewPuppeteerCmd(),
		NewTestcafeCmd(),
		NewXCUITestCmd(),
	)

	return cmd
}

// preRun is a pre-run step that is executed before the main 'run` step. All shared dependencies are initialized here.
func preRun() error {
	println("Running version", version.Version)
	checkForUpdates()
	go awaitGlobalTimeout()

	creds := credentials.Get()
	if !creds.IsValid() {
		color.Red("\nSauceCTL requires a valid Sauce Labs account!\n\n")
		fmt.Println(`Set up your credentials by running:
> saucectl configure`)
		println()
		return fmt.Errorf("no credentials set")
	}

	// TODO Performing the "kind" check in the global run method is necessary for as long as we support the global
	// `saucectl run`, rather than the framework specific `saucectl run {framework}`. After we drop the global run
	// support, the run command does not need to the determine the config type any longer, and each framework should
	// perform this validation on its own.
	d, err := config.Describe(gFlags.cfgFilePath)
	if err != nil {
		return err
	}
	typeDef = d

	tcClient = testcomposer.Client{
		HTTPClient:  &http.Client{Timeout: testComposerTimeout},
		URL:         "", // updated later once region is determined
		Credentials: creds,
	}

	restoClient = resto.New("", creds.Username, creds.AccessKey, 0)

	rdcClient = rdc.New("", creds.Username, creds.AccessKey, rdcTimeout, config.ArtifactDownload{})

	appsClient = *appstore.New("", creds.Username, creds.AccessKey, appStoreTimeout)

	return nil
}

// Run runs the command
func Run(cmd *cobra.Command) (int, error) {
	if typeDef.Kind == cypress.Kind {
		return runCypress(cmd, tcClient, restoClient, appsClient)
	}
	if typeDef.Kind == playwright.Kind {
		return runPlaywright(cmd, tcClient, restoClient, appsClient)
	}
	if typeDef.Kind == testcafe.Kind {
		return runTestcafe(cmd, testcafeFlags{}, tcClient, restoClient, appsClient)
	}
	if typeDef.Kind == puppeteer.Kind {
		return runPuppeteer(cmd, tcClient, restoClient)
	}
	if typeDef.Kind == espresso.Kind {
		return runEspresso(cmd, espressoFlags{}, tcClient, restoClient, rdcClient, appsClient)
	}
	if typeDef.Kind == xcuitest.Kind {
		return runXcuitest(cmd, xcuitestFlags{}, tcClient, restoClient, rdcClient, appsClient)
	}

	return 1, errors.New(msg.UnknownFrameworkConfig)
}

func printTestEnv(testEnv string) {
	if gFlags.testEnvSilent {
		return
	}

	switch testEnv {
	case "docker":
		fmt.Println(msg.DockerLogo)
	case "sauce":
		fmt.Println(msg.SauceLogo)
	}
}

// awaitGlobalTimeout waits for the global timeout event. In case of global timeout event, it attempts to interrupt the
// current process. Should this fail, a hard immediate exit is performed.
func awaitGlobalTimeout() {
	if gFlags.globalTimeout == 0 {
		return
	}

	<-time.After(gFlags.globalTimeout)
	msg.LogGlobalTimeoutShutdown()

	// A timeout for soft shutdown.
	go func() {
		<-time.After(10 * time.Second)
		color.Red("Unable to perform soft shutdown. Exiting immediately...")
		os.Exit(1)
	}()

	// Can't send interrupt signals on windows. A hard exit is our only choice.
	if runtime.GOOS == "windows" {
		os.Exit(1)
	}

	p, err := os.FindProcess(os.Getpid())
	if err == nil {
		_ = p.Signal(syscall.SIGINT)
	}
}

// checkForUpdates check if there is a saucectl update available.
func checkForUpdates() {
	gh := github.Client{
		HTTPClient: &http.Client{Timeout: githubTimeout},
		URL:        "https://api.github.com",
	}

	v, err := gh.HasUpdateAvailable()
	if err != nil {
		return
	}
	if v != "" {
		log.Warn().Msgf("A new version of saucectl is available (%s)", v)
	}
}

func createReporters(c config.Reporters, ntfs config.Notifications, metadata config.Metadata,
	svc slack.Service, framework, env string) []report.Reporter {
	reps := []report.Reporter{
		&captor.Default,
		&table.Reporter{
			Dst: os.Stdout,
		}}

	if c.JUnit.Enabled {
		reps = append(reps, &junit.Reporter{
			Filename: c.JUnit.Filename,
		})
	}

	reps = append(reps, &slack.Reporter{
		Channels:            ntfs.Slack.Channels,
		Framework:           framework,
		Metadata:            metadata,
		TestEnv:             env,
		TestResults:         []report.TestResult{},
		Config:              ntfs,
		Service:             svc,
		DisableUsageMetrics: gFlags.disableUsageMetrics,
	})

	return reps
}

// fullCommandName returns the full command name by concatenating the command names of any parents,
// except the name of the CLI itself.
func fullCommandName(cmd *cobra.Command) string {
	name := ""

	for cmd.Name() != "saucectl" {
		// Prepending, because we are looking up names from the bottom up: cypress < run < saucectl
		// which ends up correctly as 'run cypress' (sans saucectl).
		name = fmt.Sprintf("%s %s", cmd.Name(), name)
		cmd = cmd.Parent()
	}

	return strings.TrimSpace(name)
}
