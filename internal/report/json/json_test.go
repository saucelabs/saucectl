package json

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/report"
)

func TestReporter_RenderWithAttempts(t *testing.T) {
	f, err := os.CreateTemp("", "saucectl-report.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %s", err)
	}
	defer os.Remove(f.Name())
	f.Close()

	r := &Reporter{
		Filename: f.Name(),
		Results: []report.TestResult{
			{
				Name:     "Login Tests",
				Duration: 90 * time.Second,
				Status:   job.StatePassed,
				Browser:  "Chrome 120",
				Platform: "Windows 11",
				Attempts: []report.Attempt{
					{ID: "job-1", Status: job.StateFailed, Duration: 30 * time.Second},
					{ID: "job-2", Status: job.StateFailed, Duration: 28 * time.Second},
					{ID: "job-3", Status: job.StatePassed, Duration: 25 * time.Second},
				},
			},
		},
	}
	r.Render()

	data, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatalf("Failed to read output file: %s", err)
	}

	// Parse back to verify structure
	var results []report.TestResult
	if err := json.Unmarshal(data, &results); err != nil {
		t.Fatalf("Failed to parse JSON output: %s", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if len(results[0].Attempts) != 3 {
		t.Fatalf("Expected 3 attempts, got %d", len(results[0].Attempts))
	}
	if results[0].Attempts[0].Status != job.StateFailed {
		t.Errorf("Expected attempt 0 status %q, got %q", job.StateFailed, results[0].Attempts[0].Status)
	}
	if results[0].Attempts[0].ID != "job-1" {
		t.Errorf("Expected attempt 0 ID %q, got %q", "job-1", results[0].Attempts[0].ID)
	}
	if results[0].Attempts[2].Status != job.StatePassed {
		t.Errorf("Expected attempt 2 status %q, got %q", job.StatePassed, results[0].Attempts[2].Status)
	}
}

func TestReporter_RenderNoAttempts(t *testing.T) {
	f, err := os.CreateTemp("", "saucectl-report.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %s", err)
	}
	defer os.Remove(f.Name())
	f.Close()

	r := &Reporter{
		Filename: f.Name(),
		Results: []report.TestResult{
			{
				Name:     "Simple Test",
				Duration: 10 * time.Second,
				Status:   job.StatePassed,
			},
		},
	}
	r.Render()

	data, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatalf("Failed to read output file: %s", err)
	}

	// With omitempty, the "attempts" key should not appear when slice is empty
	raw := string(data)
	if containsString(raw, `"attempts"`) {
		t.Errorf("Expected no 'attempts' key in JSON when Attempts is empty, got:\n%s", raw)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
