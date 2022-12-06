package apitest

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/apitesting"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report"
	"github.com/saucelabs/saucectl/internal/tunnel"
)

var pollMaximumWait = time.Second * 180
var pollWaitTime = time.Second * 5

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

func (r *Runner) runSuites() bool {
	results := make(chan []apitesting.TestResult)
	expected := 0

	for _, s := range r.Project.Suites {
		suite := s
		var resp apitesting.AsyncResponse
		var err error

		// If no tags or no tests are defined for the suite, run all tests for a hookId
		if len(suite.Tags) == 0 && len(suite.Tests) == 0 {
			log.Info().Str("hookId", suite.HookID).Msg("Running project.")

			resp, err = r.Client.RunAllAsync(context.Background(), suite.HookID, r.Project.Sauce.Metadata.Build, r.Project.Sauce.Tunnel)
			if err != nil {
				log.Error().Err(err).Msg("Failed to run project.")
			}

			if r.Async {
				r.fetchTestDetails(suite.HookID, resp.EventIDs, resp.TestIDs, results)
			} else {
				r.startPollingAsyncResponse(suite.HookID, resp.EventIDs, results)
			}
			expected += len(resp.EventIDs)
		} else {
			for _, t := range suite.Tests {
				test := t
				log.Info().Str("test", test).Str("hookId", suite.HookID).Msg("Running test.")

				resp, err = r.Client.RunTestAsync(context.Background(), suite.HookID, test, r.Project.Sauce.Metadata.Build, r.Project.Sauce.Tunnel)

				if err != nil {
					log.Error().Err(err).Msg("Failed to run test.")
				}
				if r.Async {
					r.fetchTestDetails(suite.HookID, resp.EventIDs, resp.TestIDs, results)
				} else {
					r.startPollingAsyncResponse(suite.HookID, resp.EventIDs, results)
				}
				expected += len(resp.EventIDs)
			}

			for _, t := range suite.Tags {
				tag := t
				log.Info().Str("tag", tag).Str("hookId", suite.HookID).Msg("Running tag.")

				resp, err = r.Client.RunTagAsync(context.Background(), suite.HookID, tag, r.Project.Sauce.Metadata.Build, r.Project.Sauce.Tunnel)
				if err != nil {
					log.Error().Err(err).Msg("Failed to run tag.")
				}
				if r.Async {
					r.fetchTestDetails(suite.HookID, resp.EventIDs, resp.TestIDs, results)
				} else {
					r.startPollingAsyncResponse(suite.HookID, resp.EventIDs, results)
				}
				expected += len(resp.EventIDs)
			}
		}
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

func (r *Runner) startPollingAsyncResponse(hookID string, eventIDs []string, results chan []apitesting.TestResult) {
	project, _ := r.Client.GetProject(context.Background(), hookID)

	for _, eventID := range eventIDs {
		go func(lEventId string) {
			timeout := (time.Now()).Add(pollMaximumWait)

			for {
				result, err := r.Client.GetEventResult(context.Background(), hookID, lEventId)

				if err == nil {
					results <- []apitesting.TestResult{result}
					break
				}
				if err.Error() != "event not found" {
					results <- []apitesting.TestResult{{
						EventID:       lEventId,
						FailuresCount: 1,
					}}
					break
				}
				if timeout.Before(time.Now()) {
					reportURL := fmt.Sprintf("%s/api-testing/project/%s/event/%s", r.Region.AppBaseURL(), project.ID, eventID)
					log.Warn().
						Str("project", project.Name).
						Str("report", fmt.Sprintf("%s/api-testing/project/%s/event/%s", r.Region.AppBaseURL(), project.ID, eventID)).
						Str("report", reportURL).
						Msg("Test did not finished before timeout.")
					results <- []apitesting.TestResult{{
						Project:  project,
						EventID:  lEventId,
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
			if testResult.FailuresCount > 0 {
				status = job.StateFailed
				passed = false
			} else if testResult.Async {
				status = job.StateInProgress
			} else if testResult.TimedOut {
				status = job.StateInProgress
				passed = false
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
