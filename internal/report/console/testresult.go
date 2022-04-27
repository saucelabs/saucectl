package console

import (
	"encoding/json"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/report"
)

type TestResult struct {
	Result []report.TestResult
}

func (j *TestResult) Add(t report.TestResult) {
	j.Result = append(j.Result, t)
}

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

func (j *TestResult) Reset() {
	j.Result = []report.TestResult{}
}

func (j *TestResult) ArtifactRequirements() []report.ArtifactType {
	return nil
}
