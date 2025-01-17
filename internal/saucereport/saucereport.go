package saucereport

import (
	"encoding/json"
	"time"
)

// FileName is the name for Sauce Labs report.
const FileName = "sauce-test-report.json"

// The different states that a job can be in.
const (
	StatusPassed  = "passed"
	StatusSkipped = "skipped"
	StatusFailed  = "failed"
)

// SauceReport represents a report generated by Sauce Labs.
type SauceReport struct {
	Status      string       `json:"status,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
	Suites      []Suite      `json:"suites,omitempty"`
}

// Suite represents a suite in a report.
type Suite struct {
	Name        string       `json:"name,omitempty"`
	Status      string       `json:"status,omitempty"`
	Metadata    Metadata     `json:"metadata,omitempty"`
	Suites      []Suite      `json:"suites,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
	Tests       []Test       `json:"tests,omitempty"`
}

// Test represents a test in a report.
type Test struct {
	Name           string       `json:"name,omitempty"`
	Status         string       `json:"status,omitempty"`
	StartTime      time.Time    `json:"startTime,omitempty"`
	Duration       int          `json:"duration,omitempty"`
	Attachments    []Attachment `json:"attachments,omitempty"`
	Metadata       Metadata     `json:"metadata,omitempty"`
	Output         string       `json:"output,omitempty"`
	Code           Code         `json:"code,omitempty"`
	VideoTimestamp float64      `json:"VideoTimestamp,omitempty"`
}

// Code represents the code of a test.
type Code struct {
	Lines []string `json:"lines,omitempty"`
}

// Attachment represents an attachment.
type Attachment struct {
	Name        string `json:"name,omitempty"`
	Path        string `json:"path,omitempty"`
	ContentType string `json:"contentType,omitempty"`
}

// Metadata represents metadata.
type Metadata map[string]interface{}

// Parse parses an json-encoded byte string and returns a `SauceReport` struct
func Parse(fileContent []byte) (SauceReport, error) {
	var report SauceReport
	err := json.Unmarshal(fileContent, &report)
	if err != nil {
		return SauceReport{}, err
	}
	return report, nil
}

// GetFailedTests get names from failed tests.
func GetFailedTests(report SauceReport) []string {
	var failedTests []string
	if report.Status == StatusPassed || report.Status == StatusSkipped {
		return failedTests
	}
	for _, s := range report.Suites {
		failedTests = append(failedTests, collectFailedTests(s)...)
	}

	return failedTests
}

func collectFailedTests(suite Suite) []string {
	if len(suite.Suites) == 0 && len(suite.Tests) == 0 {
		return []string{}
	}
	if suite.Status == StatusPassed || suite.Status == StatusSkipped {
		return []string{}
	}

	var failedTests []string
	for _, s := range suite.Suites {
		if s.Status == StatusFailed {
			failedTests = append(failedTests, collectFailedTests(s)...)
		}
	}
	for _, t := range suite.Tests {
		if t.Status == StatusFailed {
			failedTests = append(failedTests, t.Name)
		}
	}

	return failedTests
}
