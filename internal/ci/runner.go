package ci

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/saucelabs/saucectl/cli/utils"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/config"
	"github.com/saucelabs/saucectl/cli/runner"
	"github.com/saucelabs/saucectl/internal/fleet"
	"github.com/saucelabs/saucectl/internal/fpath"
	"github.com/saucelabs/saucectl/internal/yaml"

	"github.com/rs/zerolog/log"
)

// Runner represents the CI implementation of a runner.Testrunner.
type Runner struct {
	runner.BaseRunner
	CIProvider Provider
}

// NewRunner creates a new Runner instance.
func NewRunner(c config.Project, cli *command.SauceCtlCli, seq fleet.Sequencer, rc config.RunnerConfiguration, cip Provider) (*Runner, error) {
	r := Runner{}

	r.Cli = cli
	r.Ctx = context.Background()
	r.Project = c
	r.RunnerConfig = rc
	r.Sequencer = seq
	r.CIProvider = cip
	return &r, nil
}

// RunProject runs the tests defined in config.Project.
func (r *Runner) RunProject() (int, error) {
	bid := r.buildID()
	log.Info().Str("buildID", bid).Msg("Generated build ID")
	fid, err := fleet.Register(r.Ctx, r.Sequencer, bid, r.Project.Files,
		r.Project.Suites)
	if err != nil {
		return 1, err
	}

	errorCount := 0
	for _, suite := range r.Project.Suites {
		err = r.runSuite(suite, fid)
		if err != nil {
			errorCount++
		}
	}
	if errorCount > 0 {
		log.Error().Msgf("%d suite(s) failed", errorCount)
	}
	return errorCount, nil
}

// setup performs any necessary steps for a test runner to execute tests.
func (r *Runner) setup(run config.Run) error {
	log.Info().Msg("Run entry.sh")
	var out bytes.Buffer
	var homeDir = utils.GetProjectDir()
	if os.Getenv("SAUCE_VM") == "" {
		cmd := exec.Command(path.Join(homeDir, "entry.sh"), "&")
		cmd.Stderr = &out
		if err := cmd.Run(); err != nil {
			log.Info().Err(err).Msg("Failed to setup test")
			return errors.New("couldn't start test: " + out.String())
		}
	}

	// TODO replace sleep with actual checks & confirmation
	// wait 2 seconds until everything is started
	time.Sleep(2 * time.Second)

	rcPath := filepath.Join(r.RunnerConfig.RootDir, "run.yaml")
	log.Info().Msg("Writing run.yaml to: " + rcPath)
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

	// installing dependencies
	err := r.installPackages(r.Project)
	if err != nil {
		return err
	}

	// running before-exec tasks
	err = r.beforeExec(r.Project.BeforeExec)
	if err != nil {
		return err
	}
	return nil
}

func (r *Runner) execute(task string) (int, error) {
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

func (r* Runner) installPackages(config config.Project) error {
	// Install NPM dependencies
	log.Info().Msg("Installing dependencies") //config.Npm.Packages
	npmInstall := []string{"npm", "install"}
	for name, version := range config.Npm.Packages {
		npmInstall = append(npmInstall, fmt.Sprintf("%s@%s", name, version))
	}
	exitCode, err := r.execute(strings.Join(npmInstall, " "))
	if exitCode != 0 {
		return fmt.Errorf("npm install exitCode is %d", exitCode)
	}
	if err != nil {
		return err
	}
	return nil
}

func (r *Runner) beforeExec(tasks []string) error {
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
		fmt.Sprintf("BROWSER_NAME=%s", suite.Settings.BrowserName),
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

func (r *Runner) runSuite(suite config.Suite, fleetID string) error {
	var assignments []string
	for {
		next, err := r.Sequencer.NextAssignment(r.Ctx, fleetID, suite.Name)
		if err != nil {
			return err
		}
		if next == "" {
			break
		}
		assignments = append(assignments, next)
	}

	if len(assignments) == 0 {
		log.Info().Msg("No tests detected. Skipping suite.")
		return nil
	}

	run := config.Run{
		Match:       assignments,
		ProjectPath: r.RunnerConfig.RootDir,
	}
	return r.runTest(suite, run)
}

func (r *Runner) runTest(suite config.Suite, run config.Run) error {
	defer func() {
		log.Info().Msg("Tearing down environment")
		if err := r.teardown(r.Cli.LogDir); err != nil {
			log.Error().Err(err).Msg("Failed to tear down environment")
		}
	}()

	log.Info().Msg("Setting up test environment")
	if err := r.setup(run); err != nil {
		return err
	}

	log.Info().Msg("Starting tests")
	exitCode, err := r.run(suite)
	log.Info().
		Int("ExitCode", exitCode).
		Msg("Command Finished")

	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("exitCode = %d", exitCode)
	}
	return nil
}

// buildID generates a build ID based on the current CI and project information.
func (r *Runner) buildID() string {
	p := r.Project
	in := struct {
		ciBuildID string
		version   string
		kind      string
		meta      config.Metadata
		files     []string
		suites    []config.Suite
		img       config.ImageDefinition
	}{
		r.CIProvider.BuildID(),
		p.APIVersion,
		p.Kind,
		p.Metadata,
		p.Files,
		p.Suites,
		p.Image,
	}
	pStr := fmt.Sprintf("%+v", in)
	h := sha1.New()
	io.WriteString(h, pStr)

	return hex.EncodeToString(h.Sum(nil))
}
