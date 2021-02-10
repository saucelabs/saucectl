package mocks

import (
	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/internal/config"
)

// TestRunner is a mock to test runner functionalities
type TestRunner struct {
	HasProject  bool
	HasSetup    bool
	HasRun      bool
	HasFinished bool
}

// NewTestRunner creates a runner for unit testing purposes
func NewTestRunner(c config.Project, cli *command.SauceCtlCli) (*TestRunner, error) {
	runner := TestRunner{}
	return &runner, nil
}

// RunProject pretends to run tests defined by config.Project.
func (r *TestRunner) RunProject() (int, error) {
	r.HasProject = true
	return 123, nil
}

// Setup testrun
func (r *TestRunner) Setup() error {
	r.HasSetup = true
	return nil
}

// Run test
func (r *TestRunner) Run() (int, error) {
	r.HasRun = true
	return 123, nil
}

// Teardown test run
func (r *TestRunner) Teardown(logDir string) error {
	r.HasFinished = true
	return nil
}
