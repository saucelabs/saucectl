package ci

import (
	"bytes"
	"context"
	"fmt"
	"github.com/saucelabs/saucectl/cli/runner"
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
func NewRunner(c config.Project, cli *command.SauceCtlCli) (*Runner, error) {
	r := Runner{}

	// read runner config file
	rc, err := config.NewRunnerConfiguration(runner.ConfigPath)
	if err != nil {
		return &r, err
	}

	r.Cli = cli
	r.Ctx = context.Background()
	r.Project = c
	r.RunnerConfig = rc
	return &r, nil
}

func (r *Runner) RunProject() (int, error) {
	for _, suite := range r.Project.Suites {
		exitCode, err := r.runSuite(suite)
		if err != nil || exitCode != 0 {
			return exitCode, err
		}
	}

	return 0, nil
}

// setup performs any necessary steps for a test runner to execute tests.
func (r *Runner) setup(suite config.Suite) error {
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

	files, err := fpath.Walk(r.Project.Files, suite.Match)
	if err != nil {
		return err
	}
	rc := config.Run{
		ProjectPath: r.RunnerConfig.RootDir,
		Match:       files,
	}
	log.Info().Strs("matched", files).Msg("Detected test files")

	rcPath := filepath.Join(r.RunnerConfig.RootDir, "run.yaml")
	if err = yaml.WriteFile(rcPath, rc); err != nil {
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
			if err := replicateFile(file, r.RunnerConfig.RootDir); err != nil {
				return err
			}
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

func copyFile(src string, targetDir string) error {
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

// replicateFile copies src to targetDir. Unlike copyFile(), the path of src is replicated at targetDir.
func replicateFile(src string, targetDir string) error {
	targetPath := filepath.Join(targetDir, filepath.Dir(src))
	if err := os.MkdirAll(targetPath, os.ModePerm); err != nil {
		return err
	}

	finfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !finfo.IsDir() {
		input, err := ioutil.ReadFile(src)
		if err != nil {
			return err
		}
		return ioutil.WriteFile(filepath.Join(targetPath, filepath.Base(src)), input, 0644)
	}

	fis, err := ioutil.ReadDir(src)
	if err != nil {
		return err
	}

	for _, ff := range fis {
		if err := replicateFile(filepath.Join(src, ff.Name()), targetDir); err != nil {
			return err
		}
	}

	return nil
}

func (r *Runner) runSuite(suite config.Suite) (int, error) {
	defer func() {
		log.Info().Msg("Tearing down environment")
		if err := r.teardown(r.Cli.LogDir); err != nil {
			log.Error().Err(err).Msg("Failed to tear down environment")
		}
	}()

	log.Info().Msg("Setting up test environment")
	if err := r.setup(suite); err != nil {
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
