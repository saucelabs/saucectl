package apitest

import (
	"context"
	"fmt"
	"github.com/xtgo/uuid"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/apitesting"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report"
	"github.com/saucelabs/saucectl/internal/tunnel"
)

var pollDefaultWait = time.Second * 180
var pollWaitTime = time.Second * 5

var unitFileName = "unit.yaml"
var inputFileName = "input.yaml"

// Runner represents an executor for api tests
type Runner struct {
	Project       Project
	Client        apitesting.Client
	Region        region.Region
	Reporters     []report.Reporter
	Async         bool
	TunnelService tunnel.Service
}

// RunProject runs the tests defined in apitest.Project
func (r *Runner) RunProject() (int, error) {
	exitCode := 1
	if err := tunnel.ValidateTunnel(r.TunnelService, r.Project.Sauce.Tunnel.Name, r.Project.Sauce.Tunnel.Owner, tunnel.V2AlphaFilter, false); err != nil {
		return 1, err
	}

	passed := r.runSuites()
	if passed {
		exitCode = 0
	}
	return exitCode, nil
}

func hasUnitInputFiles(dir string) bool {
	st, err := os.Stat(path.Join(dir, unitFileName))
	if err != nil || st.IsDir() {
		return false
	}

	st, err = os.Stat(path.Join(dir, inputFileName))
	if err != nil || st.IsDir() {
		return false
	}
	return true
}

func matchPath(dir string, pathMatch []string) bool {
	if len(pathMatch) == 0 {
		return true
	}
	for _, v := range pathMatch {
		re, err := regexp.Compile(v)
		if err != nil {
			continue
		}
		if re.MatchString(dir) {
			return true
		}
	}
	return false
}

func findTests(rootDir string, testMatch []string) []string {
	var tests []string

	filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() {
			return nil
		}
		if !hasUnitInputFiles(path) {
			return nil
		}

		relPath, _ := filepath.Rel(rootDir, path)
		if matchPath(relPath, testMatch) {
			tests = append(tests, relPath)
		}
		return nil
	})
	return tests
}

func loadTest(unitPath string, inputPath string, suiteName string, testName string, tags []string) (apitesting.TestRequest, error) {
	unitContent, err := os.ReadFile(unitPath)
	if err != nil {
		return apitesting.TestRequest{}, err
	}
	inputContent, err := os.ReadFile(inputPath)
	if err != nil {
		return apitesting.TestRequest{}, err
	}
	return apitesting.TestRequest{
		Name:  fmt.Sprintf("%s - %s", suiteName, testName),
		Tags:  append([]string{}, tags...),
		Input: string(inputContent),
		Unit:  string(unitContent),
	}, nil
}

func (r *Runner) loadTests(s Suite, tests []string) []apitesting.TestRequest {
	var testRequests []apitesting.TestRequest

	for _, test := range tests {
		req, err := loadTest(
			path.Join(r.Project.RootDir, test, unitFileName),
			path.Join(r.Project.RootDir, test, inputFileName),
			s.Name,
			test,
			s.Tags)
		if err != nil {
			log.Warn().
				Str("testName", test).
				Err(err).
				Msg("Unable to load test.")
		}
		testRequests = append(testRequests, req)
	}
	return testRequests
}

func (r *Runner) runSuites() bool {
	results := make(chan []apitesting.TestResult)

	expected := 0

	for _, s := range r.Project.Suites {
		var eventIDs []string
		taskID := uuid.NewRandom().String()
		suite := s
		log.Info().
			Str("hookId", suite.HookID).
			Str("taskId", taskID).
			Str("suite", suite.Name).
			Bool("sequential", suite.Sequential).
			Msg("Starting suite")

		maximumWaitTime := pollDefaultWait
		if suite.Timeout != 0 {
			pollWaitTime = suite.Timeout
		}

		testNames := findTests(r.Project.RootDir, s.TestMatch)
		tests := r.loadTests(s, testNames)

		for _, test := range tests {
			log.Info().
				Str("hookId", suite.HookID).
				Str("testName", test.Name).
				Msg("Running test.")

			resp, err := r.Client.RunEphemeralAsync(context.Background(), suite.HookID, r.Project.Sauce.Metadata.Build, r.Project.Sauce.Tunnel, taskID, test)
			if err != nil {
				log.Error().
					Err(err).
					Str("testName", test.Name).
					Msg("Failed to run test.")
				continue
			}
			eventIDs = append(eventIDs, resp.EventIDs...)
			expected++
		}

		r.startPollingAsyncResponse(suite.HookID, eventIDs, results, maximumWaitTime)
	}
	return r.collectResults(expected, results)
}

