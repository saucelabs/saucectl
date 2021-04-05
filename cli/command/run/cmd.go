package run

import (
	"errors"
	"fmt"
	"github.com/saucelabs/saucectl/cli/version"
	"github.com/saucelabs/saucectl/internal/espresso"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/puppeteer"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/internal/appstore"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/docker"
	serrors "github.com/saucelabs/saucectl/internal/errors"
	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/resto"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/testcafe"
	"github.com/saucelabs/saucectl/internal/testcomposer"
	"github.com/spf13/cobra"
)

var (
	runUse     = "run ./.sauce/config.yaml"
	runShort   = "Run a test on Sauce Labs"
	runLong    = `Some long description`
	runExample = "saucectl run ./.sauce/config.yaml"

	defaultLogFir      = "<cwd>/logs"
	defaultTimeout     = 60
	defaultRegion      = "us-west-1"
	defaultSauceignore = ".sauceignore"

	cfgFilePath    string
	cfgLogDir      string
	testTimeout    int
	regionFlag     string
	env            map[string]string
	parallel       bool
	ciBuildID      string
	sauceAPI       string
	suiteName      string
	testEnv        string
	testEnvSilent  bool
	showConsoleLog bool
	concurrency    int
	tunnelID       string
	tunnelParent   string
	runnerVersion  string
	sauceignore    string
	experiments    map[string]string
	dryRun         bool

	// General Request Timeouts
	appStoreTimeout     = 300 * time.Second
	testComposerTimeout = 300 * time.Second
	restoTimeout        = 60 * time.Second
)

// Command creates the `run` command
func Command(cli *command.SauceCtlCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     runUse,
		Short:   runShort,
		Long:    runLong,
		Example: runExample,
		Run: func(cmd *cobra.Command, args []string) {
			exitCode, err := Run(cmd, cli, args)
			if err != nil {
				log.Err(err).Msg("failed to execute run command")
				serrors.HandleAndFlush(err)
			}
			os.Exit(exitCode)
		},
	}

	defaultCfgPath := filepath.Join(".sauce", "config.yml")
	cmd.Flags().StringVarP(&cfgFilePath, "config", "c", defaultCfgPath, "config file, e.g. -c ./.sauce/config.yaml")
	cmd.Flags().StringVarP(&cfgLogDir, "logDir", "l", defaultLogFir, "log path")
	cmd.Flags().IntVarP(&testTimeout, "timeout", "t", 0, "test timeout in seconds (default: 60sec)")
	cmd.Flags().StringVarP(&regionFlag, "region", "r", "", "The sauce labs region. (default: us-west-1)")
	cmd.Flags().StringToStringVarP(&env, "env", "e", map[string]string{}, "Set environment variables, e.g. -e foo=bar.")
	cmd.Flags().BoolVarP(&parallel, "parallel", "p", false, "Run tests in parallel across multiple machines.")
	cmd.Flags().StringVar(&ciBuildID, "ci-build-id", "", "Overrides the CI dependent build ID.")
	cmd.Flags().StringVar(&sauceAPI, "sauce-api", "", "Overrides the region specific sauce API URL. (e.g. https://api.us-west-1.saucelabs.com)")
	cmd.Flags().StringVar(&suiteName, "suite", "", "Run specified test suite.")
	cmd.Flags().BoolVar(&testEnvSilent, "test-env-silent", false, "Skips the test environment announcement.")
	cmd.Flags().StringVar(&testEnv, "test-env", "sauce", "Specifies the environment in which the tests should run. Choice: docker|sauce.")
	cmd.Flags().BoolVarP(&showConsoleLog, "show-console-log", "", false, "Shows suites console.log locally. By default console.log is only shown on failures.")
	cmd.Flags().IntVar(&concurrency, "ccy", 2, "Concurrency specifies how many suites are run at the same time.")
	cmd.Flags().StringVar(&tunnelID, "tunnel-id", "", "Sets the sauce-connect tunnel ID to be used for the run.")
	cmd.Flags().StringVar(&tunnelParent, "tunnel-parent", "", "Sets the sauce-connect tunnel parent to be used for the run.")
	cmd.Flags().StringVar(&runnerVersion, "runner-version", "", "Overrides the automatically determined runner version.")
	cmd.Flags().StringVar(&sauceignore, "sauceignore", "", "Specifies the path to the .sauceignore file.")
	cmd.Flags().StringToStringVar(&experiments, "experiment", map[string]string{}, "Specifies a list of experimental flags and values")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "", false, "Simulate a test run without actually running any tests.")

	// Hide undocumented flags that the user does not need to care about.
	_ = cmd.Flags().MarkHidden("sauce-api")
	_ = cmd.Flags().MarkHidden("runner-version")
	_ = cmd.Flags().MarkHidden("experiment")

	// Hide documented flags that aren't fully released yet or WIP.
	_ = cmd.Flags().MarkHidden("parallel")    // WIP.
	_ = cmd.Flags().MarkHidden("ci-build-id") // Related to 'parallel'. WIP.

	return cmd
}

