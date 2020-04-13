package runner

import (
	"context"
	"time"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/config"
)

const logDir = "/var/log/cont"

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
	Context() context.Context
	CLI() *command.SauceCtlCli

	Setup() error
	Run() (int, error)
	Teardown(logDir string) error
}

type baseRunner struct {
	config  config.Configuration
	context context.Context
	cli     *command.SauceCtlCli

	startTime int64
}

func (r baseRunner) Context() context.Context {
	return r.context
}

func (r baseRunner) CLI() *command.SauceCtlCli {
	return r.cli
}

const (
	// Local test execution
	Local = iota
	// CI test execution
	CI
)

// ToDo(Christian) detect target dir based on image
const targetDir = "/home/testrunner/tests"

// New creates a new testrunner object
func New(runnerType int, config config.Configuration, cli *command.SauceCtlCli) Testrunner {
	ctx := context.Background()

	if runnerType == Local {
		return localRunner{baseRunner{config, ctx, cli, makeTimestamp()}, ""}
	}

	return ciRunner{baseRunner{config, ctx, cli, makeTimestamp()}}
}

func makeTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
