package run

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/report/buildtable"
	"github.com/saucelabs/saucectl/internal/report/json"
	"github.com/saucelabs/saucectl/internal/report/junit"
	"github.com/saucelabs/saucectl/internal/report/spotlight"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/saucelabs/saucectl/internal/apitest"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/cucumber"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/espresso"
	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/imagerunner"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/notification/slack"
	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/saucelabs/saucectl/internal/puppeteer/replay"
	"github.com/saucelabs/saucectl/internal/report"
	"github.com/saucelabs/saucectl/internal/report/captor"
	"github.com/saucelabs/saucectl/internal/report/github"
	"github.com/saucelabs/saucectl/internal/testcafe"
	"github.com/saucelabs/saucectl/internal/version"
	"github.com/saucelabs/saucectl/internal/xcuitest"
)

var (
	runUse   = "run"
	runShort = "Runs tests on Sauce Labs"

	// General Request Timeouts
	testComposerTimeout = 15 * time.Minute
	webdriverTimeout    = 15 * time.Minute
	rdcTimeout          = 15 * time.Minute
	insightsTimeout     = 10 * time.Second
	iamTimeout          = 10 * time.Second
	apitestingTimeout   = 30 * time.Second
	imgExecTimeout      = 30 * time.Second

	typeDef config.TypeDef

	// ErrEmptySuiteName is thrown when a flag is specified that has a dependency on the --name flag.
	ErrEmptySuiteName = errors.New(msg.EmptyAdhocSuiteName)
)

// gFlags contains all global flags that are set when 'run' is invoked.
var gFlags = globalFlags{}

type globalFlags struct {
	cfgFilePath   string
	selectedSuite string
	testEnvSilent bool
	async         bool
	failFast      bool
	noAutoTagging bool

	globalTimeout   time.Duration
	appStoreTimeout time.Duration
}

