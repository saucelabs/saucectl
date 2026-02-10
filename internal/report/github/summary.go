package github

import (
	"fmt"
	"os"
	"time"

	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/report"
)

// Reporter generates Job Summaries for GitHub.
// https://github.blog/2022-05-09-supercharging-github-actions-with-job-summaries/
type Reporter struct {
	startTime       time.Time
	stepSummaryFile string
	results         []report.TestResult
}

func NewJobSummaryReporter() Reporter {
	return Reporter{
		startTime:       time.Now(),
		stepSummaryFile: os.Getenv("GITHUB_STEP_SUMMARY"),
	}
}

func (r *Reporter) isActive() bool {
	return r.stepSummaryFile != ""
}

func (r *Reporter) Add(t report.TestResult) {
	if !r.isActive() {
		return
	}
	r.results = append(r.results, t)
}

func hasDevice(results []report.TestResult) bool {
	for _, t := range results {
		if t.DeviceName != "" {
			return true
		}
	}
	return false
}

func (r *Reporter) Render() {
	if !r.isActive() {
		return
	}

	endTime := time.Now()
	hasDevices := hasDevice(r.results)
	errors := 0
	inProgress := 0

	content := renderHeader(hasDevices)
	for _, result := range r.results {
		if result.Status == job.StateInProgress || result.Status == job.StateNew {
			inProgress++
		}
		if result.Status == job.StateFailed || result.Status == job.StateError {
			errors++
		}
		content += renderTestResult(result, hasDevices)
	}
	content += renderFooter(errors, inProgress, len(r.results), endTime.Sub(r.startTime))

	err := os.WriteFile(r.stepSummaryFile, []byte(content), 0x644)
	if err != nil {
		fmt.Printf("Unable to save summary: %v\n", err)
	}
}

func (r *Reporter) Reset() {
	if !r.isActive() {
		return
	}
}

func (r *Reporter) ArtifactRequirements() []report.ArtifactType {
	return []report.ArtifactType{}
}

func renderHeader(hasDevices bool) string {
	deviceTitle := ""
	deviceSeparator := ""
	if hasDevices {
		deviceTitle = " Device |"
		deviceSeparator = " --- |"
	}
	content := fmt.Sprintf("| | Name | Duration | Status | Browser | Platform |%s Attempts |\n", deviceTitle)
	content += fmt.Sprintf("| --- | --- | --- | --- | --- | --- |%s --- |\n", deviceSeparator)
	return content
}

func statusToEmoji(status string) string {
	switch status {
	case job.StateInProgress, job.StateNew:
		return ":clock10:"
	case job.StatePassed, job.StateComplete:
		return ":white_check_mark:"
	case job.StateUnknown:
		return ":interrobang:"
	case job.StateError, job.StateFailed:
		return ":x:"
	default:
		return ":warning:"
	}
}

func renderTestResult(t report.TestResult, hasDevices bool) string {
	content := ""

	mark := statusToEmoji(t.Status)
	deviceValue := ""
	if hasDevices {
		deviceValue = fmt.Sprintf(" %s |", t.DeviceName)
	}

	content += fmt.Sprintf("| %s | [%s](%s) | %.0fs | %s | %s | %s |%s %d |\n",
		mark, t.Name, t.URL, t.Duration.Seconds(), t.Status, t.Browser, t.Platform, deviceValue, len(t.Attempts))
	return content
}

func renderFooter(errors, inProgress, tests int, dur time.Duration) string {
	if errors != 0 {
		relative := float64(errors) / float64(tests) * 100
		return fmt.Sprintf("\n:x: %d of %d suites have failed (%.0f%%) in %s\n\n", errors, tests, relative, dur.Truncate(1*time.Second))
	}
	if inProgress != 0 {
		return fmt.Sprintf("\n:clock10: All suites have launched in %s\n\n", dur.Truncate(1*time.Second))
	}
	return fmt.Sprintf("\n:white_check_mark: All suites have passed in %s\n\n", dur.Truncate(1*time.Second))
}
