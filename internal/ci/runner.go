package ci

import (
	"bytes"
	"context"
	"fmt"
	"github.com/saucelabs/saucectl/cli/runner"
	"github.com/saucelabs/saucectl/internal/fleet"
	"github.com/saucelabs/saucectl/internal/fpath"
	"github.com/saucelabs/saucectl/internal/yaml"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"errors"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/config"
)

// Runner represents the CI implementation of a runner.Testrunner.
type Runner struct {
	runner.BaseRunner
}

// NewRunner creates a new Runner instance.
func NewRunner(c config.Project, cli *command.SauceCtlCli, seq fleet.Sequencer, rc config.RunnerConfiguration) (*Runner, error) {
	r := Runner{}

	r.Cli = cli
	r.Ctx = context.Background()
	r.Project = c
	r.RunnerConfig = rc
	r.Sequencer = seq
	return &r, nil
}

// RunProject runs the tests defined in config.Project.
func (r *Runner) RunProject() (int, error) {
	fid, err := fleet.Register(r.Ctx, r.Sequencer, r.Project.Files, r.Project.Suites)
	if err != nil {
		return 1, err
	}

	for _, suite := range r.Project.Suites {
		exitCode, err := r.runSuite(suite, fid)
		if err != nil || exitCode != 0 {
			return exitCode, err
		}
	}

	return 0, nil
}

// setup performs any necessary steps for a test runner to execute tests.
func (r *Runner) setup(run config.Run) error {
	log.Info().Msg("Run entry.sh")
	var out bytes.Buffer
	cmd := exec.Command("/home/seluser/entry.sh", "&")
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return errors.New("couldn't start test: " + out.String())
	}

	// TODO replace sleep with actual checks & confirmation
	// wait 2 seconds until everything is started
	time.Sleep(2 * time.Second)

	rcPath := filepath.Join(r.RunnerConfig.RootDir, "run.yaml")
	if err := yaml.WriteFile(rcPath, run); err != nil {
		return err
	}

	// copy files from repository into target dir
	log.Info().Msg("Copy files into assigned directories")
	for _, pattern := range r.Project.Files {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}

		for _, file := range matches {
			log.Info().Msg("Copy file " + file + " to " + r.RunnerConfig.RootDir)
			if err := fpath.DeepCopy(file, filepath.Join(r.RunnerConfig.RootDir, file)); err != nil {
				return err
			}
		}
	}
	// running before-exec tasks
	err := r.beforeExec(r.Project.BeforeExec)
	if err != nil {
		return err
	}
	return nil
}

func (r* Runner) execute(task string) (int, error) {
	args := strings.Fields(task)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = r.Cli.Out()
	cmd.Stderr = r.Cli.Out()
	cmd.Dir = r.RunnerConfig.RootDir
	cmd.Env = append(os.Environ())
	err := cmd.Run()
	if err != nil {
		return 1, err
	}
	return 0, nil
}

func (r* Runner) beforeExec(tasks []string) (error) {
	for _, task := range tasks {
		log.Info().Msgf("Running BeforeExec task: %s", task)
		exitCode, err := r.execute(task)
		if err != nil {
			return err
		}
		if exitCode != 0 {
			return fmt.Errorf("failed to run BeforeExec task: %s - exit code: %d", task, exitCode)
		}
	}
	return nil
}

// run runs the tests defined in the config.Project.
func (r *Runner) run(suite config.Suite) (int, error) {
	cmd := exec.Command(r.RunnerConfig.ExecCommand[0], r.RunnerConfig.ExecCommand[1])
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("SAUCE_BUILD_NAME=%s", r.Project.Metadata.Build),
		fmt.Sprintf("SAUCE_TAGS=%s", strings.Join(r.Project.Metadata.Tags, ",")),
		fmt.Sprintf("SAUCE_REGION=%s", r.Project.Sauce.Region),
		fmt.Sprintf("TEST_TIMEOUT=%d", r.Project.Timeout),
		fmt.Sprintf("BROWSER_NAME=%s", suite.Capabilities.BrowserName),
	)

	// Add any defined env variables from the job config / CLI args.
	for k, v := range r.Project.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	cmd.Stdout = r.Cli.Out()
	cmd.Stderr = r.Cli.Out()
	cmd.Dir = r.RunnerConfig.RootDir
	err := cmd.Run()

	if err != nil {
		return 1, err
	}
	return 0, nil
}

// teardown cleans up the test environment.
func (r *Runner) teardown(logDir string) error {
	if logDir != "" {
		return nil
	}

	for _, containerSrcPath := range runner.LogFiles {
		file := filepath.Base(containerSrcPath)
		dstPath := filepath.Join(logDir, file)
		if err := copyFile(containerSrcPath, dstPath); err != nil {
			continue
		}
	}

	return nil
}

var copyFile = copyFileFunc
func copyFileFunc(src string, targetDir string) error {
	input, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filepath.Join(targetDir, filepath.Base(src)), input, 0644)
	if err != nil {
		return err
	}

	return nil
}

func (r *Runner) runSuite(suite config.Suite, fleetID string) (int, error) {
	for {
		next, err := r.Sequencer.NextAssignment(r.Ctx, fleetID, suite.Name)
		if err != nil {
			return 1, err
		}
		if next == "" {
			return 0, nil
		}

		run := config.Run{
			Match:       []string{next},
			ProjectPath: r.RunnerConfig.RootDir,
		}

		code, err := r.runTest(suite, run)
		if err != nil || code != 0 {
			return code, err
		}
	}
}

func (r *Runner) runTest(suite config.Suite, run config.Run) (int, error) {
	defer func() {
		log.Info().Msg("Tearing down environment")
		if err := r.teardown(r.Cli.LogDir); err != nil {
			log.Error().Err(err).Msg("Failed to tear down environment")
		}
	}()

	log.Info().Msg("Setting up test environment")
	if err := r.setup(run); err != nil {
		return 1, err
	}

	log.Info().Msg("Starting tests")
	exitCode, err := r.run(suite)
	if err != nil {
		return 1, err
	}

	log.Info().
		Int("ExitCode", exitCode).
		Msg("Command Finished")

	return exitCode, err
}
