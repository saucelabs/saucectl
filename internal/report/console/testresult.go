package console

import (
	"encoding/json"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/report"
)

// TestResult represents struct to log in test_result.json
type TestResult struct {
	Result []report.TestResult
}

// Add adds a TestResult to the report
func (j *TestResult) Add(t report.TestResult) {
	j.Result = append(j.Result, t)
}

// Render writes the collected TestResult into test result file
func (j *TestResult) Render() {
	body, err := json.Marshal(j.Result)
	if err != nil {
		log.Error().Msgf("failed to generate test result: %s", err.Error())
		return
	}
	err = os.WriteFile("test_result.json", body, 0666)
	if err != nil {
		log.Error().Msgf("failed to write test result to test_result.json: %s", err.Error())
	}
}

// Reset resets the reporter to its initial state. This action will delete all test results.
func (j *TestResult) Reset() {
	j.Result = []report.TestResult{}
}

// ArtifactRequirements returns a list of artifact types are this reporter requires to create a proper report.
func (j *TestResult) ArtifactRequirements() []report.ArtifactType {
	return nil
}
