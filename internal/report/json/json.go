package json

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/build"
	"github.com/saucelabs/saucectl/internal/report"
)

// Reporter represents struct to report in json format
type Reporter struct {
	Service    build.Reader
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
	r.cleanup()
	r.buildData()
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

// cleanup removes any information that isn't relevant in the rendered report. Particularly when it comes to
// artifacts, this reporter is only interested in those that have been persisted to the file system.
func (r *Reporter) cleanup() {
	for i, result := range r.Results {
		var artifacts []report.Artifact
		for _, a := range result.Artifacts {
			if a.FilePath == "" {
				continue
			}
			artifacts = append(artifacts, a)
		}
		r.Results[i].Artifacts = artifacts
	}
}

// Reset resets the reporter to its initial state. This action will delete all test results.
func (r *Reporter) Reset() {
	r.Results = make([]report.TestResult, 0)
}

// ArtifactRequirements returns a list of artifact types this reporter requires to create a proper report.
func (r *Reporter) ArtifactRequirements() []report.ArtifactType {
	return nil
}

func (r *Reporter) buildData() {
	if len(r.Results) < 1 {
		return
	}

	var vdcJobURL string
	var rdcJobURL string
	for _, result := range r.Results {
		if !result.RDC && result.URL != "" {
			vdcJobURL = result.URL
			break
		}
	}
	for _, result := range r.Results {
		if result.RDC && result.URL != "" {
			rdcJobURL = result.URL
			break
		}
	}
	vdcBuildURL := r.getBuildURL(vdcJobURL, build.VDC)
	rdcBuildURL := r.getBuildURL(rdcJobURL, build.RDC)
	for i, result := range r.Results {
		if !result.RDC {
			result.BuildURL = vdcBuildURL
		} else {
			result.BuildURL = rdcBuildURL
		}
		r.Results[i] = result
	}
}

func (r *Reporter) getBuildURL(jobURL string, buildSource build.Source) string {
	pURL, err := url.Parse(jobURL)
	if err != nil {
		log.Debug().Err(err).Msgf("Failed to parse job url (%s)", jobURL)
		return ""
	}
	p := strings.Split(pURL.Path, "/")
	jID := p[len(p)-1]

	bID, err := r.Service.GetBuildID(context.Background(), jID, buildSource)
	if err != nil {
		log.Debug().Err(err).Msgf("Failed to retrieve build id for job (%s)", jID)
		return ""
	}

	return fmt.Sprintf("%s://%s/builds/%s/%s", pURL.Scheme, pURL.Host, buildSource, bID)
}
