package json

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"

	"github.com/rs/zerolog/log"
	saucehttp "github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/report"
)

// Reporter represents struct to report in json format
type Reporter struct {
	WebhookURL string
	Filename   string
	Results    []report.TestResult
}

// Add adds a TestResult
func (r *Reporter) Add(t report.TestResult) {
	r.Results = append(r.Results, t)
}

// Render sends the result to specified webhook WebhookURL and log the result to the specified json file
func (r *Reporter) Render() {
	body, err := json.Marshal(r.Results)
	if err != nil {
		log.Err(err).Msg("failed to generate test result.")
		return
	}

	if r.WebhookURL != "" {
		resp, err := r.postWebhook(body)
		if err != nil {
			log.Err(err).Str("webhook", r.WebhookURL).Msg("failed to send test result to webhook.")
		} else {
			webhookBody, _ := io.ReadAll(resp.Body)
			if resp.StatusCode >= http.StatusBadRequest {
				log.Error().Str("webhook", r.WebhookURL).Msgf("failed to send test result to webhook, status: %d, msg: %q.", resp.StatusCode, string(webhookBody))
			}
			if resp.StatusCode%100 == 2 {
				log.Info().Str("webhook", r.WebhookURL).Msgf("test result has been sent successfully to webhook, msg: %q.", string(webhookBody))
			}
		}
	}

	if r.Filename != "" {
		err = os.WriteFile(r.Filename, body, 0666)
		if err != nil {
			log.Err(err).Msgf("failed to write test result to %q.", r.Filename)
		}
	}
}

func (r *Reporter) postWebhook(body []byte) (*http.Response, error) {
	req, err := saucehttp.NewRequestWithContext(context.Background(), http.MethodPost, r.WebhookURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	return http.DefaultClient.Do(req)
}

// Reset resets the reporter to its initial state. This action will delete all test results.
func (r *Reporter) Reset() {
	r.Results = make([]report.TestResult, 0)
}

// ArtifactRequirements returns a list of artifact types this reporter requires to create a proper report.
func (r *Reporter) ArtifactRequirements() []report.ArtifactType {
	return nil
}
