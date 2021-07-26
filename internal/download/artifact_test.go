package download

import (
	"testing"

	"github.com/saucelabs/saucectl/internal/config"
	"gotest.tools/assert"
)

func TestShouldDownloadArtifact(t *testing.T) {
	type testCase struct {
		name     string
		config   config.ArtifactDownload
		jobID    string
		passed   bool
		timedOut bool
		want     bool
	}
	testCases := []testCase{
		{
			name:   "should not download when jobID is empty even being required",
			config: config.ArtifactDownload{When: config.WhenAlways},
			jobID:  "",
			want:   false,
		},
		{
			name:   "should not download when jobID is empty and not being required",
			config: config.ArtifactDownload{When: config.WhenNever},
			jobID:  "",
			want:   false,
		},
		{
			name:   "should download artifacts when it's always required",
			config: config.ArtifactDownload{When: config.WhenAlways},
			jobID:  "fake-id",
			want:   true,
		},
		{
			name:   "should download artifacts when it's always required even it's failed",
			config: config.ArtifactDownload{When: config.WhenAlways},
			jobID:  "fake-id",
			passed: false,
			want:   true,
		},
		{
			name:   "should not download artifacts when it's not required",
			config: config.ArtifactDownload{When: config.WhenNever},
			jobID:  "fake-id",
			passed: true,
			want:   false,
		},
		{
			name:   "should not download artifacts when it's not required and failed",
			config: config.ArtifactDownload{When: config.WhenNever},
			jobID:  "fake-id",
			passed: false,
			want:   false,
		},
		{
			name:   "should download artifacts when it only requires passed one and test is passed",
			config: config.ArtifactDownload{When: config.WhenPass},
			jobID:  "fake-id",
			passed: true,
			want:   true,
		},
		{
			name:   "should download artifacts when it requires passed one but test is failed",
			config: config.ArtifactDownload{When: config.WhenPass},
			jobID:  "fake-id",
			passed: false,
			want:   false,
		},
		{
			name:   "should download artifacts when it requirs failed one but test is passed",
			config: config.ArtifactDownload{When: config.WhenFail},
			jobID:  "fake-id",
			passed: true,
			want:   false,
		},
		{
			name:   "should download artifacts when it requires failed one and test is failed",
			config: config.ArtifactDownload{When: config.WhenFail},
			jobID:  "fake-id",
			passed: false,
			want:   true,
		},
		{
			name:     "should not download artifacts when it has timedOut",
			config:   config.ArtifactDownload{When: config.WhenFail},
			jobID:    "fake-id",
			passed:   false,
			timedOut: true,
			want:     false,
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldDownloadArtifact(tt.jobID, tt.passed, tt.timedOut, tt.config)
			assert.Equal(t, tt.want, got)
		})
	}
}
