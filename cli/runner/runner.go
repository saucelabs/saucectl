package runner

import (
	"context"
	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/config"
	"github.com/saucelabs/saucectl/cli/utils"
	"github.com/saucelabs/saucectl/internal/fleet"
	"os"
)

const logDir = "/var/log/cont"

var homeDir, _ = utils.GetHomeDir()

func getConfigYamlFile() string {
	if (os.Getenv("SAUCE_VM") == "") {
		return "config.yaml"
	}
	return "config-local.yaml"
}

// ConfigPath represents the path for the runner config.
var ConfigPath = homeDir + "/" + getConfigYamlFile()

// LogFiles contains the locations of log and resource files that are useful for reporting.
var LogFiles = []string{
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
	homeDir + "/videos/video.mp4",
	homeDir + "/docker.log",
}

// Testrunner describes the test runner interface
type Testrunner interface {
	RunProject() (int, error)
}

// BaseRunner contains common properties across all runners
type BaseRunner struct {
	Project      config.Project
	RunnerConfig config.RunnerConfiguration
	Ctx          context.Context
	Cli          *command.SauceCtlCli
	Sequencer    fleet.Sequencer
}
