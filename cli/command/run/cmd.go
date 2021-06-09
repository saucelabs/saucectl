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
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/docker"
	"github.com/saucelabs/saucectl/internal/github"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/saucelabs/saucectl/internal/puppeteer"
	"github.com/saucelabs/saucectl/internal/rdc"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/resto"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/sentry"
	"github.com/saucelabs/saucectl/internal/testcafe"
	"github.com/saucelabs/saucectl/internal/testcomposer"
	"github.com/saucelabs/saucectl/internal/xcuitest"
)

var (
	runUse   = "run"
	runShort = "Runs tests on Sauce Labs"

	defaultLogFir      = "<cwd>/logs"
	defaultRegion      = "us-west-1"
	defaultSauceignore = ".sauceignore"

	cfgFilePath    string
	cfgLogDir      string
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

	// General Request Timeouts
	appStoreTimeout     = 300 * time.Second
	testComposerTimeout = 300 * time.Second
	restoTimeout        = 60 * time.Second
	rdcTimeout          = 15 * time.Second
	githubTimeout       = 2 * time.Second
)

// Command creates the `run` command
func Command(cli *command.SauceCtlCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:              runUse,
		Short:            runShort,
		TraverseChildren: true,
		Run: func(cmd *cobra.Command, args []string) {
			exitCode, err := Run(cmd, cli, args)
			if err != nil {
				log.Err(err).Msg("failed to execute run command")
				sentry.CaptureError(err, sentry.Scope{
					Username:   credentials.Get().Username,
					ConfigFile: cfgFilePath,
				})
			}
			os.Exit(exitCode)
		},
	}

	defaultCfgPath := filepath.Join(".sauce", "config.yml")
	cmd.PersistentFlags().StringVarP(&cfgFilePath, "config", "c", defaultCfgPath, "Specifies which config file to use")
	cmd.PersistentFlags().StringVarP(&cfgLogDir, "logDir", "l", defaultLogFir, "log path")
	cmd.PersistentFlags().DurationVarP(&globalTimeout, "timeout", "t", 0, "Global timeout that limits how long saucectl can run in total. Supports duration values like '10s', '30m' etc. (default: no timeout)")
	cmd.PersistentFlags().StringVarP(&regionFlag, "region", "r", "", "The sauce labs region. (default: us-west-1)")
	cmd.PersistentFlags().StringToStringVarP(&env, "env", "e", map[string]string{}, "Set environment variables, e.g. -e foo=bar.")
	cmd.PersistentFlags().StringVar(&sauceAPI, "sauce-api", "", "Overrides the region specific sauce API URL. (e.g. https://api.us-west-1.saucelabs.com)")
	cmd.PersistentFlags().StringVar(&suiteName, "suite", "", "Run specified test suite.")
	cmd.PersistentFlags().BoolVar(&testEnvSilent, "test-env-silent", false, "Skips the test environment announcement.")
	cmd.PersistentFlags().StringVar(&testEnv, "test-env", "", "Specifies the environment in which the tests should run. Choice: docker|sauce.")
	cmd.PersistentFlags().BoolVarP(&showConsoleLog, "show-console-log", "", false, "Shows suites console.log locally. By default console.log is only shown on failures.")
	cmd.PersistentFlags().IntVar(&concurrency, "ccy", 2, "Concurrency specifies how many suites are run at the same time.")
	cmd.PersistentFlags().StringVar(&tunnelID, "tunnel-id", "", "Sets the sauce-connect tunnel ID to be used for the run.")
	cmd.PersistentFlags().StringVar(&tunnelParent, "tunnel-parent", "", "Sets the sauce-connect tunnel parent to be used for the run.")
	cmd.PersistentFlags().StringVar(&runnerVersion, "runner-version", "", "Overrides the automatically determined runner version.")
	cmd.PersistentFlags().StringVar(&sauceignore, "sauceignore", "", "Specifies the path to the .sauceignore file.")
	cmd.PersistentFlags().StringToStringVar(&experiments, "experiment", map[string]string{}, "Specifies a list of experimental flags and values")
	cmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "", false, "Simulate a test run without actually running any tests.")

	// Metadata
	cmd.PersistentFlags().StringSliceVar(&tags, "tags", []string{}, "Adds tags to tests")
	cmd.PersistentFlags().StringVar(&build, "build", "", "Associates tests with a build")

	cmd.Flags().MarkDeprecated("test-env", "please set mode in config file")

	// Hide undocumented flags that the user does not need to care about.
	_ = cmd.PersistentFlags().MarkHidden("sauce-api")
	_ = cmd.PersistentFlags().MarkHidden("runner-version")
	_ = cmd.PersistentFlags().MarkHidden("experiment")

	cmd.AddCommand(NewEspressoCmd(cli))

	return cmd
}

