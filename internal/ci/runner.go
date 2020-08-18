package ci

import (
	"bytes"
	"context"
	"fmt"
	"github.com/saucelabs/saucectl/cli/runner"
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
	rc, err := config.NewRunnerConfiguration(runner.RunnerConfigPath)
	if err != nil {
		return &r, err
	}

	r.Cli = cli
	r.Ctx = context.Background()
	r.Project = c
	r.RunnerConfig = rc
	return &r, nil
}

// Setup performs any necessary steps for a test runner to execute tests.
func (r *Runner) Setup() error {
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

	// copy files from repository into target dir
	log.Info().Msg("Copy files into assigned directories")
	for _, pattern := range r.Project.Files {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}

		for _, file := range matches {
			log.Info().Msg("Copy file " + file + " to " + r.RunnerConfig.TargetDir)
			if err := copyFile(file, r.RunnerConfig.TargetDir); err != nil {
				return err
			}
		}
	}
	return nil
}

// Run runs the tests defined in the config.Project.
func (r *Runner) Run() (int, error) {
	browserName := ""
	if len(r.Project.Capabilities) > 0 {
		browserName = r.Project.Capabilities[0].BrowserName
	}

	cmd := exec.Command(r.RunnerConfig.ExecCommand[0], r.RunnerConfig.ExecCommand[1])
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("SAUCE_BUILD_NAME=%s", r.Project.Metadata.Build),
		fmt.Sprintf("SAUCE_TAGS=%s", strings.Join(r.Project.Metadata.Tags, ",")),
		fmt.Sprintf("SAUCE_REGION=%s", r.Project.Sauce.Region),
		fmt.Sprintf("TEST_TIMEOUT=%d", r.Project.Timeout),
		fmt.Sprintf("BROWSER_NAME=%s", browserName),
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

// Teardown cleans up the test environment.
func (r *Runner) Teardown(logDir string) error {
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
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	srcFile := src
	if !filepath.IsAbs(srcFile) {
		srcFile = filepath.Join(pwd, src)
	}

	input, err := ioutil.ReadFile(srcFile)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(targetDir+"/"+filepath.Base(srcFile), input, 0644)
	if err != nil {
		return err
	}

	return nil
}
