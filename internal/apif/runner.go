package apif

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

// ApifRunner represents an executor for api tests
type ApifRunner struct {
	Project       Project
	Client        apitesting.Client
	Region        region.Region
	Reporters     []report.Reporter
	Async         bool
	TunnelService tunnel.Service
}

// RunProject runs the tests defined in apif.Project
func (r *ApifRunner) RunProject() (int, error) {
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

func (r *ApifRunner) runSuites() bool {
	results := make(chan []apitesting.TestResult)
	expected := 0

	for _, s := range r.Project.Suites {
		suite := s

		if len(suite.Tags) == 0 && len(suite.Tests) == 0 {
			go func() {
				log.Info().Str("project", suite.HookId).Msg("Running project.")

				var resp []apitesting.TestResult
				var err error

				if r.Async {
					resp, err = r.Client.RunAllAsync(context.Background(), suite.HookId, r.Project.Sauce.Metadata.Build, r.Project.Sauce.Tunnel)
				} else {
					resp, err = r.Client.RunAllSync(context.Background(), suite.HookId, r.Project.Sauce.Metadata.Build, r.Project.Sauce.Tunnel)
				}
				if err != nil {
					log.Error().Err(err).Msg("Failed to run project.")
				}

				results <- resp
			}()
			expected++
		} else {
			for _, t := range suite.Tests {
				test := t
				go func() {
					log.Info().Str("test", test).Str("project", suite.HookId).Msg("Running test.")
					var resp []apitesting.TestResult
					var err error

					if r.Async {
						resp, err = r.Client.RunTestAsync(context.Background(), suite.HookId, test, r.Project.Sauce.Metadata.Build, r.Project.Sauce.Tunnel)
					} else {
						resp, err = r.Client.RunTestSync(context.Background(), suite.HookId, test, r.Project.Sauce.Metadata.Build, r.Project.Sauce.Tunnel)
					}

					if err != nil {
						log.Error().Err(err).Msg("Failed to run test.")
					}
					results <- resp
				}()
				expected++
			}

			for _, t := range suite.Tags {
				tag := t
				go func() {
					log.Info().Str("tag", tag).Str("project", suite.HookId).Msg("Running tag.")

					var resp []apitesting.TestResult
					var err error

					if r.Async {
						resp, err = r.Client.RunTagAsync(context.Background(), suite.HookId, tag, r.Project.Sauce.Metadata.Build, r.Project.Sauce.Tunnel)
					} else {
						resp, err = r.Client.RunTagSync(context.Background(), suite.HookId, tag, r.Project.Sauce.Metadata.Build, r.Project.Sauce.Tunnel)
					}
					if err != nil {
						log.Error().Err(err).Msg("Failed to run tag.")
					}
					results <- resp
				}()
				expected++
			}
		}
	}

	return r.collectResults(expected, results)
}

func (r *ApifRunner) collectResults(expected int, results chan []apitesting.TestResult) bool {
	inProgress := expected
	passed := true

	done := make(chan interface{})
	go func(r *ApifRunner) {
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

			if testResult.Async {
				testName = testResult.Project.Name

				log.Info().
					Str("project", testResult.Project.Name).
					Str("report", fmt.Sprintf("%s/api-testing/project/%s/event/%s", r.Region.AppBaseURL(), testResult.Project.ID, testResult.EventID)).
					Msg("Async test started.")
			} else {
				testName = fmt.Sprintf("%s - %s", testResult.Project.Name, testResult.Test.Name)

				log.Info().
					Int("failures", testResult.FailuresCount).
					Str("project", testResult.Project.Name).
					Str("report", fmt.Sprintf("%s/api-testing/project/%s/event/%s", r.Region.AppBaseURL(), testResult.Project.ID, testResult.EventID)).
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
					Name:   testName,
					Status: status,
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
