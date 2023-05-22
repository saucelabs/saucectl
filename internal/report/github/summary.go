package github

import (
	"fmt"
	"os"

	"github.com/saucelabs/saucectl/internal/report"
)

type Reporter struct {
	stepSummaryFile string
	results         []report.TestResult
}

func NewGithubSummary() Reporter {
	return Reporter{
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

func (r *Reporter) Render() {
	if !r.isActive() {
		return
	}
	fd, err := os.Open(r.stepSummaryFile)
	if err != nil {
		return
	}
	renderHeader(fd)
	for _, result := range r.results {
		renderTestResult(fd, result)
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

func renderHeader(f *os.File) {
	fmt.Fprint(f, "| | Name | Duration | Status | Browser | Platform | Device |\n")
	fmt.Fprint(f, "| --- | --- | --- | --- | --- | --- | --- |\n")
}

func renderTestResult(f *os.File, t report.TestResult) {
	mark := ":x:"
	if t.Status == "passed" {
		mark = ":white_check_mark:"
	}
	fmt.Fprintf(f, "| %s | [%s](%s) | %.0fs | %s | %s | %s | %s |\n",
		mark, t.Name, t.URL, t.Duration.Seconds(), t.Status, t.Browser, t.Platform, t.DeviceName)
}
