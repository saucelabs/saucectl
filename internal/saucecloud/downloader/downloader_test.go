package downloader

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/config"
	httpServices "github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/job"
)

func TestArtifactDownloader_DownloadArtifact(t *testing.T) {
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

	rc := httpServices.NewRDCService(ts.URL, "dummy-user", "dummy-key", 10*time.Second)
	artifactCfg := config.ArtifactDownload{
		Directory: tempDir,
		Match:     []string{"junit.xml"},
		When:      config.WhenAlways,
	}
	downloader := NewArtifactDownloader(&rc, artifactCfg)
	downloader.DownloadArtifact(
		job.Job{
			ID:     "test-123",
			Name:   "suite name",
			IsRDC:  true,
			Status: job.StateComplete,
		}, false,
	)

	fileName := filepath.Join(tempDir, "suite_name", "junit.xml")
	d, err := os.ReadFile(fileName)
	if err != nil {
		t.Errorf("file '%s' not found: %v", fileName, err)
	}

	if string(d) != fileContent {
		t.Errorf("file content mismatch: got '%v', expects: '%v'", d, fileContent)
	}
}
