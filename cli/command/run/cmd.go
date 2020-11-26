package run

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/config"
	"github.com/saucelabs/saucectl/cli/credentials"
	"github.com/saucelabs/saucectl/cli/mocks"
	"github.com/saucelabs/saucectl/cli/runner"
	"github.com/saucelabs/saucectl/cli/version"
	"github.com/saucelabs/saucectl/internal/ci"
	"github.com/saucelabs/saucectl/internal/ci/github"
	"github.com/saucelabs/saucectl/internal/ci/gitlab"
	"github.com/saucelabs/saucectl/internal/ci/jenkins"
	"github.com/saucelabs/saucectl/internal/cypress"
	cypressDocker "github.com/saucelabs/saucectl/internal/cypress/docker"
	"github.com/saucelabs/saucectl/internal/cypress/sauce"
	"github.com/saucelabs/saucectl/internal/docker"
	"github.com/saucelabs/saucectl/internal/fleet"
	"github.com/saucelabs/saucectl/internal/memseq"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/testcomposer"
	"github.com/spf13/cobra"
)

var (
	runUse     = "run ./.sauce/config.yaml"
	runShort   = "Run a test on Sauce Labs"
	runLong    = `Some long description`
	runExample = "saucectl run ./.sauce/config.yaml"

	defaultLogFir  = "<cwd>/logs"
	defaultTimeout = 60
	defaultRegion  = "us-west-1"

	cfgFilePath string
	cfgLogDir   string
	testTimeout int
	regionFlag  string
	env         map[string]string
	parallel    bool
	ciBuildID   string
	sauceAPI    string
	suiteName   string
	testEnv     string
)

