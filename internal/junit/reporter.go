package junit

import (
	"encoding/xml"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/report"
	"os"
	"strconv"
	"sync"
)

type Reporter struct {
	TestResults []report.TestResult
	Path        string
	lock        sync.Mutex
}

// Add adds the test result to the summary.
func (r *Reporter) Add(t report.TestResult) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.TestResults = append(r.TestResults, t)
}

func (r *Reporter) Render() {
	r.lock.Lock()
	defer r.lock.Unlock()

	tt := TestSuites{}
	for _, v := range r.TestResults {
		t := TestSuite{
			Name: v.Name,
			Time: strconv.Itoa(int(v.Duration.Seconds())),
		}

		t.Properties = append(t.Properties, extractProperties(v)...)

		for _, a := range v.Artifacts {
			if a.AssetType != report.JUnitArtifact {
				continue
			}

			if a.Error != nil {
				t.Errors++
				log.Warn().Err(a.Error).Str("suite", v.Name).Msg("Failed to download junit report. Summary may be incorrect!")
				continue
			}

			jsuites, err := Parse(a.Body)
			if err != nil {
				t.Errors++
				log.Warn().Err(err).Str("suite", v.Name).Msg("Failed to parse junit report. Summary may be incorrect!")
				continue
			}

			for _, ts := range jsuites.TestSuites {
				t.Tests += ts.Tests
				t.Failures += ts.Failures
				t.Errors += ts.Errors
				t.TestCases = append(t.TestCases, ts.TestCases...)
			}
		}

		tt.Tests += t.Tests
		tt.Failures += t.Failures
		tt.Errors += t.Errors
		tt.TestSuites = append(tt.TestSuites, t)
	}

	b, err := xml.MarshalIndent(tt, "", "  ")
	if err != nil {
		log.Err(err).Msg("Failed to create junit report.")
		return
	}

	f, err := os.Create(r.Path)
	if err != nil {
		log.Err(err).Msg("Failed to render junit report.")
		return
	}
	defer f.Close()

	_, _ = f.Write(b)
	_, _ = fmt.Fprint(f, "\n")
}

func extractProperties(r report.TestResult) []Property {
	props := []Property{
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

	var filtered []Property
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
