package run

import (
	"github.com/saucelabs/saucectl/cli/mocks"
	"github.com/saucelabs/saucectl/internal/ci"
	"github.com/saucelabs/saucectl/internal/docker"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/config"
	"github.com/saucelabs/saucectl/cli/runner"
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
	region      string
	env         map[string]string
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
	cmd.Flags().StringVarP(&region, "region", "r", "", "The sauce labs region. (default: us-west-1)")
	cmd.Flags().StringToStringVarP(&env, "env", "e", map[string]string{}, "Set environment variables, e.g. -e foo=bar.")
	return cmd
}

// Run runs the command
func Run(cmd *cobra.Command, cli *command.SauceCtlCli, args []string) (int, error) {
	// Todo(Christian) write argument parser/validator
	if cfgLogDir == defaultLogFir {
		pwd, _ := os.Getwd()
		cfgLogDir = filepath.Join(pwd, "logs")
	}

	log.Info().Str("config", cfgFilePath).Msg("Reading config file")
	cfg, err := config.NewJobConfiguration(cfgFilePath)
	if err != nil {
		return 1, err
	}

	mergeArgs(&cfg)
	cfg.Metadata.ExpandEnv()

	if len(cfg.Suites) == 0 {
		// As saucectl is transitioning into supporting test suites, we'll try to support this transition without
		// a breaking change (if possible). This means that a suite may not necessarily be defined by the user.
		// As such, we create an imaginary suite based on the project configuration.
		cfg.Suites = []config.Suite{newDefaultSuite(cfg)}
	}

	for _, suite := range cfg.Suites {
		exitCode, err := runSuite(cfg, suite, cli)
		if err != nil  || exitCode != 0 {
			return exitCode, err
		}
	}

	return 0, nil
}

func runSuite(p config.Project, s config.Suite, cli *command.SauceCtlCli) (int, error) {
	tr, err := newRunner(p, s, cli)
	if err != nil {
		return 1, err
	}
	defer func() {
		log.Info().Msg("Tearing down environment")
		if err != tr.Teardown(cfgLogDir) {
			log.Error().Err(err).Msg("Failed to tear down environment")
		}
	}()

	log.Info().Msg("Setting up test environment")
	if err := tr.Setup(); err != nil {
		return 1, err
	}

	log.Info().Msg("Starting tests")
	exitCode, err := tr.Run()
	if err != nil {
		return 1, err
	}

	log.Info().
		Int("ExitCode", exitCode).
		Msg("Command Finished")

	return exitCode, err
}

func newRunner(p config.Project, s config.Suite, cli *command.SauceCtlCli) (runner.Testrunner, error) {
	// return test runner for testing
	if p.Image.Base == "test" {
		return mocks.NewTestRunner(p, cli)
	}

	if ci.IsAvailable() {
		log.Info().Msg("Starting CI runner")
		return ci.NewRunner(p, s, cli)
	}

	log.Info().Msg("Starting local runner")
	return docker.NewRunner(p, s, cli)
}

// newDefaultSuite creates a rudimentary test suite from a project configuration.
// Its main use is for when no suites are defined in the config.
func newDefaultSuite(p config.Project) config.Suite {
	// TODO remove this method once saucectl fully transitions to the new config
	s := config.Suite{Name: "default", Files: p.Files}
	if len(p.Capabilities) > 0 {
		s.Capabilities = p.Capabilities[0]
	}

	return s
}

// mergeArgs merges settings from CLI arguments with the loaded job configuration.
func mergeArgs(cfg *config.Project) {
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

	if region != "" {
		cfg.Sauce.Region = region
	}
}
