package json

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"

	"github.com/rs/zerolog/log"
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
		log.Error().Msgf("failed to generate test result (%v)", err)
		return
	}

	if r.WebhookURL != "" {
		resp, err := http.Post(r.WebhookURL, "application/json", bytes.NewBuffer(body))
		if err != nil {
			log.Error().Err(err).Str("webhook", r.WebhookURL).Msg("failed to send test result to webhook.")
		} else {
			webhookBody, _ := io.ReadAll(resp.Body)
			if resp.StatusCode >= http.StatusBadRequest {
				log.Error().Str("webhook", r.WebhookURL).Msgf("failed to send test result to webhook, status: '%d', msg:'%v'", resp.StatusCode, string(webhookBody))
			}
			if resp.StatusCode%100 == 2 {
				log.Info().Str("webhook", r.WebhookURL).Msgf("test result has been sent successfully to webhook, msg: '%v'.", string(webhookBody))
			}
		}
	}

	if r.Filename != "" {
		err = os.WriteFile(r.Filename, body, 0666)
		if err != nil {
			log.Error().Err(err).Msgf("failed to write test result to %s", r.Filename)
		}
	}
}

// Reset resets the reporter to its initial state. This action will delete all test results.
func (r *Reporter) Reset() {
	r.Results = make([]report.TestResult, 0)
}

// ArtifactRequirements returns a list of artifact types this reporter requires to create a proper report.
func (r *Reporter) ArtifactRequirements() []report.ArtifactType {
	return []report.ArtifactType{report.JSONArtifact}
}

// NeedParents
func (r *Reporter) NeedParents() bool {
	return false
}
