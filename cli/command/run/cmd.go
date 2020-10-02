package run

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/config"
	"github.com/saucelabs/saucectl/cli/mocks"
	"github.com/saucelabs/saucectl/cli/runner"
	"github.com/saucelabs/saucectl/internal/ci"
	"github.com/saucelabs/saucectl/internal/ci/github"
	"github.com/saucelabs/saucectl/internal/ci/gitlab"
	"github.com/saucelabs/saucectl/internal/ci/jenkins"
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
)

// Command creates the `run` command
func Command(cli *command.SauceCtlCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     runUse,
		Short:   runShort,
		Long:    runLong,
		Example: runExample,
		Run: func(cmd *cobra.Command, args []string) {
			log.Info().Msg("Start Run Command")
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
	p, err := config.NewJobConfiguration(cfgFilePath)
	if err != nil {
		return 1, err
	}

	mergeArgs(cmd, &p)
	p.Metadata.ExpandEnv()

	if len(p.Suites) == 0 {
		// As saucectl is transitioning into supporting test suites, we'll try to support this transition without
		// a breaking change (if possible). This means that a suite may not necessarily be defined by the user.
		// As such, we create an imaginary suite based on the project configuration.
		p.Suites = []config.Suite{newDefaultSuite(p)}
	}

	r, err := newRunner(p, cli)
	if err != nil {
		return 1, err
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
	u := os.Getenv("SAUCE_USERNAME")
	k := os.Getenv("SAUCE_ACCESS_KEY")
	if u == "" || k == "" {
		log.Info().Msg("No credentials provided. Running tests sequentially.")
		return &memseq.Sequencer{}
	}
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
		HTTPClient: &http.Client{Timeout: 3 * time.Second},
		URL:        apiBaseURL(r),
		Username:   u,
		AccessKey:  k,
	}
}

func apiBaseURL(r region.Region) string {
	// Check for overrides.
	if sauceAPI != "" {
		return sauceAPI
	}

	return r.APIBaseURL()
}

// newDefaultSuite creates a rudimentary test suite from a project configuration.
// Its main use is for when no suites are defined in the config.
func newDefaultSuite(p config.Project) config.Suite {
	// TODO remove this method once saucectl fully transitions to the new config
	s := config.Suite{Name: "default"}

	return s
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
