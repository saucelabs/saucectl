package runner

import (
	"bytes"
	"context"
	"fmt"
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

type CIRunner struct {
	BaseRunner
}

func NewCIRunner(c config.Project, cli *command.SauceCtlCli) (*CIRunner, error) {
	runner := CIRunner{}

	// read runner config file
	rc, err := config.NewRunnerConfiguration(RunnerConfigPath)
	if err != nil {
		return &runner, err
	}

	runner.Cli = cli
	runner.Ctx = context.Background()
	runner.Project = c
	runner.RunnerConfig = rc
	return &runner, nil
}

func (r *CIRunner) Setup() error {
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

func (r *CIRunner) Run() (int, error) {
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

func (r *CIRunner) Teardown(logDir string) error {
	if logDir != "" {
		return nil
	}

	for _, containerSrcPath := range LogFiles {
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
