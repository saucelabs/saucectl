package apif

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/apitesting"
)

type ApifRunner struct {
	Project Project
	Client apitesting.Client
}

func (r *ApifRunner) RunSuites() {
	// TODO: 1. Make channels
	results := make(chan []apitesting.RunSyncResponse)
	expected := 0

	for _, s := range r.Project.Suites {
		suite := s

		if len(suite.Tags) == 0 && len(suite.Tests) == 0 {
			go func() {
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

	// TODO: 3. Collect results
	r.collectResults(expected, results)
}

func (r *ApifRunner) collectResults(expected int, results chan []apitesting.RunSyncResponse) {
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
				// if !r.interrupted {
				// 	log.Info().Msgf("Suites in progress: %d", inProgress)
				// }
			}
		}
	}(r)

	for i := 0; i < expected; i++ {
		res := <-results

		inProgress--

		for _, r := range res {
			log.Info().Int("failures", r.FailuresCount).Str("Project", r.Project.Name).Msg("Finished tests")
		}
	}
	close(done)
}