// Command creates the `run` command
func Command(cli *command.SauceCtlCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     runUse,
		Short:   runShort,
		Long:    runLong,
		Example: runExample,
		Run: func(cmd *cobra.Command, args []string) {
			log.Info().Msgf("Running version %s", version.Version)
			exitCode, err := Run(cmd, cli, args)
			if err != nil {
				log.Err(err).Msg("failed to execute run command")
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
	cmd.Flags().StringVar(&testEnv, "test-env", "docker", "Specifies the environment in which the tests should run. Choice: docker|sauce.")

	// Hide undocumented flags that the user does not need to care about.
	_ = cmd.Flags().MarkHidden("sauce-api")

	// Hide documented flags that aren't fully released yet or WIP.
	_ = cmd.Flags().MarkHidden("parallel")    // WIP.
	_ = cmd.Flags().MarkHidden("ci-build-id") // Related to 'parallel'. WIP.
	_ = cmd.Flags().MarkHidden("test-env")    // WIP.

	return cmd
}

// Run runs the command
func Run(cmd *cobra.Command, cli *command.SauceCtlCli, args []string) (int, error) {
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

	if d.Kind == "cypress" && d.APIVersion == "v1alpha" {
		return runCypress(cli)
	}

	return runLegacyMode(cmd, cli)
}

func runLegacyMode(cmd *cobra.Command, cli *command.SauceCtlCli) (int, error) {
	p, err := config.NewJobConfiguration(cfgFilePath)
	if err != nil {
		return 1, err
	}

	mergeArgs(cmd, &p)
	if err := validateFiles(p.Files); err != nil {
		return 1, err
	}
	if cmd.Flags().Lookup("suite").Changed {
		if err := filterSuite(&p); err != nil {
			return 1, err
		}
	}
	p.Metadata.ExpandEnv()

	r, err := newRunner(p, cli)
	if err != nil {
		return 1, err
	}
	return r.RunProject()
}

func runCypress(cli *command.SauceCtlCli) (int, error) {
	p, err := cypress.FromFile(cfgFilePath)
	if err != nil {
		return 1, err
	}

	p.Sauce.Metadata.ExpandEnv()

	// Merge env from CLI args and job config. CLI args take precedence.
	for k, v := range env {
		for _, s := range p.Suites {
			if s.Config.Env == nil {
				s.Config.Env = map[string]string{}
			}
			s.Config.Env[k] = v
		}
	}

	if p.Sauce.Region == "" {
		p.Sauce.Region = defaultRegion
	}

	if regionFlag != "" {
		p.Sauce.Region = regionFlag
	}

	switch testEnv {
	case "docker":
		return runCypressInDocker(p, cli)
	case "sauce":
		return runCypressInSauce(p)
	default:
		return 1, errors.New("unsupported test environment")
	}
}

func runCypressInDocker(p cypress.Project, cli *command.SauceCtlCli) (int, error) {
	log.Info().Msg("Running Cypress in Docker")

	cd, err := cypressDocker.New(p, cli)
	if err != nil {
		return 1, err
	}
	return cd.RunProject()
}

func runCypressInSauce(p cypress.Project) (int, error) {
	log.Info().Msg("Running Cypress in Sauce Labs")

	r := sauce.Runner{
		Project: p,
	}
	return r.RunProject()
}

func newRunner(p config.Project, cli *command.SauceCtlCli) (runner.Testrunner, error) {
	// return test runner for testing
	if p.Image.Base == "test" {
		return mocks.NewTestRunner(p, cli)
	}
	if ci.IsAvailable() {
		rc, err := config.NewRunnerConfiguration(runner.ConfigPath)
		if err != nil {
			return nil, err
		}
		if os.Getenv("SAUCE_TARGET_DIR") != "" {
			rc.TargetDir = os.Getenv("SAUCE_TARGET_DIR")
		}
		if os.Getenv("SAUCE_ROOT_DIR") != "" {
			rc.RootDir = os.Getenv("SAUCE_ROOT_DIR")
		}
		cip := createCIProvider()
		seq := createCISequencer(p, cip)

		log.Info().Msg("Starting CI runner")
		return ci.NewRunner(p, cli, seq, rc, cip)
	}
	log.Info().Msg("Starting local runner")
	return docker.NewRunner(p, cli, &memseq.Sequencer{})
}

func createCIProvider() ci.Provider {
	enableCIProviders()
	cip := ci.Detect()
	// Allow users to override the CI build ID
	if ciBuildID != "" {
		log.Info().Str("id", ciBuildID).Msg("Using user provided build ID.")
		cip.SetBuildID(ciBuildID)
	}

	return cip
}

func createCISequencer(p config.Project, cip ci.Provider) fleet.Sequencer {
	if !p.Parallel {
		log.Info().Msg("Parallel execution is turned off. Running tests sequentially.")
		log.Info().Msg("If you'd like to speed up your tests, please visit " +
			"https://github.com/saucelabs/saucectl on how to configure parallelization across machines.")
		return &memseq.Sequencer{}
	}
	creds := credentials.Get()
	if creds == nil {
		log.Info().Msg("No valid credentials provided. Running tests sequentially.")
		return &memseq.Sequencer{}
	}
	log.Info().Msgf("Using credentials from %s", creds.Source)
	if cip == ci.NoProvider && ciBuildID == "" {
		// Since we don't know the CI provider, we can't reliably generate a build ID, which is a requirement for
		// running tests in parallel. The user has to provide one in this case, and if they didn't, we have to disable
		// parallelism.
		log.Warn().Msg("Unable to detect CI provider. Running tests sequentially.")
		return &memseq.Sequencer{}
	}
	r := region.FromString(p.Sauce.Region)
	if r == region.None {
		log.Warn().Str("region", regionFlag).Msg("Unable to determine region. Running tests sequentially.")
		return &memseq.Sequencer{}
	}

	log.Info().Msg("Running tests in parallel.")
	return &testcomposer.Client{
		HTTPClient:  &http.Client{Timeout: 3 * time.Second},
		URL:         apiBaseURL(r),
		Credentials: *creds,
	}
}

func apiBaseURL(r region.Region) string {
	// Check for overrides.
	if sauceAPI != "" {
		return sauceAPI
	}

	return r.APIBaseURL()
}

// mergeArgs merges settings from CLI arguments with the loaded job configuration.
func mergeArgs(cmd *cobra.Command, cfg *config.Project) {
	// Merge env from CLI args and job config. CLI args take precedence.
	for k, v := range env {
		cfg.Env[k] = v
	}

	if testTimeout != 0 {
		cfg.Timeout = testTimeout
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = defaultTimeout
	}

	if cfg.Sauce.Region == "" {
		cfg.Sauce.Region = defaultRegion
	}

	if regionFlag != "" {
		cfg.Sauce.Region = regionFlag
	}

	if cmd.Flags().Lookup("parallel").Changed {
		cfg.Parallel = parallel
	}
}

func enableCIProviders() {
	github.Enable()
	gitlab.Enable()
	jenkins.Enable()
}

func filterSuite(c *config.Project) error {
	for _, s := range c.Suites {
		if s.Name == suiteName {
			c.Suites = []config.Suite{s}
			return nil
		}
	}
	return errors.New("suite name is invalid")
}

func validateFiles(files []string) error {
	for _, f := range files {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
