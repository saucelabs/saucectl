package saucecloud

import (
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/config"
	httpServices "github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/region"
	"gotest.tools/v3/assert"
)

func TestJobService_DownloadArtifact(t *testing.T) {
	fileContent := "<xml>junit.xml</xml>"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		switch r.URL.Path {
		case "/v1/rdc/jobs/test-123":
			_, err = w.Write([]byte(`{"automation_backend":"espresso"}`))
		case "/v1/rdc/jobs/test-123/junit.xml":
			_, err = w.Write([]byte(fileContent))
		default:
			w.WriteHeader(http.StatusNotFound)
		}

		if err != nil {
			t.Errorf("failed to respond: %v", err)
		}
	}))
	defer ts.Close()

	tempDir, err := os.MkdirTemp("", "saucectl-download-artifact")
	if err != nil {
		t.Errorf("Failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	rdc := httpServices.NewRDCService(
		region.None,
		"dummy-user",
		"dummy-key",
		10*time.Second,
	)
	rdc.URL = ts.URL

	artifactCfg := config.ArtifactDownload{
		Directory: tempDir,
		Match:     []string{"junit.xml"},
		When:      config.WhenAlways,
	}
	downloader := JobService{
		RDC:                    rdc,
		ArtifactDownloadConfig: artifactCfg,
	}
	downloader.DownloadArtifacts(context.Background(), job.Job{
		ID:     "test-123",
		Name:   "suite name",
		IsRDC:  true,
		Status: job.StateComplete,
	}, true)

	fileName := filepath.Join(tempDir, "suite_name", "junit.xml")
	d, err := os.ReadFile(fileName)
	if err != nil {
		t.Errorf("file '%s' not found: %v", fileName, err)
	}

	if string(d) != fileContent {
		t.Errorf("file content mismatch: got '%v', expects: '%v'", d, fileContent)
	}
}

func TestJobService_RetryDownloadArtifact(t *testing.T) {
	tries := 0
	fileContent := "<xml>junit.xml</xml>"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		switch r.URL.Path {
		case "/v1/rdc/jobs/test-123":
			_, err = w.Write([]byte(`{"automation_backend":"espresso"}`))
		case "/v1/rdc/jobs/test-123/junit.xml":
			if tries < 3 {
				tries++
				_, err = w.Write([]byte(fileContent[:5]))

				hj, ok := w.(http.Hijacker)
				if !ok {
					log.Println("Hijacking not supported")
					return
				}

				conn, _, err := hj.Hijack()
				if err != nil {
					log.Printf("Hijacking failed: %v", err)
					return
				}

				// This simulates the "unexpected EOF" error
				conn.Close()
			} else {
				_, err = w.Write([]byte(fileContent))
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}

		if err != nil {
			t.Errorf("failed to respond: %v", err)
		}
	}))
	defer ts.Close()

	tempDir, err := os.MkdirTemp("", "saucectl-download-artifact")
	if err != nil {
		t.Errorf("Failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	rdc := httpServices.NewRDCService(
		region.None,
		"dummy-user",
		"dummy-key",
		10*time.Second,
	)
	rdc.URL = ts.URL

	retryCount := uint(3)
	retryInterval := 0.1

	artifactCfg := config.ArtifactDownload{
		Directory:     tempDir,
		Match:         []string{"junit.xml"},
		When:          config.WhenAlways,
		RetryCount:    &retryCount,
		RetryInterval: &retryInterval,
	}
	downloader := JobService{
		RDC:                    rdc,
		ArtifactDownloadConfig: artifactCfg,
	}
	downloader.DownloadArtifacts(context.Background(), job.Job{
		ID:     "test-123",
		Name:   "suite name",
		IsRDC:  true,
		Status: job.StateComplete,
	}, true)

	fileName := filepath.Join(tempDir, "suite_name", "junit.xml")
	d, err := os.ReadFile(fileName)
	if err != nil {
		t.Errorf("file '%s' not found: %v", fileName, err)
	}

	if string(d) != fileContent {
		t.Errorf("file content mismatch: got '%v', expects: '%v'", d, fileContent)
	}
}

func TestJobService_skipDownload(t *testing.T) {
	testcases := []struct {
		name          string
		config        config.ArtifactDownload
		jobData       job.Job
		isLastAttempt bool
		expResult     bool
	}{
		{
			name:          "Should not skip download",
			config:        config.ArtifactDownload{When: config.WhenAlways},
			jobData:       job.Job{ID: "fake-id", Status: job.StatePassed, Passed: true},
			isLastAttempt: true,
			expResult:     false,
		},
		{
			name:          "Should skip download when job ID is empty",
			config:        config.ArtifactDownload{When: config.WhenAlways},
			jobData:       job.Job{Status: job.StatePassed, Passed: true},
			isLastAttempt: true,
			expResult:     true,
		},
		{
			name:          "Should skip download when job is timeout",
			config:        config.ArtifactDownload{When: config.WhenAlways},
			jobData:       job.Job{ID: "fake-id", TimedOut: true},
			isLastAttempt: true,
			expResult:     true,
		},
		{
			name:          "Should skip download when job is not done",
			config:        config.ArtifactDownload{When: config.WhenAlways},
			jobData:       job.Job{ID: "fake-id", TimedOut: true, Status: job.StateInProgress},
			isLastAttempt: true,
			expResult:     true,
		},
		{
			name:          "Should skip download when artifact config is not set to download",
			config:        config.ArtifactDownload{When: config.WhenNever},
			jobData:       job.Job{ID: "fake-id", Status: job.StatePassed, Passed: true},
			isLastAttempt: true,
			expResult:     true,
		},
		{
			name:          "Should skip download when it's not last attempt and not set download all attempts",
			config:        config.ArtifactDownload{When: config.WhenAlways, AllAttempts: false},
			jobData:       job.Job{ID: "fake-id", Status: job.StatePassed, Passed: true},
			isLastAttempt: false,
			expResult:     true,
		},
		{
			name:          "Should download when it's the last attempt and not set download all attempts",
			config:        config.ArtifactDownload{When: config.WhenAlways, AllAttempts: false},
			jobData:       job.Job{ID: "fake-id", Status: job.StatePassed, Passed: true},
			isLastAttempt: true,
			expResult:     false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			downloader := &JobService{
				ArtifactDownloadConfig: tc.config,
			}
			got := downloader.skipDownload(tc.jobData, tc.isLastAttempt)
			assert.Equal(t, tc.expResult, got)
		})
	}

}
