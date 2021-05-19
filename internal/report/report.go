package report

import "time"

// TestResult represents the test result.
type TestResult struct {
	Name       string
	Duration   time.Duration
	Passed     bool
	Browser    string
	Platform   string
	DeviceName string
}

// Reporter is the interface for rest result reporting.
type Reporter interface {
	// Add adds the TestResult to the reporter. TestResults added this way can then be rendered out by calling Render().
	Add(t TestResult)
	// Render renders the test results. The destination depends on the implementation.
	Render()
	// Reset resets the state of the reporter (e.g. remove any previously reported TestResults).
	Reset()
}