func (r *Runner) fetchTestDetails(hookID string, eventIDs []string, testIDs []string, results chan []apitesting.TestResult) {
	project, _ := r.Client.GetProject(context.Background(), hookID)
	for _, eventID := range eventIDs {
		reportURL := fmt.Sprintf("%s/api-testing/project/%s/event/%s", r.Region.AppBaseURL(), project.ID, eventID)
		log.Info().
			Str("project", project.Name).
			Str("report", fmt.Sprintf("%s/api-testing/project/%s/event/%s", r.Region.AppBaseURL(), project.ID, eventID)).
			Str("report", reportURL).
			Msg("Async test started.")
	}

	for _, testID := range testIDs {
		go func(p apitesting.Project, testID string) {
			test, _ := r.Client.GetTest(context.Background(), hookID, testID)
			results <- []apitesting.TestResult{{
				Test:    test,
				Project: p,
				Async:   true,
			}}
		}(project, testID)
	}
}

func (r *Runner) startPollingAsyncResponse(hookID string, eventIDs []string, results chan []apitesting.TestResult, pollMaximumWait time.Duration) {
	project, _ := r.Client.GetProject(context.Background(), hookID)

	for _, eventID := range eventIDs {
		go func(lEventID string) {
			timeout := (time.Now()).Add(pollMaximumWait)

			for {
				result, err := r.Client.GetEventResult(context.Background(), hookID, lEventID)

				if err == nil {
					log.Info().
						Str("hookId", hookID).
						Str("testName", result.Test.Name).
						Msg("Finished test.")
					results <- []apitesting.TestResult{result}
					break
				}
				if err.Error() != "event not found" {
					results <- []apitesting.TestResult{{
						EventID:       lEventID,
						FailuresCount: 1,
					}}
					break
				}
				if timeout.Before(time.Now()) {
					reportURL := fmt.Sprintf("%s/api-testing/project/%s/event/%s", r.Region.AppBaseURL(), project.ID, lEventID)
					log.Warn().
						Str("project", project.Name).
						Str("report", fmt.Sprintf("%s/api-testing/project/%s/event/%s", r.Region.AppBaseURL(), project.ID, lEventID)).
						Str("report", reportURL).
						Msg("Test did not finish before timeout.")
					results <- []apitesting.TestResult{{
						Project:  project,
						EventID:  lEventID,
						Async:    true,
						TimedOut: true,
					}}
					break
				}
				time.Sleep(pollWaitTime)
			}
		}(eventID)
	}
}

func (r *Runner) collectResults(expected int, results chan []apitesting.TestResult) bool {
	inProgress := expected
	passed := true

	done := make(chan interface{})
	go func(r *Runner) {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				log.Info().Msgf("Tests in progress: %d", inProgress)
			}
		}
	}(r)

	for i := 0; i < expected; i++ {
		res := <-results

		inProgress--

		for _, testResult := range res {
			var reportURL string
			testName := buildTestName(testResult.Project, testResult.Test)

			if !testResult.Async {
				reportURL = fmt.Sprintf("%s/api-testing/project/%s/event/%s", r.Region.AppBaseURL(), testResult.Project.ID, testResult.EventID)

				log.Info().
					Int("failures", testResult.FailuresCount).
					Str("project", testResult.Project.Name).
					Str("report", reportURL).
					Str("test", testResult.Test.Name).
					Msg("Finished test.")
			}

			status := job.StatePassed
			if testResult.FailuresCount > 0 || testResult.TimedOut {
				status = job.StateFailed
				passed = false
			} else if testResult.Async {
				status = job.StateInProgress
			}

			for _, rep := range r.Reporters {
				rep.Add(report.TestResult{
					Name:      testName,
					URL:       reportURL,
					Status:    status,
					Duration:  time.Second * time.Duration(testResult.ExecutionTimeSeconds),
					StartTime: (time.Now()).Add(-time.Second * time.Duration(testResult.ExecutionTimeSeconds)),
					Attempts:  1,
				})
			}
		}
	}
	close(done)

	for _, rep := range r.Reporters {
		rep.Render()
	}

	return passed
}

func buildTestName(project apitesting.Project, test apitesting.Test) string {
	if test.Name != "" {
		return fmt.Sprintf("%s - %s", project.Name, test.Name)
	}
	return project.Name
}
