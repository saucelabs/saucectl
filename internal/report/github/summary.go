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
	fmt.Printf("GITHUB_STEP_SUMMARY: %s", os.Getenv("GITHUB_STEP_SUMMARY"))
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

	content := renderHeader()
	for _, result := range r.results {
		content += renderTestResult(result)
	}

	err := os.WriteFile(r.stepSummaryFile, []byte(content), 0x644)
	if err != nil {
		fmt.Printf("Unable to save summary: %v", err)
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

func renderHeader() string {
	content := fmt.Sprint("| | Name | Duration | Status | Browser | Platform | Device |\n")
	content += fmt.Sprint("| --- | --- | --- | --- | --- | --- | --- |\n")
	return content
}

func renderTestResult(t report.TestResult) string {
	content := ""

	mark := ":x:"
	if t.Status == "passed" {
		mark = ":white_check_mark:"
	}
	content += fmt.Sprintf("| %s | [%s](%s) | %.0fs | %s | %s | %s | %s |\n",
		mark, t.Name, t.URL, t.Duration.Seconds(), t.Status, t.Browser, t.Platform, t.DeviceName)
	return content
}
