package runner

import (
	"context"
	"github.com/saucelabs/saucectl/internal/ci"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/config"
	"github.com/saucelabs/saucectl/cli/mocks"
)

const logDir = "/var/log/cont"

// RunnerConfigPath represents the path for the runner config.
var RunnerConfigPath = "/home/seluser/config.yaml"

// LogFiles contains the locations of log and resource files that are useful for reporting.
var LogFiles = [...]string{
	logDir + "/chrome_browser.log",
	logDir + "/firefox_browser.log",
	logDir + "/supervisord.log",
	logDir + "/video-rec-stderr.log",
	logDir + "/video-rec-stdout.log",
	logDir + "/wait-xvfb.1.log",
	logDir + "/wait-xvfb.2.log",
	logDir + "/wait-xvfb-stdout.log",
	logDir + "/xvfb-tryouts-stderr.log",
	logDir + "/xvfb-tryouts-stdout.log",
	"/home/seluser/videos/video.mp4",
	"/home/seluser/docker.log",
}

// Testrunner describes the test runner interface
type Testrunner interface {
	Setup() error
	Run() (int, error)
	Teardown(logDir string) error
}

// BaseRunner contains common properties across all runners
type BaseRunner struct {
	project      config.Project
	runnerConfig config.RunnerConfiguration
	context      context.Context
	cli          *command.SauceCtlCli

	startTime int64
}

// New creates a new testrunner object
func New(c config.Project, cli *command.SauceCtlCli) (Testrunner, error) {
	// return test runner for testing
	if c.Image.Base == "test" {
		return mocks.NewTestRunner(c, cli)
	}

	if ci.IsAvailable() {
		log.Info().Msg("Starting CI runner")
		return newCIRunner(c, cli)
	}

	log.Info().Msg("Starting local runner")
	return newLocalRunner(c, cli)
}

func makeTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
