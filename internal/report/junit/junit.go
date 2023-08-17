package junit

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/junit"
	"github.com/saucelabs/saucectl/internal/report"
	"golang.org/x/exp/maps"
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

func parseJunitFiles(junits []report.Artifact) ([]junit.TestSuites, error) {
	var parsed []junit.TestSuites
	var errs []error
	for _, ju := range junits {
		if ju.Error != nil {
			errs = append(errs, fmt.Errorf("failed to retrieve junit file: %w", ju.Error))
			continue
		}
		ts, err := junit.Parse(ju.Body)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to parse junit file: %w", err))
			continue
		}
		parsed = append(parsed, ts)
	}
	if len(errs) > 0 {
		return parsed, fmt.Errorf("%d errors occured while evaluating junit files: %w", len(errs), errors.Join(errs...))
	}
	return parsed, nil
}

// reduceSuite updates "old" with values from "new".
func reduceSuite(old junit.TestSuite, new junit.TestSuite) junit.TestSuite {
	testMap := map[string]int{}
	for idx, tc := range old.TestCases {
		key := fmt.Sprintf(`%s.%s`, tc.ClassName, tc.Name)
		testMap[key] = idx
	}

	for _, tc := range new.TestCases {
		key := fmt.Sprintf(`%s.%s`, tc.ClassName, tc.Name)
		var idx int
		var ok bool
		if idx, ok = testMap[key]; !ok {
			log.Warn().Str("test", key).Msg("Sanity check failed when merging related junit test suites. New test encountered without prior history.")
			continue
		}
		old.TestCases[idx] = tc
	}
	old.Tests = len(old.TestCases)
	old.Errors = countErrors(old.TestCases)
	old.Skipped = countSkipped(old.TestCases)
	return old
}

func reduceJunitFiles(junits []junit.TestSuites) junit.TestSuites {
	suites := map[string]junit.TestSuite{}

	for _, junit := range junits {
		for _, suite := range junit.TestSuites {
			if _, ok := suites[suite.Name]; !ok {
				suites[suite.Name] = suite
				continue
			}
			suites[suite.Name] = reduceSuite(suites[suite.Name], suite)
		}
	}

	output := junit.TestSuites{}

	output.TestSuites = append(output.TestSuites, maps.Values(suites)...)
	return output
}

func countErrors(tcs []junit.TestCase) int {
	count := 0
	for _, tc := range tcs {
		if tc.Status == "error" {
			count++
		}
	}
	return count
}
func countSkipped(tcs []junit.TestCase) int {
	count := 0
	for _, tc := range tcs {
		if tc.Status == "skipped" {
			count++
		}
	}
	return count
}

func filterJunitArtifacts(artifacts []report.Artifact) []report.Artifact {
	var junits []report.Artifact
	for _, v := range artifacts {
		if v.AssetType == report.JUnitArtifact {
			junits = append(junits, v)
		}
	}
	return junits
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

		mainJunits := filterJunitArtifacts(v.Artifacts)
		junitFiles := v.ParentJUnits
		junitFiles = append(junitFiles, mainJunits...)

		jsuites, err := parseJunitFiles(junitFiles)
		if err != nil {
			log.Warn().Err(err).Str("suite", v.Name).Msg("Failed to parse some junit report. Summary may be incorrect!")
			continue
		}
		reduced := reduceJunitFiles(jsuites)

		for _, ts := range reduced.TestSuites {
			t.Tests += ts.Tests
			t.Failures += ts.Failures
			t.Errors += ts.Errors
			t.TestCases = append(t.TestCases, ts.TestCases...)
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

	f, err := os.Create(r.Filename)
	if err != nil {
		log.Err(err).Msg("Failed to render junit report.")
		return
	}
	defer f.Close()

	_, _ = f.Write(b)
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
