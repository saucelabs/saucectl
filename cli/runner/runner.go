package runner

import (
	"context"
	"os"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/config"
	"github.com/saucelabs/saucectl/cli/mocks"
)

const logDir = "/var/log/cont"

var runnerConfigPath = "/home/seluser/config.yaml"

var logFiles = [...]string{
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
	jobConfig    config.JobConfiguration
	runnerConfig config.RunnerConfiguration
	context      context.Context
	cli          *command.SauceCtlCli
	
	startTime int64
}

// New creates a new testrunner object
func New(c config.JobConfiguration, cli *command.SauceCtlCli) (Testrunner, error) {
	var (
		runner Testrunner
		err    error
	)

	// return test runner for testing
	if c.Image.Base == "test" {
		return mocks.NewTestRunner(c, cli)
	}

	_, err = os.Stat(runnerConfigPath)
	if os.IsNotExist(err) {
		log.Info().Msg("Start local runner")
		runner, err = newLocalRunner(c, cli)
	} else {
		log.Info().Msg("Start CI runner")
		runner, err = newCIRunner(c, cli)
	}

	return runner, err
}

func makeTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
