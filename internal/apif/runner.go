package apif

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/apitesting"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report"
)

type ApifRunner struct {
	Project   Project
	Client    apitesting.Client
	Region    region.Region
	Reporters []report.Reporter
}

func (r *ApifRunner) RunSuites() {
	results := make(chan []apitesting.SyncTestResult)
	expected := 0

	for _, s := range r.Project.Suites {
		suite := s

		if len(suite.Tags) == 0 && len(suite.Tests) == 0 {
			go func() {
				log.Info().Str("project", suite.Project).Msg("Running project.")
				resp, err := r.Client.RunAllSync(context.Background(), suite.Project, "json", "")
				if err != nil {
					log.Error().Err(err).Msg("Failed to run")
				}

				results <- resp
			}()
			expected++
		} else {
			for _, t := range suite.Tests {
				test := t
				go func() {
					log.Info().Str("test", test).Str("project", suite.Project).Msg("Running test.")
					resp, err := r.Client.RunTestSync(context.Background(), suite.Project, test, "json", "")
					if err != nil {
						log.Error().Err(err).Msg("Failed to run")
					}
					results <- resp
				}()
				expected++
			}

			for _, t := range suite.Tags {
				tag := t
				go func() {
					log.Info().Str("tag", tag).Str("project", suite.Project).Msg("Running tag.")
					resp, err := r.Client.RunTagSync(context.Background(), suite.Project, tag, "json", "")
					if err != nil {
						log.Error().Err(err).Msg("Failed to run")
					}
					results <- resp
				}()
				expected++
			}
		}
	}

	r.collectResults(expected, results)
}

func (r *ApifRunner) collectResults(expected int, results chan []apitesting.SyncTestResult) {
	inProgress := expected

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
			log.Info().
				Int("failures", testResult.FailuresCount).
				Str("project", testResult.Project.Name).
				Str("report", fmt.Sprintf("%s/api-testing/project/%s/event/%s", r.Region.AppBaseURL(), testResult.Project.ID, testResult.ID)).
				Str("test", testResult.Test.Name).
				Msg("Finished test.")

			status := "passed"
			if testResult.FailuresCount > 0 {
				status = "failed"
			}
			for _, rep := range r.Reporters {
				rep.Add(report.TestResult{
					Name: fmt.Sprintf("%s - %s", testResult.Project.Name, testResult.Test.Name),
					Status: status,
				})
			}
		}
	}
	close(done)

	for _, rep := range r.Reporters {
		rep.Render()
	}
}
