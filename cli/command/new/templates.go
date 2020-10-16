package new

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"github.com/google/go-github/github"
	"github.com/rs/zerolog/log"
	"github.com/tj/survey"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

var (
	templateFileName = "saucetpl.tar.gz"
)

// GetReleaseArtifactURL provides template artifact url for a given repo
func GetReleaseArtifactURL(org string, repo string) (string, error) {
	ctx := context.Background()
	ghClient := github.NewClient(nil)
	release, _, err := ghClient.Repositories.GetLatestRelease(ctx, org, repo)
	if err != nil {
		return "", err
	}

	for _, asset := range release.Assets {
		if *asset.Name == templateFileName {
			return asset.GetBrowserDownloadURL(), nil
		}
	}
	return "", fmt.Errorf("no %s found", templateFileName)
}

func confirmOverwriting(name string, overWriteAll *bool) bool {
	if *overWriteAll {
		return true
	}

	var answer string
	question := &survey.Select{
		Message: fmt.Sprintf("Overwrite %s:", name),
		Options: []string{"No", "Yes", "All"},
		Default: "No",
	}
	err := survey.AskOne(question, &answer, nil)
	if err != nil {
		log.Err(err).Msg("unable to get survey answer")
		return false
	}

	*overWriteAll = answer == "All"
	return answer == "Yes" || answer == "All"
}

func requiresOverwriting(name string) (bool, error) {
	_, err := os.Stat(name)
	if err != nil && !os.IsNotExist(err) {
		log.Err(err).Msgf("unable to check for %s existence", name)
		return false, err
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, nil
}

func extractFile(name string, mode int64, src io.Reader, overWriteAll *bool) error {
	requireOverwrite, err := requiresOverwriting(name)
	if err != nil {
		return err
	}
	if requireOverwrite && confirmOverwriting(name, overWriteAll) == false {
		return nil
	}

	file, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(mode))
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, src)
	return err
}

// FetchAndExtractTemplate gathers latest version of the template for the repo and extracts it locally.
func FetchAndExtractTemplate(org string, repo string) error {
	url, err := GetReleaseArtifactURL(org, repo)
	if err != nil {
		return err
	}

	httpClient := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := httpClient.Get(url)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	zipReader, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return err
	}

	overWriteAll := false
	tarReader := tar.NewReader(zipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if header == nil {
			break
		}

		if header.Typeflag == tar.TypeDir {
			err := os.MkdirAll(header.Name, os.FileMode(header.Mode))
			if err != nil {
				log.Err(err).Msgf("Unable to create %s", header.Name)
			}
		}
		if header.Typeflag == tar.TypeReg {
			err = extractFile(header.Name, header.Mode, tarReader, &overWriteAll)
			if err != nil {
				log.Err(err).Msgf("Unable to extract %s", header.Name)
			}
		}
	}
	return nil
}
