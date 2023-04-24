package retry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/saucelabs/saucectl/internal/progress"
	"github.com/saucelabs/saucectl/internal/saucecloud/zip"
	"github.com/saucelabs/saucectl/internal/saucereport"
	"github.com/saucelabs/saucectl/internal/storage"
	"github.com/saucelabs/saucectl/internal/testcafe"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type BasicRetrier struct {
	VDCReader       job.Reader
	ProjectUploader storage.AppService

	CypressProject    cypress.Project
	PlaywrightProject playwright.Project
	TestcafeProject   testcafe.Project
}

func (r *BasicRetrier) Retry(jobOpts chan<- job.StartOptions, opt job.StartOptions, previous job.Job) {
	if r.VDCReader != nil && opt.SmartRetry.FailedTestsOnly {
		r.RetryFailedTests(jobOpts, opt, previous)
		return
	}

	log.Info().Str("suite", opt.DisplayName).
		Str("attempt", fmt.Sprintf("%d of %d", opt.Attempt+1, opt.Retries+1)).
		Msg("Retrying suite.")
	jobOpts <- opt
}

func (r *BasicRetrier) RetryFailedTests(jobOpts chan<- job.StartOptions, opt job.StartOptions, previous job.Job) {
	failedTests, err := r.getFailedTests(previous)
	if err != nil {
		log.Debug().Err(err).Msgf(msg.UnableToFetchFile, saucereport.SauceReportFileName)
		log.Info().Msg(msg.SkippingSmartRetries)
		jobOpts <- opt
		return
	}
	tempDir, err := os.MkdirTemp(os.TempDir(), "saucectl-app-payload-")
	if err != nil {
		log.Debug().Err(err).Msgf(msg.UnableToFetchFile, saucereport.SauceReportFileName)
		log.Info().Msg(msg.SkippingSmartRetries)
		jobOpts <- opt
		return
	}

	var runnerFile string
	var project interface{}
	switch opt.Framework {
	case cypress.Kind:
		r.CypressProject.SetTestGrep(opt.SuiteIndex, failedTests)
		project = r.CypressProject
	case playwright.Kind:
		r.PlaywrightProject.Suites[opt.SuiteIndex].Params.Grep = strings.Join(failedTests, "|")
		project = r.PlaywrightProject
	case testcafe.Kind:
		r.TestcafeProject.Suites[opt.SuiteIndex].Filter.TestGrep = strings.Join(failedTests, "|")
		project = r.TestcafeProject
	}

	runnerFile, err = zip.ArchiveRunnerConfig(project, tempDir)
	if err != nil {
		log.Debug().Err(err).Msgf(msg.UnableToFetchFile, saucereport.SauceReportFileName)
		log.Info().Msg(msg.SkippingSmartRetries)
		jobOpts <- opt
		return
	}

	fileURL, err := r.uploadConfig(runnerFile)
	if err != nil {
		log.Debug().Err(err).Msgf(msg.UnableToFetchFile, saucereport.SauceReportFileName)
		log.Info().Msg(msg.SkippingSmartRetries)
		jobOpts <- opt
		return
	}

	opt.OtherApps = []string{fmt.Sprintf("storage:%s", fileURL)}
	log.Info().Str("suite", opt.DisplayName).
		Str("attempt", fmt.Sprintf("%d of %d", opt.Attempt+1, opt.Retries+1)).
		Msg("Retrying suite.")
	jobOpts <- opt
}

func (r *BasicRetrier) uploadConfig(filename string) (string, error) {
	filename, err := filepath.Abs(filename)
	if err != nil {
		return "", err
	}
	file, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", filename, err)
	}
	defer file.Close()

	progress.Show("Uploading runner config %s", filename)
	start := time.Now()
	resp, err := r.ProjectUploader.UploadStream(filepath.Base(filename), "", file)
	progress.Stop()
	if err != nil {
		return "", err
	}
	log.Info().Dur("durationMs", time.Since(start)).Str("storageId", resp.ID).Msg("Runner Config uploaded.")

	return resp.ID, nil
}

func (r *BasicRetrier) getFailedTests(job job.Job) ([]string, error) {
	var failedTests []string
	content, err := r.VDCReader.GetJobAssetFileContent(context.Background(), job.ID, saucereport.SauceReportFileName, false)
	if err != nil {
		return failedTests, err
	}
	report, err := saucereport.Parse(content)
	if err != nil {
		return failedTests, err
	}
	if report.Status == saucereport.StatusPassed || report.Status == saucereport.StatusSkipped {
		return failedTests, nil
	}
	for _, s := range report.Suites {
		failedTests = append(failedTests, collectFailedTests(s)...)
	}

	return failedTests, nil
}

func collectFailedTests(suite saucereport.Suite) []string {
	if len(suite.Suites) == 0 && len(suite.Tests) == 0 {
		return []string{}
	}
	if suite.Status == saucereport.StatusPassed || suite.Status == saucereport.StatusSkipped {
		return []string{}
	}

	var failedTests []string
	if len(suite.Suites) > 0 {
		for _, s := range suite.Suites {
			if s.Status == saucereport.StatusFailed {
				failedTests = append(failedTests, collectFailedTests(s)...)
			}
		}
		return failedTests
	}

	for _, t := range suite.Tests {
		if t.Status == saucereport.StatusFailed {
			failedTests = append(failedTests, t.Name)
		}
	}

	return failedTests
}
