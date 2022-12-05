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

			r.startPollingAsyncResponse(suite.HookID, resp.EventIDs, results)
			expected += len(resp.EventIDs)
		} else {
			for _, t := range suite.Tests {
				test := t
				log.Info().Str("test", test).Str("hookId", suite.HookID).Msg("Running test.")

				resp, err = r.Client.RunTestAsync(context.Background(), suite.HookID, test, r.Project.Sauce.Metadata.Build, r.Project.Sauce.Tunnel)

				if err != nil {
					log.Error().Err(err).Msg("Failed to run test.")
				}
				r.startPollingAsyncResponse(suite.HookID, resp.EventIDs, results)
				expected += len(resp.EventIDs)
			}

			for _, t := range suite.Tags {
				tag := t
				log.Info().Str("tag", tag).Str("hookId", suite.HookID).Msg("Running tag.")

				resp, err = r.Client.RunTagAsync(context.Background(), suite.HookID, tag, r.Project.Sauce.Metadata.Build, r.Project.Sauce.Tunnel)
				if err != nil {
					log.Error().Err(err).Msg("Failed to run tag.")
				}
				r.startPollingAsyncResponse(suite.HookID, resp.EventIDs, results)
				expected += len(resp.EventIDs)
			}
		}
	}

	return r.collectResults(expected, results)
}

func (r *Runner) startPollingAsyncResponse(hookID string, eventIDs []string, results chan []apitesting.TestResult) {
	for _, eventID := range eventIDs {
		go func(lEventId string) {
			// TODO: Implement timeout
			for {
				// TODO: Make Dynamic
				time.Sleep(5 * time.Second)

				result, err := r.Client.GetEventResult(context.Background(), hookID, lEventId)

				if err == nil {
					results <- []apitesting.TestResult{result}
					break
				}
				if err.Error() == "event not found" {
					continue
				}
				if err != nil {
					break
				}
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
				log.Info().Msgf("Suites in progress: %d", inProgress)
			}
		}
	}(r)

	for i := 0; i < expected; i++ {
		res := <-results

		inProgress--

		for _, testResult := range res {
			var testName string
			var reportUrl string

			if testResult.Async {
				testName = testResult.Project.Name
				reportUrl = fmt.Sprintf("%s/api-testing/project/%s/event/%s", r.Region.AppBaseURL(), testResult.Project.ID, testResult.EventID)
				log.Info().
					Str("project", testResult.Project.Name).
					Str("report", reportUrl).
					Msg("Async test started.")
			} else {
				testName = fmt.Sprintf("%s - %s", testResult.Project.Name, testResult.Test.Name)
				reportUrl = fmt.Sprintf("%s/api-testing/project/%s/event/%s", r.Region.AppBaseURL(), testResult.Project.ID, testResult.EventID)

				log.Info().
					Int("failures", testResult.FailuresCount).
					Str("project", testResult.Project.Name).
					Str("report", reportUrl).
					Str("test", testResult.Test.Name).
					Msg("Finished test.")
			}

			status := job.StatePassed
			if testResult.FailuresCount > 0 {
				status = job.StateFailed
				passed = false
			} else if testResult.Async {
				status = job.StateInProgress
			}

			for _, rep := range r.Reporters {
				rep.Add(report.TestResult{
					Name:     testName,
					URL:      reportUrl,
					Status:   status,
					Duration: time.Second * time.Duration(testResult.ExecutionTimeSeconds),
					Attempts: 1,
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
