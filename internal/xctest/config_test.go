package xctest

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/stretchr/testify/assert"
	"gotest.tools/v3/fs"
)

func TestFromFile_NetworkThrottling(t *testing.T) {
	dir := fs.NewDir(t, "xctest-cfg",
		fs.WithFile("config.yml", `
apiVersion: v1alpha
kind: xctest
xctest:
  app: ./tests/apps/SauceLabs.ipa
  xcTestRunFile: ./tests/apps/SauceLabs.xctestrun
suites:
  - name: "network throttle test"
    networkProfile: "3G-slow"
    devices:
      - name: "iPhone XR"
        platformVersion: "14.3"
  - name: "custom conditions test"
    networkConditions:
      downloadSpeed: 5000
      uploadSpeed: 2000
      latency: 100
      loss: 5
    devices:
      - name: "iPhone XR"
        platformVersion: "14.3"
`, fs.WithMode(0655)))
	defer dir.Remove()

	cfg, err := FromFile(filepath.Join(dir.Path(), "config.yml"))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	assert.Equal(t, "3G-slow", cfg.Suites[0].NetworkProfile)
	assert.Nil(t, cfg.Suites[0].NetworkConditions)

	assert.Equal(t, "", cfg.Suites[1].NetworkProfile)
	assert.NotNil(t, cfg.Suites[1].NetworkConditions)

	dl := 5000
	ul := 2000
	lat := 100
	loss := 5
	expected := &config.NetworkConditions{
		DownloadSpeed: &dl,
		UploadSpeed:   &ul,
		Latency:       &lat,
		Loss:          &loss,
	}
	diff := cmp.Diff(expected, cfg.Suites[1].NetworkConditions)
	if diff != "" {
		t.Errorf("NetworkConditions mismatch: %s", diff)
	}
}
