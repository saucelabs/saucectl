package github

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/report"
)

func TestReporter_RenderWithAttempts(t *testing.T) {
	f, err := os.CreateTemp("", "github-step-summary")
	if err != nil {
		t.Fatalf("Failed to create temp file: %s", err)
	}
	defer os.Remove(f.Name())
	f.Close()

	r := &Reporter{
		startTime:       time.Now(),
		stepSummaryFile: f.Name(),
	}

	r.Add(report.TestResult{
		Name:     "Login Tests",
		Duration: 90 * time.Second,
		Status:   job.StatePassed,
		Browser:  "Chrome 120",
		Platform: "Windows 11",
		URL:      "https://app.saucelabs.com/tests/job-3",
		Attempts: []report.Attempt{
			{ID: "job-1", Status: job.StateFailed},
			{ID: "job-2", Status: job.StateFailed},
			{ID: "job-3", Status: job.StatePassed},
		},
	})
	r.Add(report.TestResult{
		Name:     "Checkout Tests",
		Duration: 45 * time.Second,
		Status:   job.StatePassed,
		Browser:  "Firefox 121",
		Platform: "macOS 13",
		URL:      "https://app.saucelabs.com/tests/job-4",
		Attempts: []report.Attempt{
			{ID: "job-4", Status: job.StatePassed},
		},
	})

	r.Render()

	data, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatalf("Failed to read output file: %s", err)
	}

	content := string(data)

	// Verify header has Attempts column
	if !strings.Contains(content, "Attempts |") {
		t.Errorf("Expected 'Attempts' column in header, got:\n%s", content)
	}
	// Verify Login Tests row shows 3 attempts
	if !strings.Contains(content, "| 3 |") {
		t.Errorf("Expected '| 3 |' for Login Tests attempts, got:\n%s", content)
	}
	// Verify Checkout Tests row shows 1 attempt
	if !strings.Contains(content, "| 1 |") {
		t.Errorf("Expected '| 1 |' for Checkout Tests attempts, got:\n%s", content)
	}
}
