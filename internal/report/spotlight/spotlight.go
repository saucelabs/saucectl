package spotlight

import (
	"fmt"
	"io"
	"sync"

	"github.com/fatih/color"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/junit"
	"github.com/saucelabs/saucectl/internal/report"
)

// Reporter implements report.Reporter and highlights the most important test
// results.
type Reporter struct {
	TestResults []report.TestResult
	Dst         io.Writer
	lock        sync.Mutex
}

// Add adds the test result that can be rendered by Render.
func (r *Reporter) Add(t report.TestResult) {
	r.lock.Lock()
	defer r.lock.Unlock()

	// skip in-progress jobs
	if !job.Done(t.Status) && !t.TimedOut {
		return
	}
	// skip passed jobs
	if t.Status == job.StatePassed {
		return
	}

	if t.TimedOut {
		t.Status = job.StateUnknown
	}

	r.TestResults = append(r.TestResults, t)
}

// Render renders out a test summary.
func (r *Reporter) Render() {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.println()
	rl := color.New(color.FgBlue, color.Underline, color.Bold).Sprintf("Spotlight:")

	if len(r.TestResults) == 0 {
		r.printf("  %s Nothing stands out!\n", rl)
		return
	}

	r.printf("  %s\n", rl)
	r.println()

	for _, ts := range r.TestResults {
		r.println("", jobStatusSymbol(ts.Status), ts.Name)
		r.println("   ● URL:", ts.URL)

		var junitReports []junit.TestSuites
		for _, attempt := range ts.Attempts {
			junitReports = append(junitReports, attempt.TestSuites)
		}
		if len(junitReports) > 0 {
			junitReport := junit.MergeReports(junitReports...)
			testCases := junitReport.TestCases()

			var failedTests []string
			for _, tc := range testCases {
				if tc.IsError() || tc.IsFailure() {
					failedTests = append(failedTests, fmt.Sprintf("%s %s › %s", testCaseStatusSymbol(tc), tc.ClassName, tc.Name))
				}
				// only show the first 5 failed tests to conserve space
				if len(failedTests) == 5 {
					break
				}
			}

			if len(failedTests) > 0 {
				r.println("   ● Failed Tests: (showing max. 5)")
				for _, test := range failedTests {
					r.println("    ", test)
				}
			}
			r.println()
		}
	}
}

func (r *Reporter) println(a ...any) {
	_, _ = fmt.Fprintln(r.Dst, a...)
}

func (r *Reporter) printf(format string, a ...any) {
	_, _ = fmt.Fprintf(r.Dst, format, a...)
}

// Reset resets the reporter to its initial state. This action will delete all test results.
func (r *Reporter) Reset() {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.TestResults = make([]report.TestResult, 0)
}

// ArtifactRequirements returns a list of artifact types this reporter requires
// to create a proper report.
func (r *Reporter) ArtifactRequirements() []report.ArtifactType {
	return []report.ArtifactType{report.JUnitArtifact}
}

func jobStatusSymbol(status string) string {
	switch status {
	case job.StatePassed, job.StateComplete:
		return color.GreenString("✔")
	case job.StateInProgress, job.StateQueued, job.StateNew:
		return color.BlueString("*")
	default:
		return color.RedString("✖")
	}
}

func testCaseStatusSymbol(tc junit.TestCase) string {
	if tc.IsError() || tc.IsFailure() {
		return color.RedString("✖")
	}
	if tc.IsSkipped() {
		return color.YellowString("-")
	}
	return color.GreenString("✔")
}
