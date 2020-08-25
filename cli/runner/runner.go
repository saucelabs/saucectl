package runner

import (
	"context"
	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/config"
)

const logDir = "/var/log/cont"

// ConfigPath represents the path for the runner config.
var ConfigPath = "/home/seluser/config.yaml"

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
	Project      config.Project
	Suite        config.Suite
	RunnerConfig config.RunnerConfiguration
	Ctx          context.Context
	Cli          *command.SauceCtlCli
}
