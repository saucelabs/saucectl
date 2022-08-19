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

	for _, s := range r.Project.Suites {
		suite := s

		go func() {
			// TODO: Choose correct api based on Suite config
			resp, err := r.Client.RunAllSync(context.Background(), suite.Project, "json", "")
			if err != nil {
				log.Error().Err(err).Msg("Failed to run")
			}

			results <- resp
		}()
	}

	// TODO: 3. Collect results
	r.collectResults(len(r.Project.Suites), results)
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
