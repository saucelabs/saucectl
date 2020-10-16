package new

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"github.com/google/go-github/github"
	"github.com/jarcoal/httpmock"
	"gotest.tools/assert"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func ensureDeleted(folderPath string) error {
	st, err := os.Stat(folderPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if st != nil {
		err = os.Remove(folderPath)
		if err != nil {
			return err
		}
	}
	return nil
}

func TestGetReleaseArtifactURL(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	validAssetName := templateFileName
	validAssetURL := "http://dummy-url/saucetpl.tar.gz"
	validRelease := &github.RepositoryRelease{
		Assets: []github.ReleaseAsset{
			{
				Name: &validAssetName,
				URL: &validAssetURL,
				BrowserDownloadURL: &validAssetURL,
			},
		},
	}

	invalidAssetName := "no-saucetpl.tar.gz"
	invalidAssetURL := "http://dummy-url/saucetpl.tar.gz"
	invalidRelease := &github.RepositoryRelease{
		Assets: []github.ReleaseAsset{
			{
				Name: &invalidAssetName,
				URL: &invalidAssetURL,
				BrowserDownloadURL: &invalidAssetURL,
			},
		},
	}

	httpmock.RegisterResponder("GET", "https://api.github.com/repos/fake-org/fake-repo/releases/latest",
		func(req *http.Request) (*http.Response, error) {
			resp, err := httpmock.NewJsonResponse(200, validRelease)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), nil
			}
			return resp, nil
		},
	)

	httpmock.RegisterResponder("GET", "https://api.github.com/repos/fake-org/fake-buggy-repo/releases/latest",
		func(req *http.Request) (*http.Response, error) {
			resp, err := httpmock.NewJsonResponse(200, invalidRelease)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), nil
			}
			return resp, nil
		},
	)

	url, err := GetReleaseArtifactURL("fake-org", "fake-repo")
	assert.NilError(t, err)
	assert.Equal(t, url, validAssetURL)

	_, err = GetReleaseArtifactURL("fake-org", "fake-buggy-repo")
	assert.Error(t, err, "no " + templateFileName + " found")
}

func TestExtractFile(t *testing.T) {
	fileName := "./test-content.yml"
	bodyContent := "default-content"
	overWriteAll := false

	err := extractFile(fileName, 0644, strings.NewReader(bodyContent), &overWriteAll)
	assert.NilError(t, err)

	st, err := os.Stat(fileName)
	assert.NilError(t, err)
	assert.Equal(t, st.Mode(), os.FileMode(0644))
	assert.Equal(t, st.IsDir(), false)

	os.Remove(fileName)
}

func createTemplateTar() *bytes.Buffer {
	buf := bytes.NewBuffer([]byte{})
	gzipStream := gzip.NewWriter(buf)
	tarStream := tar.NewWriter(gzipStream)

	header := &tar.Header{
		Name:    "./test-folder/",
		Size:    12, // "content-file"
		Mode:    int64(0644),
		ModTime: time.Now(),
		Typeflag: tar.TypeDir,
	}
	tarStream.WriteHeader(header)
	tarStream.Write([]byte("content-file"))

	header = &tar.Header{
		Name:    "./test-folder/test-config.yml",
		Size:    12, // "content-file"
		Mode:    int64(0644),
		ModTime: time.Now(),
		Typeflag: tar.TypeReg,
	}
	tarStream.WriteHeader(header)
	tarStream.Write([]byte("content-file"))

	tarStream.Close()
	gzipStream.Close()
	return buf
}

func TestFetchAndExtractTemplate(t *testing.T) {
	tarFile := createTemplateTar()

	// Add hooks
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	validAssetName := "saucetpl.tar.gz"
	validAssetURL := "http://dummy-url/saucetpl.tar.gz"
	validRelease := &github.RepositoryRelease{
		Assets: []github.ReleaseAsset{
			{
				Name: &validAssetName,
				URL: &validAssetURL,
				BrowserDownloadURL: &validAssetURL,
			},
		},
	}
	httpmock.RegisterResponder("GET", "https://api.github.com/repos/fake-org/fake-repo/releases/latest",
		func(req *http.Request) (*http.Response, error) {
			resp, err := httpmock.NewJsonResponse(200, validRelease)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), nil
			}
			return resp, nil
		},
	)

	httpmock.RegisterResponder("GET", validAssetURL,
		func(req *http.Request) (*http.Response, error) {
			return httpmock.NewBytesResponse(200, tarFile.Bytes()), nil
		},
	)

	err := FetchAndExtractTemplate("fake-org", "fake-repo")
	assert.NilError(t, err)
	os.Remove("./test-folder/test-config.yml")
	os.Remove("./test-folder")
}