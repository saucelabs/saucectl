package captor

import (
	"github.com/saucelabs/saucectl/internal/report"
	"sync"
)

// Default is the default, global instance of Reporter. Use judiciously.
var Default = Reporter{}

// Reporter is a simple implementation for report.Reporter, a no-output reporter for capturing test results.
type Reporter struct {
	TestResults []report.TestResult
	lock        sync.Mutex
}

// Add adds the test result.
func (r *Reporter) Add(t report.TestResult) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.TestResults = append(r.TestResults, t)
}

// GetAll returns all added test results, unless they've been purged via Reset().
func (r *Reporter) GetAll() []report.TestResult {
	return r.TestResults
}

// Render does nothing.
func (r *Reporter) Render() {
	//	no op
}

// Reset resets the reporter to its initial state. This action will delete all test results.
func (r *Reporter) Reset() {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.TestResults = make([]report.TestResult, 0)
}

// ArtifactRequirements returns a list of artifact types are this reporter requires to create a proper report.
func (r *Reporter) ArtifactRequirements() []report.ArtifactType {
	return nil
}
