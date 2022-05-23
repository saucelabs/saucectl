package run

import (
	"errors"
	"fmt"
	"github.com/saucelabs/saucectl/internal/webdriver"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/saucelabs/saucectl/internal/puppeteer/replay"

	"github.com/fatih/color"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/saucelabs/saucectl/internal/appstore"
	"github.com/saucelabs/saucectl/internal/backtrace"
	"github.com/saucelabs/saucectl/internal/build"
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
	"github.com/saucelabs/saucectl/internal/report/buildtable"
	"github.com/saucelabs/saucectl/internal/report/captor"
	"github.com/saucelabs/saucectl/internal/report/json"
	"github.com/saucelabs/saucectl/internal/resto"
	"github.com/saucelabs/saucectl/internal/testcafe"
	"github.com/saucelabs/saucectl/internal/testcomposer"
	"github.com/saucelabs/saucectl/internal/version"
	"github.com/saucelabs/saucectl/internal/xcuitest"
)

var (
	runUse   = "run"
	runShort = "Runs tests on Sauce Labs"

	// General Request Timeouts
	testComposerTimeout = 15 * time.Minute
	rdcTimeout          = 15 * time.Second
	githubTimeout       = 2 * time.Second

	typeDef config.TypeDef

	testcompClient  testcomposer.Client
	webdriverClient webdriver.Client
	restoClient     resto.Client
	appsClient      appstore.AppStore
	rdcClient       rdc.Client

	// ErrEmptySuiteName is thrown when a flag is specified that has a dependency on the --name flag.
	ErrEmptySuiteName = errors.New(msg.EmptyAdhocSuiteName)
)

// gFlags contains all global flags that are set when 'run' is invoked.
var gFlags = globalFlags{}

type globalFlags struct {
	cfgFilePath         string
	globalTimeout       time.Duration
	selectedSuite       string
	testEnvSilent       bool
	disableUsageMetrics bool
	async               bool
	failFast            bool
	appStoreTimeout     time.Duration
	noAutoTagging       bool
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
				backtrace.Report(err, map[string]interface{}{
					"username": credentials.Get().Username,
				}, gFlags.cfgFilePath)
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
	cmd.PersistentFlags().DurationVar(&gFlags.appStoreTimeout, "uploadTimeout", 5*time.Minute, "Upload timeout that limits how long saucectl will wait for an upload to finish. Supports duration values like '10s' '30m' etc. (default: 5m)")
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
	sc.Bool("reporters.json.enabled", "reporters::json::enabled", false, "Toggle saucectl's JSON test result reporting on/off. This only affects the reports that saucectl itself generates as a summary of your tests.")
	sc.String("reporters.json.filename", "reporters::json::filename", "saucectl-report.json", "Specifies the report filename.")
	sc.String("reporters.json.webhookURL", "reporters::json::webhookURL", "", "Specifies the webhook URL. When saucectl test is finished, it'll send a HTTP POST payload to the configured webhook URL.")

	cmd.PersistentFlags().StringVar(&gFlags.selectedSuite, "select-suite", "", "Run specified test suite.")
	cmd.PersistentFlags().BoolVar(&gFlags.testEnvSilent, "test-env-silent", false, "Skips the test environment announcement.")
	cmd.PersistentFlags().BoolVar(&gFlags.disableUsageMetrics, "disable-usage-metrics", false, "Disable usage metrics collection.")
	cmd.PersistentFlags().BoolVar(&gFlags.noAutoTagging, "no-auto-tagging", false, "Disable the automatic tagging of jobs with metadata, such as CI or GIT information.")

	// Hide undocumented flags that the user does not need to care about.
	_ = cmd.PersistentFlags().MarkHidden("runner-version")
	_ = cmd.PersistentFlags().MarkHidden("experiment")

	// Deprecated flags
	_ = sc.Fset.MarkDeprecated("tunnel-id", "please use --tunnel-name instead")
	_ = sc.Fset.MarkDeprecated("tunnel-parent", "please use --tunnel-owner instead")

	sc.BindAll()

	cmd.AddCommand(
		NewCypressCmd(),
		NewEspressoCmd(),
		NewPlaywrightCmd(),
		NewPuppeteerCmd(),
		NewReplayCmd(),
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

	d, err := config.Describe(gFlags.cfgFilePath)
	if err != nil {
		return err
	}
	typeDef = d

	testcompClient = testcomposer.Client{
		HTTPClient:  &http.Client{Timeout: testComposerTimeout},
		URL:         "", // updated later once region is determined
		Credentials: creds,
	}

	webdriverClient = webdriver.Client{
		HTTPClient:  &http.Client{Timeout: testComposerTimeout},
		URL:         "", // updated later once region is determined
		Credentials: creds,
	}

	restoClient = resto.New("", creds.Username, creds.AccessKey, 0)

	rdcClient = rdc.New("", creds.Username, creds.AccessKey, rdcTimeout, config.ArtifactDownload{})

	appsClient = *appstore.New("", creds.Username, creds.AccessKey, gFlags.appStoreTimeout)

	return nil
}

// Run runs the command
func Run(cmd *cobra.Command) (int, error) {
	if typeDef.Kind == cypress.Kind {
		return runCypress(cmd)
	}
	if typeDef.Kind == playwright.Kind {
		return runPlaywright(cmd)
	}
	if typeDef.Kind == testcafe.Kind {
		return runTestcafe(cmd, testcafeFlags{})
	}
	if typeDef.Kind == puppeteer.Kind {
		return runPuppeteer(cmd)
	}
	if typeDef.Kind == replay.Kind {
		return runReplay(cmd)
	}
	if typeDef.Kind == espresso.Kind {
		return runEspresso(cmd, espressoFlags{})
	}
	if typeDef.Kind == xcuitest.Kind {
		return runXcuitest(cmd, xcuitestFlags{})
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
		fmt.Println()
	case "sauce":
		fmt.Println(msg.SauceLogo)
		fmt.Println()
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
	svc slack.Service, buildReader build.Reader, framework, env string) []report.Reporter {
	buildReporter := buildtable.New(buildReader)

	reps := []report.Reporter{
		&captor.Default,
		&buildReporter,
	}

	if c.JUnit.Enabled {
		reps = append(reps, &junit.Reporter{
			Filename: c.JUnit.Filename,
		})
	}

	if c.JSON.Enabled {
		reps = append(reps, &json.Reporter{
			WebhookURL: c.JSON.WebhookURL,
			Filename:   c.JSON.Filename,
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