// Command creates the `run` command
func Command() *cobra.Command {
	sc := flags.SnakeCharmer{Fmap: map[string]*pflag.Flag{}}

	cmd := &cobra.Command{
		Use:              runUse,
		Short:            runShort,
		SilenceUsage:     true,
		TraverseChildren: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRun()
		},
		Run: func(cmd *cobra.Command, args []string) {
			exitCode, err := Run(cmd)
			if err != nil {
				log.Err(err).Msg("failed to execute run command")
			}
			os.Exit(exitCode)
		},
	}

	sc.Fset = cmd.PersistentFlags()

	defaultCfgPath := filepath.Join(".sauce", "config.yml")
	cmd.PersistentFlags().StringVarP(&gFlags.cfgFilePath, "config", "c", defaultCfgPath, "Specifies which config file to use")
	cmd.PersistentFlags().DurationVarP(&gFlags.globalTimeout, "timeout", "t", 0, "Global timeout that limits how long saucectl can run in total. Supports duration values like '10s', '30m' etc. (default: no timeout)")
	cmd.PersistentFlags().BoolVar(&gFlags.async, "async", false, "Launches tests without waiting for test results")
	cmd.PersistentFlags().BoolVar(&gFlags.failFast, "fail-fast", false, "Stops suites after the first failure")
	cmd.PersistentFlags().DurationVar(&gFlags.appStoreTimeout, "uploadTimeout", 5*time.Minute, "Upload timeout that limits how long saucectl will wait for an upload to finish. Supports duration values like '10s', '30m' etc.")
	cmd.PersistentFlags().DurationVar(&gFlags.appStoreTimeout, "upload-timeout", 5*time.Minute, "Upload timeout that limits how long saucectl will wait for an upload to finish. Supports duration values like '10s', '30m' etc.")
	sc.StringP("region", "r", "sauce::region", "", "The sauce labs region. Options: us-west-1, eu-central-1.")
	sc.StringToStringP("env", "e", "envFlag", map[string]string{}, "Set environment variables, e.g. -e foo=bar. Not supported for RDC or Espresso on virtual devices!")
	sc.Bool("show-console-log", "showConsoleLog", false, "Shows suites console.log locally. By default console.log is only shown on failures.")
	sc.Int("ccy", "sauce::concurrency", 2, "Concurrency specifies how many suites are run at the same time.")
	sc.String("tunnel-name", "sauce::tunnel::name", "", "Sets the sauce-connect tunnel name to be used for the run.")
	sc.String("tunnel-owner", "sauce::tunnel::owner", "", "Sets the sauce-connect tunnel owner to be used for the run.")
	sc.Duration("tunnel-timeout", "sauce::tunnel::timeout", 30*time.Second, "How long to wait for the specified tunnel to be ready. Supports duration values like '10s', '30m' etc.")
	sc.String("runner-version", "runnerVersion", "", "Overrides the automatically determined runner version.")
	sc.String("sauceignore", "sauce::sauceignore", ".sauceignore", "Specifies the path to the .sauceignore file.")
	sc.String("root-dir", "rootDir", ".", "Specifies the project directory. Not applicable to mobile frameworks.")
	sc.StringToString("experiment", "sauce::experiment", map[string]string{}, "Specifies a list of experimental flags and values")
	sc.Bool("dry-run", "dryRun", false, "Simulate a test run without actually running any tests.")
	sc.Int("retries", "sauce::retries", 0, "Retries specifies the number of times to retry a failed suite")
	sc.String("launch-order", "sauce::launchOrder", "", `Launch jobs based on the failure rate. Jobs with the highest failure rate launch first. Supports values: ["fail rate"]`)
	sc.Bool("live-logs", "liveLogs", false, "Display live logs for a running job (supported only by Sauce Orchestrate).")

	// Metadata
	sc.StringSlice("tags", "sauce::metadata::tags", []string{}, "Adds tags to tests")
	sc.String("build", "sauce::metadata::build", "", "Associates tests with a build")

	// Artifacts
	sc.String("artifacts.download.when", "artifacts::download::when", "never", "Specifies when to download test artifacts")
	sc.StringSlice("artifacts.download.match", "artifacts::download::match", []string{}, "Specifies which test artifacts to download")
	sc.String("artifacts.download.directory", "artifacts::download::directory", "", "Specifies the location where to download test artifacts to")
	sc.Bool("artifacts.download.allAttempts", "artifacts::download::allAttempts", false, "Specifies whether to download artifacts for all attempted tests if a test needs to be retried. If false, only the artifacts of the last attempt will be downloaded.")
	sc.Bool("artifacts.cleanup", "artifacts::cleanup", false, "Specifies whether to remove all contents of artifacts directory")

	// Reporters
	sc.Bool("reporters.junit.enabled", "reporters::junit::enabled", false, "Toggle saucectl's own junit reporting on/off. This only affects the reports that saucectl itself generates as a summary of your tests. Each Job in Sauce Labs has an independent report regardless.")
	sc.String("reporters.junit.filename", "reporters::junit::filename", "saucectl-report.xml", "Specifies the report filename.")
	sc.Bool("reporters.json.enabled", "reporters::json::enabled", false, "Toggle saucectl's JSON test result reporting on/off. This only affects the reports that saucectl itself generates as a summary of your tests.")
	sc.String("reporters.json.filename", "reporters::json::filename", "saucectl-report.json", "Specifies the report filename.")
	sc.String("reporters.json.webhookURL", "reporters::json::webhookURL", "", "Specifies the webhook URL. When saucectl test is finished, it'll send a HTTP POST payload to the configured webhook URL.")

	cmd.PersistentFlags().StringVar(&gFlags.selectedSuite, "select-suite", "", "Run specified test suite.")
	cmd.PersistentFlags().BoolVar(&gFlags.testEnvSilent, "test-env-silent", false, "Skips the test environment announcement.")
	cmd.PersistentFlags().BoolVar(&gFlags.noAutoTagging, "no-auto-tagging", false, "Disable the automatic tagging of jobs with metadata, such as CI or GIT information.")

	// Hide undocumented flags that the user does not need to care about.
	_ = cmd.PersistentFlags().MarkHidden("runner-version")
	_ = cmd.PersistentFlags().MarkHidden("experiment")

	// Deprecated flags
	_ = sc.Fset.MarkDeprecated("uploadTimeout", "please use --upload-timeout instead")

	sc.BindAll()

	cmd.AddCommand(
		NewCypressCmd(),
		NewEspressoCmd(),
		NewPlaywrightCmd(),
		NewReplayCmd(),
		NewTestcafeCmd(),
		NewXCUITestCmd(),
		NewCucumberCmd(),
	)

	return cmd
}

