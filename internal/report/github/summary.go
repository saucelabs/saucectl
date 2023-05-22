package github

import (
	"fmt"
	"os"
	"syscall"

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
	fd, err := os.OpenFile(r.stepSummaryFile, syscall.O_APPEND, 0x644)
	if err != nil {
		return
	}
	defer func() {
		err := fd.Sync()
		if err != nil {
			fmt.Printf("error while syncing: %v", err)
		}
		fd.Close()
	}()
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
	_, err := fmt.Fprint(f, "| | Name | Duration | Status | Browser | Platform | Device |\n")
	if err != nil {
		fmt.Printf("error while syncing: %v", err)
	}
	_, err = fmt.Fprint(f, "| --- | --- | --- | --- | --- | --- | --- |\n")
	if err != nil {
		fmt.Printf("error while syncing: %v", err)
	}
}

func renderTestResult(f *os.File, t report.TestResult) {
	mark := ":x:"
	if t.Status == "passed" {
		mark = ":white_check_mark:"
	}
	_, err := fmt.Fprintf(f, "| %s | [%s](%s) | %.0fs | %s | %s | %s | %s |\n",
		mark, t.Name, t.URL, t.Duration.Seconds(), t.Status, t.Browser, t.Platform, t.DeviceName)
	if err != nil {
		fmt.Printf("error while syncing: %v", err)
	}
}