// Run runs the command
func Run(cmd *cobra.Command, cli *command.SauceCtlCli, args []string) (int, error) {
	printTestEnv()
	log.Info().Msgf("Running version %s", version.Version)

	// Todo(Christian) write argument parser/validator
	if cfgLogDir == defaultLogFir {
		pwd, _ := os.Getwd()
		cfgLogDir = filepath.Join(pwd, "logs")
	}
	cli.LogDir = cfgLogDir
	log.Info().Str("config", cfgFilePath).Msg("Reading config file")

	d, err := config.Describe(cfgFilePath)
	if err != nil {
		return 1, err
	}

	// TODO switch statement with pre-constructed type definition structs?
	if d.Kind == config.KindCypress && d.APIVersion == config.VersionV1Alpha {
		return runCypress(cmd)
	}
	if d.Kind == config.KindPlaywright && d.APIVersion == config.VersionV1Alpha {
		return runPlaywright(cmd)
	}
	if d.Kind == config.KindTestcafe && d.APIVersion == config.VersionV1Alpha {
		return runTestcafe(cmd)
	}
	if d.Kind == config.KindPuppeteer && d.APIVersion == config.VersionV1Alpha {
		return runPuppeteer(cmd)
	}
	if d.Kind == config.KindEspresso && d.APIVersion == config.VersionV1Alpha {
		return runEspresso(cmd)
	}

	return 1, errors.New("unknown framework configuration")
}

func printTestEnv() {
	if testEnvSilent {
		return
	}

	switch testEnv {
	case "docker":
		fmt.Println(msg.DockerLogo)
	case "sauce":
		fmt.Println(msg.SauceLogo)
	}
}

func runCypress(cmd *cobra.Command) (int, error) {
	p, err := cypress.FromFile(cfgFilePath)
	if err != nil {
		return 1, err
	}

	p.Sauce.Metadata.ExpandEnv()
	applyDefaultValues(&p.Sauce)
	overrideCliParameters(cmd, &p.Sauce)

	// Merge env from CLI args and job config. CLI args take precedence.
	for k, v := range env {
		for _, s := range p.Suites {
			if s.Config.Env == nil {
				s.Config.Env = map[string]string{}
			}
			s.Config.Env[k] = v
		}
	}

	if showConsoleLog {
		p.ShowConsoleLog = true
	}
	if runnerVersion != "" {
		p.RunnerVersion = runnerVersion
	}
	if dryRun {
		p.DryRun = true
	}

	if cmd.Flags().Lookup("suite").Changed {
		if err := filterCypressSuite(&p); err != nil {
			return 1, err
		}
	}

	if err := cypress.Validate(p); err != nil {
		return 1, err
	}

	creds := credentials.Get()
	if creds == nil {
		return 1, errors.New("no sauce credentials set")
	}

	regio := region.FromString(p.Sauce.Region)
	if regio == region.None {
		log.Error().Str("region", regionFlag).Msg("Unable to determine sauce region.")
		return 1, errors.New("no sauce region set")
	}

	tc := testcomposer.Client{
		HTTPClient:  &http.Client{Timeout: testComposerTimeout},
		URL:         regio.APIBaseURL(),
		Credentials: *creds,
	}

	switch testEnv {
	case "docker":
		return runCypressInDocker(p, tc)
	case "sauce":
		return runCypressInSauce(p, regio, creds, tc)
	default:
		return 1, errors.New("unsupported test environment")
	}
}

func runCypressInDocker(p cypress.Project, testco testcomposer.Client) (int, error) {
	log.Info().Msg("Running Cypress in Docker")

	cd, err := docker.NewCypress(p, &testco)
	if err != nil {
		return 1, err
	}
	return cd.RunProject()
}

