package artifact

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/mocks"
	"github.com/stretchr/testify/assert"
)

func TestShouldDownload(t *testing.T) {
	type testCase struct {
		name   string
		config config.ArtifactDownload
		jobID  string
		passed bool
		want   bool
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
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			downloader := &Download{Config: tt.config}
			got := downloader.shouldDownload(tt.jobID, tt.passed)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDownloadArtifacts(t *testing.T) {
	var testCase = []struct {
		name          string
		cfg           config.ArtifactDownload
		jobReader     job.Reader
		expContent    string
		isFileExisted bool
	}{
		{
			name: "should download artifacts successfully",
			cfg: config.ArtifactDownload{
				Directory: "artifacts",
				Match:     []string{"console.log"},
				When:      config.WhenAlways,
			},
			jobReader: &mocks.FakeJobReader{
				GetJobAssetFileNamesFn: func(ctx context.Context, jobID string) ([]string, error) {
					return []string{"console.log", "dummy.file"}, nil
				},
				GetJobAssetFileContentFn: func(ctx context.Context, jobID, fileName string) ([]byte, error) {
					return []byte("file-content"), nil
				},
			},
			expContent:    "file-content",
			isFileExisted: true,
		},
		{
			name: "should not download artifacts when set to never",
			cfg: config.ArtifactDownload{
				Directory: "artifacts",
				Match:     []string{"console.log"},
				When:      config.WhenNever,
			},
			jobReader: &mocks.FakeJobReader{
				GetJobAssetFileNamesFn: func(ctx context.Context, jobID string) ([]string, error) {
					return []string{"console.log", "dummy.file"}, nil
				},
				GetJobAssetFileContentFn: func(ctx context.Context, jobID, fileName string) ([]byte, error) {
					return []byte("file-content"), nil
				},
			},
			expContent:    "file-content",
			isFileExisted: false,
		},
		{
			name: "should not download artifacts when failed to get file name ",
			cfg: config.ArtifactDownload{
				Directory: "artifacts",
				Match:     []string{"console.log"},
				When:      config.WhenNever,
			},
			jobReader: &mocks.FakeJobReader{
				GetJobAssetFileNamesFn: func(ctx context.Context, jobID string) ([]string, error) {
					return []string{"console.log", "dummy.file"}, errors.New("500")
				},
				GetJobAssetFileContentFn: func(ctx context.Context, jobID, fileName string) ([]byte, error) {
					return []byte("file-content"), nil
				},
			},
			expContent:    "file-content",
			isFileExisted: false,
		},
	}
	for _, tc := range testCase {
		t.Run(tc.name, func(t *testing.T) {
			artifact := &Download{JobReader: tc.jobReader, Config: tc.cfg}
			artifact.DownloadArtifacts("fake-id", true)
			content, err := os.ReadFile(filepath.Join(tc.cfg.Directory, "fake-id", tc.cfg.Match[0]))
			if err != nil {
				assert.False(t, tc.isFileExisted)
			} else {
				assert.Equal(t, tc.expContent, string(content))
			}
			t.Cleanup(func() {
				os.RemoveAll(tc.cfg.Directory)
			})
		})
	}
}
