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

func hasSomeDevice(results []report.TestResult) bool {
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

	hasDevices := hasSomeDevice(r.results)

	content := renderHeader(hasDevices)
	for _, result := range r.results {
		content += renderTestResult(result, hasDevices)
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

func renderHeader(hasDevices bool) string {
	deviceTitle := ""
	deviceSeparator := ""
	if hasDevices {
		deviceTitle = " Device |"
		deviceSeparator = " --- |"
	}
	content := fmt.Sprintf("| | Name | Duration | Status | Browser | Platform |%s\n", deviceTitle)
	content += fmt.Sprintf("| --- | --- | --- | --- | --- | --- |%s\n", deviceSeparator)
	return content
}

func renderTestResult(t report.TestResult, hasDevices bool) string {
	content := ""

	var mark string
	switch t.Status {
	case "in progress":
		mark = ":clock10:"
	case "passed":
		mark = ":white_check_mark:"
	default:
		mark = ":x:"
	}

	deviceValue := ""
	if hasDevices {
		deviceValue = fmt.Sprintf(" %s |", t.DeviceName)
	}

	content += fmt.Sprintf("| %s | [%s](%s) | %.0fs | %s | %s | %s |%s\n",
		mark, t.Name, t.URL, t.Duration.Seconds(), t.Status, t.Browser, t.Platform, deviceValue)
	return content
}
