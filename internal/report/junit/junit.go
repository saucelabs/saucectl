package junit

import (
	"encoding/xml"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/junit"
	"github.com/saucelabs/saucectl/internal/report"
	"time"
)

// Reporter is a junit implementation for report.Reporter.
type Reporter struct {
	TestResults []report.TestResult
	Filename    string
	lock        sync.Mutex
}

// Add adds the test result to the summary.
func (r *Reporter) Add(t report.TestResult) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.TestResults = append(r.TestResults, t)
}

// Render renders out a test summary junit report to the destination of Reporter.Filename.
func (r *Reporter) Render() {
	r.lock.Lock()
	defer r.lock.Unlock()

	tt := junit.TestSuites{}
	for _, v := range r.TestResults {
		t := junit.TestSuite{
			Name: v.Name,
			Time: strconv.Itoa(int(v.Duration.Seconds())),
		}
		t.Properties = append(t.Properties, extractProperties(v)...)

		var allTestSuites []junit.TestSuites
		for _, attempt := range v.Attempts {
			allTestSuites = append(allTestSuites, attempt.TestSuites)
		}

		combinedReports := junit.MergeReports(allTestSuites...)
		for _, ts := range combinedReports.TestSuites {
			t.TestCases = append(t.TestCases, ts.TestCases...)
		}

		tt.TestSuites = append(tt.TestSuites, t)
	}

	tt.Compute()

	b, err := xml.MarshalIndent(tt, "", "  ")
	if err != nil {
		log.Err(err).Msg("Failed to create junit report.")
		return
	}

	f, err := os.Create(r.Filename)
	if err != nil {
		log.Err(err).Msg("Failed to render junit report.")
		return
	}
	defer f.Close()

	if _, err = f.Write(b); err != nil {
		log.Err(err).Msg("Failed to render junit report.")
		return
	}
	_, _ = fmt.Fprint(f, "\n")
}

func extractProperties(r report.TestResult) []junit.Property {
	props := []junit.Property{
		{
			Name:  "url",
			Value: r.URL,
		},
		{
			Name:  "browser",
			Value: r.Browser,
		},
		{
			Name:  "device",
			Value: r.DeviceName,
		},
		{
			Name:  "platform",
			Value: r.Platform,
		},
	}

	// Add retry attempt properties when more than one attempt was made.
	if len(r.Attempts) > 1 {
		props = append(props,
			junit.Property{Name: "retried", Value: "true"},
			junit.Property{Name: "retries", Value: strconv.Itoa(len(r.Attempts))},
		)
		for i, a := range r.Attempts {
			prefix := fmt.Sprintf("attempt.%d.", i)
			props = append(props,
				junit.Property{Name: prefix + "id", Value: a.ID},
				junit.Property{Name: prefix + "status", Value: a.Status},
				junit.Property{Name: prefix + "duration", Value: fmt.Sprintf("%.0f", a.Duration.Truncate(time.Second).Seconds())},
			)
		}
	}

	var filtered []junit.Property
	for _, p := range props {
		// we don't want to display properties with empty values
		if p.Value == "" {
			continue
		}

		filtered = append(filtered, p)
	}

	return filtered
}

// Reset resets the reporter to its initial state. This action will delete all test results.
func (r *Reporter) Reset() {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.TestResults = make([]report.TestResult, 0)
}

// ArtifactRequirements returns a list of artifact types are this reporter requires to create a proper report.
func (r *Reporter) ArtifactRequirements() []report.ArtifactType {
	return []report.ArtifactType{report.JUnitArtifact}
}
