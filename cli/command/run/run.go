package run

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/version"
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

	defaultLogFir      = "<cwd>/logs"
	defaultRegion      = "us-west-1"
	defaultSauceignore = ".sauceignore"

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
	cfgFilePath    string
	globalTimeout  time.Duration
	regionFlag     string
	env            map[string]string
	sauceAPI       string
	suiteName      string
	testEnvSilent  bool
	testEnv        string
	showConsoleLog bool
	concurrency    int
	tunnelID       string
	tunnelParent   string
	runnerVersion  string
	sauceignore    string
	experiments    map[string]string
	dryRun         bool
	tags           []string
	build          string
	artifacts      struct {
		download struct {
			when      string
			match     []string
			directory string
		}
	}
}

// Command creates the `run` command
func Command(cli *command.SauceCtlCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:              runUse,
		Short:            runShort,
		TraverseChildren: true,
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			return persistentPreRun()
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

	defaultCfgPath := filepath.Join(".sauce", "config.yml")
	cmd.PersistentFlags().StringVarP(&gFlags.cfgFilePath, "config", "c", defaultCfgPath, "Specifies which config file to use")
	cmd.PersistentFlags().DurationVarP(&gFlags.globalTimeout, "timeout", "t", 0, "Global timeout that limits how long saucectl can run in total. Supports duration values like '10s', '30m' etc. (default: no timeout)")
	cmd.PersistentFlags().StringVarP(&gFlags.regionFlag, "region", "r", "", "The sauce labs region. (default: us-west-1)")
	cmd.PersistentFlags().StringToStringVarP(&gFlags.env, "env", "e", map[string]string{}, "Set environment variables, e.g. -e foo=bar.")
	cmd.PersistentFlags().StringVar(&gFlags.sauceAPI, "sauce-api", "", "Overrides the region specific sauce API URL. (e.g. https://api.us-west-1.saucelabs.com)")
	cmd.PersistentFlags().StringVar(&gFlags.suiteName, "suite", "", "Run specified test suite.")
	cmd.PersistentFlags().BoolVar(&gFlags.testEnvSilent, "test-env-silent", false, "Skips the test environment announcement.")
	cmd.PersistentFlags().StringVar(&gFlags.testEnv, "test-env", "", "Specifies the environment in which the tests should run. Choice: docker|sauce.")
	cmd.PersistentFlags().BoolVarP(&gFlags.showConsoleLog, "show-console-log", "", false, "Shows suites console.log locally. By default console.log is only shown on failures.")
	cmd.PersistentFlags().IntVar(&gFlags.concurrency, "ccy", 2, "Concurrency specifies how many suites are run at the same time.")
	cmd.PersistentFlags().StringVar(&gFlags.tunnelID, "tunnel-id", "", "Sets the sauce-connect tunnel ID to be used for the run.")
	cmd.PersistentFlags().StringVar(&gFlags.tunnelParent, "tunnel-parent", "", "Sets the sauce-connect tunnel parent to be used for the run.")
	cmd.PersistentFlags().StringVar(&gFlags.runnerVersion, "runner-version", "", "Overrides the automatically determined runner version.")
	cmd.PersistentFlags().StringVar(&gFlags.sauceignore, "sauceignore", "", "Specifies the path to the .sauceignore file.")
	cmd.PersistentFlags().StringToStringVar(&gFlags.experiments, "experiment", map[string]string{}, "Specifies a list of experimental flags and values")
	cmd.PersistentFlags().BoolVarP(&gFlags.dryRun, "dry-run", "", false, "Simulate a test run without actually running any tests.")

	// Metadata
	cmd.PersistentFlags().StringSliceVar(&gFlags.tags, "tags", []string{}, "Adds tags to tests")
	cmd.PersistentFlags().StringVar(&gFlags.build, "build", "", "Associates tests with a build")

	// Artifacts
	cmd.PersistentFlags().StringVar(&gFlags.artifacts.download.when, "artifacts.download.when", "never", "Specifies when to download test artifacts")
	cmd.PersistentFlags().StringSliceVar(&gFlags.artifacts.download.match, "artifacts.download.match", []string{}, "Specifies which test artifacts to download")
	cmd.PersistentFlags().StringVar(&gFlags.artifacts.download.directory, "artifacts.download.directory", "", "Specifies the location where to download test artifacts to")

	cmd.Flags().MarkDeprecated("test-env", "please set mode in config file")

	// Hide undocumented flags that the user does not need to care about.
	_ = cmd.PersistentFlags().MarkHidden("sauce-api")
	_ = cmd.PersistentFlags().MarkHidden("runner-version")
	_ = cmd.PersistentFlags().MarkHidden("experiment")

	cmd.AddCommand(NewEspressoCmd(cli), NewXCUITestCmd(cli))

	return cmd
}

