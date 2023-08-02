package saucecloud

import (
	"archive/zip"
	"io"
	"os"
	"path"
	"reflect"
	"regexp"
	"testing"

	"gotest.tools/v3/fs"

	"github.com/stretchr/testify/assert"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/xcuitest"
)

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

	appPath, err := archive(originalAppPath, ipaArchive)
	if err != nil {
		t.Errorf("got error: %v", err)
	}
	defer os.Remove(appPath)

	testAppPath, err := archive(originalTestAppPath, ipaArchive)
	if err != nil {
		t.Errorf("got error: %v", err)
	}
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
