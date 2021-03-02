package saucecloud

import (
	"fmt"

	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/testcafe"
)

// TestcafeRunner represents the SauceLabs cloud implementation
type TestcafeRunner struct {
	CloudRunner
	Project testcafe.Project
}

// RunProject runs the defined tests on sauce cloud
func (r *TestcafeRunner) RunProject() (int, error) {
	exitCode := 1

	fileID, err := r.archiveAndUpload(r.Project, []string{r.Project.Testcafe.ProjectPath})
	if err != nil {
		return exitCode, err
	}
	passed := r.runSuites(fileID)
	if passed {
		return 0, nil
	}

	return exitCode, nil
}

func (r *TestcafeRunner) runSuites(fileID string) bool {
	jobOpts, results := r.createWorkerPool(r.Project.Sauce.Concurrency)
	defer close(results)

	// Submit suites to work on
	for _, s := range r.Project.Suites {
		jobOpts <- job.StartOptions{
			App:              fmt.Sprintf("storage:%s", fileID),
			Suite:            s.Name,
			Framework:        "testcafe",
			FrameworkVersion: r.Project.Testcafe.Version,
			BrowserName:      s.BrowserName,
			BrowserVersion:   s.BrowserVersion,
			PlatformName:     s.PlatformName,
			Name:             fmt.Sprintf("%s - %s", r.Project.Sauce.Metadata.Name, s.Name),
			Build:            r.Project.Sauce.Metadata.Build,
			Tags:             r.Project.Sauce.Metadata.Tags,
			Tunnel: job.TunnelOptions{
				ID:     r.Project.Sauce.Tunnel.ID,
				Parent: r.Project.Sauce.Tunnel.Parent,
			},
			ScreenResolution: s.ScreenResolution,
			RunnerVersion:    r.Project.RunnerVersion,
		}
	}
	close(jobOpts)

	return r.collectResults(results, len(r.Project.Suites))
}