// Run runs the command
func Run(cmd *cobra.Command, cli *command.SauceCtlCli, args []string) (int, error) {
	println("Running version", version.Version)
	checkForUpdates()
	go awaitGlobalTimeout()

	creds := credentials.Get()
	if !creds.IsValid() {
		color.Red("\nSauceCTL requires a valid Sauce Labs account!\n\n")
		fmt.Println(`Set up your credentials by running:
> saucectl configure`)
		println()
		return 1, fmt.Errorf("no credentials set")
	}

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

	tc := testcomposer.Client{
		HTTPClient:  &http.Client{Timeout: testComposerTimeout},
		URL:         "", // updated later once region is determined
		Credentials: creds,
	}

	rs := resto.Client{
		HTTPClient: &http.Client{Timeout: restoTimeout},
		URL:        "", // updated later once region is determined
		Username:   creds.Username,
		AccessKey:  creds.AccessKey,
	}

	rc := rdc.Client{
		HTTPClient: &http.Client{
			Timeout: rdcTimeout,
		},
		Username:  creds.Username,
		AccessKey: creds.AccessKey,
	}

	as := appstore.New("", creds.Username, creds.AccessKey, appStoreTimeout)

	// TODO switch statement with pre-constructed type definition structs?
	if d.Kind == config.KindCypress && d.APIVersion == config.VersionV1Alpha {
		return runCypress(cmd, tc, rs, as)
	}
	if d.Kind == config.KindPlaywright && d.APIVersion == config.VersionV1Alpha {
		return runPlaywright(cmd, tc, rs, as)
	}
	if d.Kind == config.KindTestcafe && d.APIVersion == config.VersionV1Alpha {
		return runTestcafe(cmd, tc, rs, as)
	}
	if d.Kind == config.KindPuppeteer && d.APIVersion == config.VersionV1Alpha {
		return runPuppeteer(cmd, tc, rs)
	}
	if d.Kind == config.KindEspresso && d.APIVersion == config.VersionV1Alpha {
		return runEspresso(cmd, tc, rs, rc, as)
	}
	if d.Kind == config.KindXcuitest && d.APIVersion == config.VersionV1Alpha {
		return runXcuitest(cmd, tc, rs, rc, as)
	}

	return 1, errors.New("unknown framework configuration")
}