func runCypressInSauce(p cypress.Project, regio region.Region, creds *credentials.Credentials, testco testcomposer.Client) (int, error) {
	log.Info().Msg("Running Cypress in Sauce Labs")

	s := appstore.New(regio.APIBaseURL(), creds.Username, creds.AccessKey, appStoreTimeout)

	rsto := resto.Client{
		HTTPClient: &http.Client{Timeout: restoTimeout},
		URL:        regio.APIBaseURL(),
		Username:   creds.Username,
		AccessKey:  creds.AccessKey,
	}

	r := saucecloud.CypressRunner{
		Project: p,
		CloudRunner: saucecloud.CloudRunner{
			ProjectUploader: s,
			JobStarter:      &testco,
			JobReader:       &rsto,
			JobStopper:      &rsto,
			CCYReader:       &rsto,
			TunnelService:   &rsto,
			Region:          regio,
			ShowConsoleLog:  p.ShowConsoleLog,
		},
	}
	return r.RunProject()
}

func runPlaywright(cmd *cobra.Command) (int, error) {
	p, err := playwright.FromFile(cfgFilePath)
	if err != nil {
		return 1, err
	}

	p.Sauce.Metadata.ExpandEnv()
	applyDefaultValues(&p.Sauce)
	overrideCliParameters(cmd, &p.Sauce)

	// Merge env from CLI args and job config. CLI args take precedence.
	for k, v := range env {
		for _, s := range p.Suites {
			if s.Env == nil {
				s.Env = map[string]string{}
			}
			s.Env[k] = v
		}
	}

	if showConsoleLog {
		p.ShowConsoleLog = true
	}
	if runnerVersion != "" {
		p.RunnerVersion = runnerVersion
	}
	if dryRun {
		p.DryRun = true
	}

	if cmd.Flags().Lookup("suite").Changed {
		if err := filterPlaywrightSuite(&p); err != nil {
			return 1, err
		}
	}

	creds := credentials.Get()
	if creds == nil {
		return 1, errors.New("no sauce credentials set")
	}

	regio := region.FromString(p.Sauce.Region)
	if regio == region.None {
		log.Error().Str("region", regionFlag).Msg("Unable to determine sauce region.")
		return 1, errors.New("no sauce region set")
	}

	tc := testcomposer.Client{
		HTTPClient:  &http.Client{Timeout: testComposerTimeout},
		URL:         regio.APIBaseURL(),
		Credentials: *creds,
	}

	switch testEnv {
	case "docker":
		return runPlaywrightInDocker(p, tc)
	case "sauce":
		return runPlaywrightInSauce(p, regio, creds, tc)
	default:
		return 1, errors.New("unsupported test environment")
	}
}

func runPlaywrightInDocker(p playwright.Project, testco testcomposer.Client) (int, error) {
	log.Info().Msg("Running Playwright in Docker")

	cd, err := docker.NewPlaywright(p, &testco)
	if err != nil {
		return 1, err
	}
	return cd.RunProject()
}

func runPlaywrightInSauce(p playwright.Project, regio region.Region, creds *credentials.Credentials, testco testcomposer.Client) (int, error) {
	log.Info().Msg("Running Playwright in Sauce Labs")

	s := appstore.New(regio.APIBaseURL(), creds.Username, creds.AccessKey, appStoreTimeout)

	rsto := resto.Client{
		HTTPClient: &http.Client{Timeout: restoTimeout},
		URL:        regio.APIBaseURL(),
		Username:   creds.Username,
		AccessKey:  creds.AccessKey,
	}

	r := saucecloud.PlaywrightRunner{
		Project: p,
		CloudRunner: saucecloud.CloudRunner{
			ProjectUploader: s,
			JobStarter:      &testco,
			JobReader:       &rsto,
			JobStopper:      &rsto,
			CCYReader:       &rsto,
			TunnelService:   &rsto,
			Region:          regio,
			ShowConsoleLog:  p.ShowConsoleLog,
		},
	}
	return r.RunProject()
}

