package jsonresult

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/report"
)

// Reporter represents struct to report in json format
type Reporter struct {
	URL      string
	Filename string
	Results  []report.TestResult
	lock     sync.Locker
}

// Add adds a TestResult
func (r *Reporter) Add(t report.TestResult) {
	r.Results = append(r.Results, t)
}

// Render sends the result to specified webhook URL and log the result to the specified json file
func (r *Reporter) Render() {
	body, err := json.Marshal(r.Results)
	if err != nil {
		log.Error().Msgf("failed to generate test result (%v)", err)
		return
	}

	if r.URL != "" {
		resp, err := http.Post(r.URL, "application/json", bytes.NewBuffer(body))
		if err != nil {
			log.Error().Msgf("failed to send result (%v)", err)
		} else {
			if resp.StatusCode >= http.StatusInternalServerError {
				log.Error().Msg(msg.InternalServerError)
			}
			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				log.Error().Msgf(":'%d', msg:'%v'", resp.StatusCode, string(body))
			}
		}
	}

	if r.Filename != "" {
		err = os.WriteFile(r.Filename, body, 0666)
		if err != nil {
			log.Error().Msgf("failed to write test result to test_result.json (%v)", err)
		}
	}
}

// Reset resets the reporter to its initial state. This action will delete all test results.
func (r *Reporter) Reset() {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.URL = ""
	r.Filename = ""
	r.Results = make([]report.TestResult, 0)
}

// ArtifactRequirements returns a list of artifact types this reporter requires to create a proper report.
func (r *Reporter) ArtifactRequirements() []report.ArtifactType {
	return []report.ArtifactType{report.JSONArtifact}
}