func printTestEnv(testEnv string) {
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

func runCypress(cmd *cobra.Command, tc testcomposer.Client, rs resto.Client, as *appstore.AppStore) (int, error) {
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

	if cmd.Flags().Lookup("suite").Changed {
		if err := filterCypressSuite(&p); err != nil {
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
	if testEnv != "" {
		for i, s := range p.Suites {
			s.Mode = testEnv
			p.Suites[i] = s
		}
	}

	if err := cypress.Validate(p); err != nil {
		return 1, err
	}

	regio := region.FromString(p.Sauce.Region)
	if regio == region.None {
		log.Error().Str("region", regionFlag).Msg("Unable to determine sauce region.")
		return 1, errors.New("no sauce region set")
	}

	tc.URL = regio.APIBaseURL()
	rs.URL = regio.APIBaseURL()
	as.URL = regio.APIBaseURL()

	rs.ArtifactConfig = p.Artifacts.Download

	dockerProject, sauceProject := cypress.SplitSuites(p)
	if len(dockerProject.Suites) != 0 {
		exitCode, err := runCypressInDocker(dockerProject, tc, rs)
		if err != nil || exitCode != 0 {
			return exitCode, err
		}
	}
	if len(sauceProject.Suites) != 0 {
		return runCypressInSauce(sauceProject, regio, tc, rs, as)
	}

	return 0, nil
}

func runCypressInDocker(p cypress.Project, testco testcomposer.Client, rs resto.Client) (int, error) {
	log.Info().Msg("Running Cypress in Docker")
	printTestEnv("docker")

	cd, err := docker.NewCypress(p, &testco, &testco, &rs)
	if err != nil {
		return 1, err
	}
	return cd.RunProject()
}

func runCypressInSauce(p cypress.Project, regio region.Region, tc testcomposer.Client, rs resto.Client, as *appstore.AppStore) (int, error) {
	log.Info().Msg("Running Cypress in Sauce Labs")
	printTestEnv("sauce")

	r := saucecloud.CypressRunner{
		Project: p,
		CloudRunner: saucecloud.CloudRunner{
			ProjectUploader:    as,
			JobStarter:         &tc,
			JobReader:          &rs,
			JobStopper:         &rs,
			JobWriter:          &tc,
			CCYReader:          &rs,
			TunnelService:      &rs,
			Region:             regio,
			ShowConsoleLog:     p.ShowConsoleLog,
			ArtifactDownloader: &rs,
			DryRun:             dryRun,
		},
	}
	return r.RunProject()
}

func runPlaywright(cmd *cobra.Command, tc testcomposer.Client, rs resto.Client, as *appstore.AppStore) (int, error) {
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
	if cmd.Flags().Lookup("suite").Changed {
		if err := filterPlaywrightSuite(&p); err != nil {
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
	if testEnv != "" {
		for i, s := range p.Suites {
			s.Mode = testEnv
			p.Suites[i] = s
		}
	}

	regio := region.FromString(p.Sauce.Region)
	if regio == region.None {
		log.Error().Str("region", regionFlag).Msg("Unable to determine sauce region.")
		return 1, errors.New("no sauce region set")
	}

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

func runPlaywrightInSauce(p playwright.Project, regio region.Region, tc testcomposer.Client, rs resto.Client, as *appstore.AppStore) (int, error) {
	log.Info().Msg("Running Playwright in Sauce Labs")
	printTestEnv("sauce")

	r := saucecloud.PlaywrightRunner{
		Project: p,
		CloudRunner: saucecloud.CloudRunner{
			ProjectUploader:    as,
			JobStarter:         &tc,
			JobReader:          &rs,
			JobStopper:         &rs,
			JobWriter:          &tc,
			CCYReader:          &rs,
			TunnelService:      &rs,
			Region:             regio,
			ShowConsoleLog:     p.ShowConsoleLog,
			ArtifactDownloader: &rs,
			DryRun:             dryRun,
		},
	}
	return r.RunProject()
}

func runTestcafe(cmd *cobra.Command, tc testcomposer.Client, rs resto.Client, as *appstore.AppStore) (int, error) {
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
	if testEnv != "" {
		for i, s := range p.Suites {
			s.Mode = testEnv
			p.Suites[i] = s
		}
	}

	regio := region.FromString(p.Sauce.Region)
	if regio == region.None {
		log.Error().Str("region", regionFlag).Msg("Unable to determine sauce region.")
		return 1, errors.New("no sauce region set")
	}

	tc.URL = regio.APIBaseURL()
	rs.URL = regio.APIBaseURL()
	as.URL = regio.APIBaseURL()

	rs.ArtifactConfig = p.Artifacts.Download

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

func runTestcafeInCloud(p testcafe.Project, regio region.Region, tc testcomposer.Client, rs resto.Client, as *appstore.AppStore) (int, error) {
	log.Info().Msg("Running Testcafe in Sauce Labs")
	printTestEnv("sauce")

	r := saucecloud.TestcafeRunner{
		Project: p,
		CloudRunner: saucecloud.CloudRunner{
			ProjectUploader:    as,
			JobStarter:         &tc,
			JobReader:          &rs,
			JobStopper:         &rs,
			JobWriter:          &tc,
			CCYReader:          &rs,
			TunnelService:      &rs,
			Region:             regio,
			ShowConsoleLog:     p.ShowConsoleLog,
			ArtifactDownloader: &rs,
			DryRun:             dryRun,
		},
	}
	return r.RunProject()
}

func runXcuitest(cmd *cobra.Command, tc testcomposer.Client, rs resto.Client, rc rdc.Client, as *appstore.AppStore) (int, error) {
	p, err := xcuitest.FromFile(cfgFilePath)
	if err != nil {
		return 1, err
	}
	p.Sauce.Metadata.ExpandEnv()
	applyDefaultValues(&p.Sauce)
	overrideCliParameters(cmd, &p.Sauce)

	regio := region.FromString(p.Sauce.Region)
	if regio == region.None {
		log.Error().Str("region", regionFlag).Msg("Unable to determine sauce region.")
		return 1, errors.New("no sauce region set")
	}

	xcuitest.SetDeviceDefaultValues(&p)
	err = xcuitest.Validate(p)
	if err != nil {
		return 1, err
	}

	if cmd.Flags().Lookup("suite").Changed {
		if err := filterXcuitestSuite(&p); err != nil {
			return 1, err
		}
	}

	tc.URL = regio.APIBaseURL()
	rs.URL = regio.APIBaseURL()
	as.URL = regio.APIBaseURL()
	rc.URL = regio.APIBaseURL()

	rs.ArtifactConfig = p.Artifacts.Download
	rc.ArtifactConfig = p.Artifacts.Download

	return runXcuitestInCloud(p, regio, tc, rs, rc, as)
}

func runXcuitestInCloud(p xcuitest.Project, regio region.Region, tc testcomposer.Client, rs resto.Client, rc rdc.Client, as *appstore.AppStore) (int, error) {
	log.Info().Msg("Running XCUITest in Sauce Labs")
	printTestEnv("sauce")

	r := saucecloud.XcuitestRunner{
		Project: p,
		CloudRunner: saucecloud.CloudRunner{
			ProjectUploader:       as,
			JobStarter:            &tc,
			JobReader:             &rs,
			RDCJobReader:          &rc,
			JobStopper:            &rs,
			JobWriter:             &tc,
			CCYReader:             &rs,
			TunnelService:         &rs,
			Region:                regio,
			ShowConsoleLog:        false,
			ArtifactDownloader:    &rs,
			RDCArtifactDownloader: &rc,
			DryRun:                dryRun,
		},
	}
	return r.RunProject()
}

func runPuppeteer(cmd *cobra.Command, tc testcomposer.Client, rs resto.Client) (int, error) {
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

	if cmd.Flags().Lookup("suite").Changed {
		if err := filterPuppeteerSuite(&p); err != nil {
			return 1, err
		}
	}

	regio := region.FromString(p.Sauce.Region)
	if regio == region.None {
		log.Error().Str("region", regionFlag).Msg("Unable to determine sauce region.")
		return 1, errors.New("no sauce region set")
	}

	rs.URL = regio.APIBaseURL()
	tc.URL = regio.APIBaseURL()
	return runPuppeteerInDocker(p, tc, rs)
}

func runPuppeteerInDocker(p puppeteer.Project, testco testcomposer.Client, rs resto.Client) (int, error) {
	log.Info().Msg("Running puppeteer in Docker")
	printTestEnv("docker")

	cd, err := docker.NewPuppeteer(p, &testco, &testco, &rs)
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

func filterXcuitestSuite(c *xcuitest.Project) error {
	for _, s := range c.Suites {
		if s.Name == suiteName {
			c.Suites = []xcuitest.Suite{s}
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
	if build != "" {
		sauce.Metadata.Build = build
	}
	if len(tags) != 0 {
		sauce.Metadata.Tags = tags
	}
}

// awaitGlobalTimeout waits for the global timeout event. In case of global timeout event, it attempts to interrupt the
// current process. Should this fail, a hard immediate exit is performed.
func awaitGlobalTimeout() {
	if globalTimeout == 0 {
		return
	}

	<-time.After(globalTimeout)
	msg.LogGlobalTimeoutShutdown()

	// Can't send interrupt signals on windows. A hard exit is our only choice.
	if runtime.GOOS == "windows" {
		os.Exit(1)
	}

	p, err := os.FindProcess(os.Getpid())
	if err == nil {
		err = p.Signal(syscall.SIGINT)
	}

	if err != nil {
		color.Red("Unable to perform soft shutdown. Exiting immediately...")
		os.Exit(1)
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
