package saucecloud

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path"
	"reflect"
	"regexp"
	"testing"
	"time"

	"gotest.tools/v3/fs"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/mocks"
	"github.com/saucelabs/saucectl/internal/xcuitest"
)

func TestXcuitestRunner_RunProject(t *testing.T) {
	httpmock.Activate()
	defer func() {
		httpmock.DeactivateAndReset()
	}()
	ccyReader := mocks.CCYReader{ReadAllowedCCYfn: func(ctx context.Context) (int, error) {
		return 1, nil
	}}
	uploader := &mocks.FakeProjectUploader{
		UploadSuccess: true,
	}

	var startOpts job.StartOptions
	runner := &XcuitestRunner{
		CloudRunner: CloudRunner{
			JobService: JobService{
				RDCStarter: &mocks.FakeJobStarter{
					StartJobFn: func(ctx context.Context, opts job.StartOptions) (jobID string, isRDC bool, err error) {
						startOpts = opts
						return "fake-job-id", false, nil
					},
				},
				RDCReader: &mocks.FakeJobReader{
					PollJobFn: func(ctx context.Context, id string, interval time.Duration, timeout time.Duration) (job.Job, error) {
						return job.Job{ID: id, Passed: true, Status: job.StatePassed}, nil
					},
					GetJobAssetFileNamesFn: func(ctx context.Context, jobID string) ([]string, error) {
						return []string{"file1", "file2"}, nil
					},
					GetJobAssetFileContentFn: func(ctx context.Context, jobID, fileName string) ([]byte, error) {
						return []byte("file content"), nil
					},
				},
				VDCWriter: &mocks.FakeJobWriter{
					UploadAssetFn: func(jobID string, fileName string, contentType string, content []byte) error {
						return nil
					},
				},
				VDCDownloader: &mocks.FakeArtifactDownloader{
					DownloadArtifactFn: func(jobID, suiteName string) []string { return []string{} },
				},
			},
			CCYReader:       ccyReader,
			ProjectUploader: uploader,
		},
		Project: xcuitest.Project{
			Xcuitest: xcuitest.Xcuitest{
				App:     "/path/to/app.ipa",
				TestApp: "/path/to/testApp.ipa",
			},
			Suites: []xcuitest.Suite{
				{
					Name: "my xcuitest project",
					Devices: []config.Device{
						{
							Name:            "iPhone 11",
							PlatformName:    "iOS",
							PlatformVersion: "14.3",
						},
					},
				},
			},
			Sauce: config.SauceConfig{
				Concurrency: 1,
			},
		},
	}
	cnt, err := runner.RunProject()
	assert.Nil(t, err)
	assert.Equal(t, 0, cnt)
	assert.Equal(t, "iPhone 11", startOpts.DeviceName)
	assert.Equal(t, "iOS", startOpts.PlatformName)
	assert.Equal(t, "14.3", startOpts.PlatformVersion)
}

func TestCalculateJobsCount(t *testing.T) {
	runner := &XcuitestRunner{
		Project: xcuitest.Project{
			Xcuitest: xcuitest.Xcuitest{
				App:     "/path/to/app.ipa",
				TestApp: "/path/to/testApp.ipa",
			},
			Suites: []xcuitest.Suite{
				{
					Name: "valid xcuitest project",
					Devices: []config.Device{
						{
							Name:            "iPhone 11",
							PlatformName:    "iOS",
							PlatformVersion: "14.3",
						},
						{
							Name:            "iPhone XR",
							PlatformName:    "iOS",
							PlatformVersion: "14.3",
						},
					},
				},
			},
		},
	}
	assert.Equal(t, runner.calculateJobsCount(runner.Project.Suites), 2)
}

func TestXcuitestRunner_ensureAppsAreIpa(t *testing.T) {
	dir := fs.NewDir(t, "my-app",
		fs.WithDir("my-app.app",
			fs.WithFile("check-me.txt", "check-me",
				fs.WithMode(0644))),
		fs.WithDir("my-test-app.app",
			fs.WithFile("test-check-me.txt", "test-check-me",
				fs.WithMode(0644))))
	defer dir.Remove()

	originalAppPath := path.Join(dir.Path(), "my-app.app")
	originalTestAppPath := path.Join(dir.Path(), "my-test-app.app")

	// Run it
	appPath, testAppPath, err := archiveAppsToIpaIfRequired(originalAppPath, originalTestAppPath)

	if err != nil {
		t.Errorf("got error: %v", err)
	}
	defer os.Remove(appPath)
	defer os.Remove(testAppPath)

	if !regexp.MustCompile(`my-app-([0-9]+)\.ipa$`).Match([]byte(appPath)) {
		t.Errorf("%v: should be an .ipa file", appPath)
	}
	if !regexp.MustCompile(`my-test-app-([0-9]+)\.ipa$`).Match([]byte(testAppPath)) {
		t.Errorf("%v: should be an .ipa file", testAppPath)
	}

	checkFileFound(t, appPath, "Payload/my-app.app/check-me.txt", "check-me")
	checkFileFound(t, testAppPath, "Payload/my-test-app.app/test-check-me.txt", "test-check-me")
}

func checkFileFound(t *testing.T, archiveName, fileName, fileContent string) {
	rd, _ := zip.OpenReader(archiveName)
	defer rd.Close()

	found := false
	for _, file := range rd.File {
		if file.Name == fileName {
			found = true
			frd, _ := file.Open()
			body, _ := io.ReadAll(frd)
			frd.Close()
			if !reflect.DeepEqual(body, []byte(fileContent)) {
				t.Errorf("want: %v, got: %v", fileContent, body)
			}
		}
	}
	if found == false {
		t.Errorf("%s was not found in archive", fileName)
	}
}