func runTestcafe(cmd *cobra.Command) (int, error) {
	p, err := testcafe.FromFile(cfgFilePath)
	if err != nil {
		return 1, err
	}
	p.Sauce.Metadata.ExpandEnv()
	applyDefaultValues(&p.Sauce)
	overrideCliParameters(cmd, &p.Sauce)

	for k, v := range env {
		for _, s := range p.Suites {
			if s.Env == nil {
				s.Env = map[string]string{}
			}
			s.Env[k] = v
		}
	}

	if showConsoleLog {
		p.ShowConsoleLog = true
	}
	if runnerVersion != "" {
		p.RunnerVersion = runnerVersion
	}
	if dryRun {
		p.DryRun = true
	}

	if cmd.Flags().Lookup("suite").Changed {
		if err := filterTestcafeSuite(&p); err != nil {
			return 1, err
		}
	}
	creds := credentials.Get()
	if creds == nil {
		return 1, errors.New("no sauce credentials set")
	}

	regio := region.FromString(p.Sauce.Region)
	if regio == region.None {
		log.Error().Str("region", regionFlag).Msg("Unable to determine sauce region.")
		return 1, errors.New("no sauce region set")
	}

	tc := testcomposer.Client{
		HTTPClient:  &http.Client{Timeout: testComposerTimeout},
		URL:         regio.APIBaseURL(),
		Credentials: *creds,
	}

	switch testEnv {
	case "docker":
		return runTestcafeInDocker(p, tc)
	case "sauce":
		return runTestcafeInCloud(p, regio, creds, tc)
	default:
		return 1, errors.New("unsupported test enviornment")
	}
}

func runTestcafeInDocker(p testcafe.Project, testco testcomposer.Client) (int, error) {
	log.Info().Msg("Running Testcafe in Docker")

	cd, err := docker.NewTestcafe(p, &testco)
	if err != nil {
		return 1, err
	}
	return cd.RunProject()
}

func runTestcafeInCloud(p testcafe.Project, regio region.Region, creds *credentials.Credentials, testco testcomposer.Client) (int, error) {
	log.Info().Msg("Running Testcafe in Sauce Labs")

	s := appstore.New(regio.APIBaseURL(), creds.Username, creds.AccessKey, appStoreTimeout)

	rsto := resto.Client{
		HTTPClient: &http.Client{Timeout: restoTimeout},
		URL:        regio.APIBaseURL(),
		Username:   creds.Username,
		AccessKey:  creds.AccessKey,
	}

	r := saucecloud.TestcafeRunner{
		Project: p,
		CloudRunner: saucecloud.CloudRunner{
			ProjectUploader: s,
			JobStarter:      &testco,
			JobReader:       &rsto,
			JobStopper:      &rsto,
			CCYReader:       &rsto,
			TunnelService:   &rsto,
			Region:          regio,
			ShowConsoleLog:  p.ShowConsoleLog,
		},
	}
	return r.RunProject()
}

func runEspresso(cmd *cobra.Command) (int, error) {
	p, err := espresso.FromFile(cfgFilePath)
	if err != nil {
		return 1, err
	}
	p.Sauce.Metadata.ExpandEnv()
	applyDefaultValues(&p.Sauce)
	overrideCliParameters(cmd, &p.Sauce)

	// TODO - add dry-run mode
	creds := credentials.Get()
	if creds == nil {
		return 1, errors.New("no sauce credentials set")
	}

	regio := region.FromString(p.Sauce.Region)
	if regio == region.None {
		log.Error().Str("region", regionFlag).Msg("Unable to determine sauce region.")
		return 1, errors.New("no sauce region set")
	}

	err = espresso.Validate(p)
	if err != nil {
		return 1, err
	}

	tc := testcomposer.Client{
		HTTPClient:  &http.Client{Timeout: testComposerTimeout},
		URL:         regio.APIBaseURL(),
		Credentials: *creds,
	}

	switch testEnv {
	case "sauce":
		return runEspressoInCloud(p, regio, creds, tc)
	default:
		return 1, fmt.Errorf("unsupported test environment for espresso: %s", testEnv)
	}
}

func runEspressoInCloud(p espresso.Project, regio region.Region, creds *credentials.Credentials, testco testcomposer.Client) (int, error) {
	log.Info().Msg("Running Espresso in Sauce Labs")

	s := appstore.New(regio.APIBaseURL(), creds.Username, creds.AccessKey, appStoreTimeout)

	rsto := resto.Client{
		HTTPClient: &http.Client{Timeout: restoTimeout},
		URL:        regio.APIBaseURL(),
		Username:   creds.Username,
		AccessKey:  creds.AccessKey,
	}

	r := saucecloud.EspressoRunner{
		Project: p,
		CloudRunner: saucecloud.CloudRunner{
			ProjectUploader: s,
			JobStarter:      &testco,
			JobReader:       &rsto,
			JobStopper:      &rsto,
			CCYReader:       &rsto,
			TunnelService:   &rsto,
			Region:          regio,
			ShowConsoleLog:  p.ShowConsoleLog,
		},
	}
	return r.RunProject()
}

