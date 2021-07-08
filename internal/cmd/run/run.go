package run

import (
	"errors"
	"fmt"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/espresso"
	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/saucelabs/saucectl/internal/puppeteer"
	"github.com/saucelabs/saucectl/internal/testcafe"
	"github.com/saucelabs/saucectl/internal/version"
	"github.com/saucelabs/saucectl/internal/xcuitest"
	"github.com/spf13/pflag"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/internal/appstore"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/github"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/rdc"
	"github.com/saucelabs/saucectl/internal/resto"
	"github.com/saucelabs/saucectl/internal/sentry"
	"github.com/saucelabs/saucectl/internal/testcomposer"
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
)

// gFlags contains all global flags that are set when 'run' is invoked.
var gFlags = globalFlags{}

type globalFlags struct {
	cfgFilePath   string
	globalTimeout time.Duration
	sauceAPI      string
	selectedSuite string
	testEnvSilent bool
	testEnv       string
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
	sc.StringP("region", "r", "sauce.region", "us-west-1", "The sauce labs region.")
	sc.StringToStringP("env", "e", "env", map[string]string{}, "Set environment variables, e.g. -e foo=bar.")
	// FIXME sauce-api is actually not implemented, but probably should
	cmd.PersistentFlags().StringVar(&gFlags.sauceAPI, "sauce-api", "", "Overrides the region specific sauce API URL. (e.g. https://api.us-west-1.saucelabs.com)")
	cmd.PersistentFlags().StringVar(&gFlags.selectedSuite, "select-suite", "", "Run specified test suite.")
	cmd.PersistentFlags().BoolVar(&gFlags.testEnvSilent, "test-env-silent", false, "Skips the test environment announcement.")
	cmd.PersistentFlags().StringVar(&gFlags.testEnv, "test-env", "", "Specifies the environment in which the tests should run. Choice: docker|sauce.")
	sc.Bool("show-console-log", "ShowConsoleLog", false, "Shows suites console.log locally. By default console.log is only shown on failures.")
	sc.Int("ccy", "sauce.concurrency", 2, "Concurrency specifies how many suites are run at the same time.")
	sc.String("tunnel-id", "sauce.tunnel.id", "", "Sets the sauce-connect tunnel ID to be used for the run.")
	sc.String("tunnel-parent", "sauce.tunnel.parent", "", "Sets the sauce-connect tunnel parent to be used for the run.")
	sc.String("runner-version", "runnerVersion", "", "Overrides the automatically determined runner version.")
	sc.String("sauceignore", "sauce.sauceignore", ".sauceignore", "Specifies the path to the .sauceignore file.")
	sc.StringToString("experiment", "sauce.experiment", map[string]string{}, "Specifies a list of experimental flags and values")
	sc.Bool("dry-run", "dryRun", false, "Simulate a test run without actually running any tests.")

	// Metadata
	sc.StringSlice("tags", "sauce.metadata.tags", []string{}, "Adds tags to tests")
	sc.String("build", "sauce.metadata.build", "", "Associates tests with a build")

	// Artifacts
	sc.String("artifacts.download.when", "artifacts.download.when", "never", "Specifies when to download test artifacts")
	sc.StringSlice("artifacts.download.match", "artifacts.download.match", []string{}, "Specifies which test artifacts to download")
	sc.String("artifacts.download.directory", "artifacts.download.directory", "", "Specifies the location where to download test artifacts to")

	cmd.Flags().MarkDeprecated("test-env", "please set mode in config file")

	// Hide undocumented flags that the user does not need to care about.
	_ = cmd.PersistentFlags().MarkHidden("sauce-api")
	_ = cmd.PersistentFlags().MarkHidden("runner-version")
	_ = cmd.PersistentFlags().MarkHidden("experiment")

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

	restoClient = resto.Client{
		HTTPClient: &http.Client{Timeout: restoTimeout},
		URL:        "", // updated later once region is determined
		Username:   creds.Username,
		AccessKey:  creds.AccessKey,
	}

	rdcClient = rdc.Client{
		HTTPClient: &http.Client{
			Timeout: rdcTimeout,
		},
		Username:  creds.Username,
		AccessKey: creds.AccessKey,
	}

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

	return 1, errors.New("unknown framework configuration")
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

func applyGlobalFlags(cmd *cobra.Command, sauce *config.SauceConfig, arti *config.Artifacts) {
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