// persistentPreRun is a pre-run step that is executed for all subcommands before the main 'run` step.
// All shared dependencies are initialized here.
func persistentPreRun() error {
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
	// TODO switch statement with pre-constructed type definition structs?
	if typeDef.Kind == config.KindCypress && typeDef.APIVersion == config.VersionV1Alpha {
		return runCypress(cmd, tcClient, restoClient, &appsClient)
	}
	if typeDef.Kind == config.KindPlaywright && typeDef.APIVersion == config.VersionV1Alpha {
		return runPlaywright(cmd, tcClient, restoClient, &appsClient)
	}
	if typeDef.Kind == config.KindTestcafe && typeDef.APIVersion == config.VersionV1Alpha {
		return runTestcafe(cmd, tcClient, restoClient, &appsClient)
	}
	if typeDef.Kind == config.KindPuppeteer && typeDef.APIVersion == config.VersionV1Alpha {
		return runPuppeteer(cmd, tcClient, restoClient)
	}
	if typeDef.Kind == config.KindEspresso && typeDef.APIVersion == config.VersionV1Alpha {
		return runEspresso(cmd, tcClient, restoClient, rdcClient, appsClient)
	}
	if typeDef.Kind == config.KindXcuitest && typeDef.APIVersion == config.VersionV1Alpha {
		return runXcuitest(cmd, tcClient, restoClient, rdcClient, appsClient)
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

func applyDefaultValues(sauce *config.SauceConfig) {
	if sauce.Region == "" {
		sauce.Region = defaultRegion
	}

	if sauce.Sauceignore == "" {
		sauce.Sauceignore = defaultSauceignore
	}
}

func overrideCliParameters(cmd *cobra.Command, sauce *config.SauceConfig, arti *config.Artifacts) {
	if cmd.Flags().Lookup("region").Changed {
		sauce.Region = gFlags.regionFlag
	}
	if cmd.Flags().Lookup("ccy").Changed {
		sauce.Concurrency = gFlags.concurrency
	}
	if cmd.Flags().Lookup("tunnel-id").Changed {
		sauce.Tunnel.ID = gFlags.tunnelID
	}
	if cmd.Flags().Lookup("tunnel-parent").Changed {
		sauce.Tunnel.Parent = gFlags.tunnelParent
	}
	if cmd.Flags().Lookup("sauceignore").Changed {
		sauce.Sauceignore = gFlags.sauceignore
	}
	if cmd.Flags().Lookup("experiment").Changed {
		sauce.Experiments = gFlags.experiments
	}
	if gFlags.build != "" {
		sauce.Metadata.Build = gFlags.build
	}
	if len(gFlags.tags) != 0 {
		sauce.Metadata.Tags = gFlags.tags
	}
	if cmd.Flags().Lookup("artifacts.download.when").Changed {
		arti.Download.When = config.When(gFlags.artifacts.download.when)
	}
	if len(gFlags.artifacts.download.match) != 0 {
		arti.Download.Match = gFlags.artifacts.download.match
	}
	if gFlags.artifacts.download.directory != "" {
		arti.Download.Directory = gFlags.artifacts.download.directory
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