func runPuppeteer(cmd *cobra.Command) (int, error) {
	p, err := puppeteer.FromFile(cfgFilePath)
	if err != nil {
		return 1, err
	}
	p.Sauce.Metadata.ExpandEnv()
	applyDefaultValues(&p.Sauce)
	overrideCliParameters(cmd, &p.Sauce)

	for k, v := range env {
		for _, s := range p.Suites {
			if s.Env == nil {
				s.Env = map[string]string{}
			}
			s.Env[k] = v
		}
	}

	if showConsoleLog {
		p.ShowConsoleLog = true
	}

	if dryRun {
		p.DryRun = true
	}

	if cmd.Flags().Lookup("suite").Changed {
		if err := filterPuppeteerSuite(&p); err != nil {
			return 1, err
		}
	}
	creds := credentials.Get()
	if creds == nil {
		return 1, errors.New("no sauce credentials set")
	}

	regio := region.FromString(p.Sauce.Region)
	if regio == region.None {
		log.Error().Str("region", regionFlag).Msg("Unable to determine sauce region.")
		return 1, errors.New("no sauce region set")
	}

	tc := testcomposer.Client{
		HTTPClient:  &http.Client{Timeout: testComposerTimeout},
		URL:         regio.APIBaseURL(),
		Credentials: *creds,
	}

	switch testEnv {
	case "docker":
		return runPuppeteerInDocker(p, tc)
	default:
		return 1, errors.New("unsupported test enviornment")
	}
}

func runPuppeteerInDocker(p puppeteer.Project, testco testcomposer.Client) (int, error) {
	log.Info().Msg("Running puppeteer in Docker")

	cd, err := docker.NewPuppeteer(p, &testco)
	if err != nil {
		return 1, err
	}
	return cd.RunProject()
}

func apiBaseURL(r region.Region) string {
	// Check for overrides.
	if sauceAPI != "" {
		return sauceAPI
	}

	return r.APIBaseURL()
}

func filterCypressSuite(c *cypress.Project) error {
	for _, s := range c.Suites {
		if s.Name == suiteName {
			c.Suites = []cypress.Suite{s}
			return nil
		}
	}
	return fmt.Errorf("suite name '%s' is invalid", suiteName)
}

func filterPlaywrightSuite(c *playwright.Project) error {
	for _, s := range c.Suites {
		if s.Name == suiteName {
			c.Suites = []playwright.Suite{s}
			return nil
		}
	}
	return fmt.Errorf("suite name '%s' is invalid", suiteName)
}

func filterTestcafeSuite(c *testcafe.Project) error {
	for _, s := range c.Suites {
		if s.Name == suiteName {
			c.Suites = []testcafe.Suite{s}
			return nil
		}
	}
	return fmt.Errorf("suite name '%s' is invalid", suiteName)
}

func filterPuppeteerSuite(c *puppeteer.Project) error {
	for _, s := range c.Suites {
		if s.Name == suiteName {
			c.Suites = []puppeteer.Suite{s}
			return nil
		}
	}
	return fmt.Errorf("suite name '%s' is invalid", suiteName)
}

func validateFiles(files []string) error {
	for _, f := range files {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func applyDefaultValues(sauce *config.SauceConfig) {
	if sauce.Region == "" {
		sauce.Region = defaultRegion
	}

	if sauce.Sauceignore == "" {
		sauce.Sauceignore = defaultSauceignore
	}
}

func overrideCliParameters(cmd *cobra.Command, sauce *config.SauceConfig) {
	if cmd.Flags().Lookup("region").Changed {
		sauce.Region = regionFlag
	}
	if cmd.Flags().Lookup("ccy").Changed {
		sauce.Concurrency = concurrency
	}
	if cmd.Flags().Lookup("tunnel-id").Changed {
		sauce.Tunnel.ID = tunnelID
	}
	if cmd.Flags().Lookup("tunnel-parent").Changed {
		sauce.Tunnel.Parent = tunnelParent
	}
	if cmd.Flags().Lookup("sauceignore").Changed {
		sauce.Sauceignore = sauceignore
	}
	if cmd.Flags().Lookup("experiment").Changed {
		sauce.Experiments = experiments
	}
}
