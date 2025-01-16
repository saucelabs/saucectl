package saucecloud

import (
	"archive/zip"
	"io"
	"os"
	"path"
	"reflect"
	"testing"

	"gotest.tools/fs"
)

func ensureAppsAreIpa(t *testing.T) {
	dir := fs.NewDir(t, "my-app",
		fs.WithDir("my-app.app",
			fs.WithFile("check-me.txt", "check-me",
				fs.WithMode(0644))),
		fs.WithDir("my-test-app.app",
			fs.WithFile("test-check-me.txt", "test-check-me",
				fs.WithMode(0644))))
	defer dir.Remove()

	tempDir := fs.NewDir(t, "tmp")
	defer tempDir.Remove()

	originalAppPath := path.Join(dir.Path(), "my-app.app")
	originalTestAppPath := path.Join(dir.Path(), "my-test-app.app")

	appPath, err := archive(originalAppPath, tempDir.Path(), ipaArchive)
	if err != nil {
		t.Errorf("got error: %v", err)
	}
	defer os.Remove(appPath)

	testAppPath, err := archive(originalTestAppPath, tempDir.Path(), ipaArchive)
	if err != nil {
		t.Errorf("got error: %v", err)
	}
	defer os.Remove(testAppPath)

	if path.Ext(appPath) != ".ipa" {
		t.Errorf("%v: should be an .ipa file", appPath)
	}
	if path.Ext(testAppPath) != ".ipa" {
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