// preRun is a pre-run step that is executed before the main 'run` step. All shared dependencies are initialized here.
func preRun() error {
	err := http.CheckProxy()
	if err != nil {
		return fmt.Errorf("invalid HTTP_PROXY value")
	}

	println("Running version", version.Version)
	checkForUpdates()
	go awaitGlobalTimeout()

	creds := credentials.Get()
	if !creds.IsSet() {
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

	return nil
}

// Run runs the command
func Run(cmd *cobra.Command) (int, error) {
	if typeDef.Kind == cypress.Kind {
		return runCypress(cmd, cypressFlags{}, false)
	}
	if typeDef.Kind == playwright.Kind {
		return runPlaywright(cmd, playwrightFlags{}, false)
	}
	if typeDef.Kind == testcafe.Kind {
		return runTestcafe(cmd, testcafeFlags{}, false)
	}
	if typeDef.Kind == replay.Kind {
		return runReplay(cmd, false)
	}
	if typeDef.Kind == espresso.Kind {
		return runEspresso(cmd, espressoFlags{}, false)
	}
	if typeDef.Kind == xcuitest.Kind {
		return runXcuitest(cmd, xcuitestFlags{}, false)
	}
	if typeDef.Kind == apitest.Kind {
		return runApitest(cmd, false)
	}
	if typeDef.Kind == cucumber.Kind {
		return runCucumber(cmd, false)
	}
	if typeDef.Kind == imagerunner.Kind {
		return runImageRunner(cmd)
	}

	msg.LogUnsupportedFramework(typeDef.Kind)
	return 1, errors.New(msg.UnknownFrameworkConfig)
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
	v, err := http.DefaultGitHub.IsUpdateAvailable(version.Version)
	if err != nil {
		return
	}
	if v != "" {
		log.Warn().Msgf("A new version of saucectl is available (%s)", v)
	}
}

func createReporters(c config.Reporters, ntfs config.Notifications, metadata config.Metadata,
	svc slack.Service, framework, env string, async bool) []report.Reporter {
	githubReporter := github.NewJobSummaryReporter()

	reps := []report.Reporter{
		&captor.Default,
		&githubReporter,
	}

	// Running async means that jobs aren't done by the time reports are
	// generated. Therefore, we disable all reporters that depend on the Job
	// results.
	if !async {
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
		if c.Spotlight.Enabled {
			reps = append(reps, &spotlight.Reporter{
				Dst: os.Stdout,
			})
		}
	}

	buildReporter := buildtable.New()
	reps = append(reps, &buildReporter)

	reps = append(reps, &slack.Reporter{
		Channels:    ntfs.Slack.Channels,
		Framework:   framework,
		Metadata:    metadata,
		TestEnv:     env,
		TestResults: []report.TestResult{},
		Config:      ntfs,
		Service:     svc,
	})

	return reps
}

// cleanupArtifacts removes any files in the artifact folder. Does nothing if cleanup is turned off.
func cleanupArtifacts(c config.Artifacts) {
	if !c.Cleanup {
		return
	}

	err := os.RemoveAll(c.Download.Directory)
	if err != nil {
		log.Err(err).Msg("Unable to clean up previous artifacts")
	}
}
