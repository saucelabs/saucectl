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
)

var (
	overWriteAll     = false
	templateFileName = "template.tar.gz"
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
	return "", fmt.Errorf("No %s found", templateFileName)
}

func createFolder(name string, mode int64) error {
	err := os.MkdirAll(name, os.FileMode(mode))
	if err != nil {
		return err
	}
	return nil
}

func confirmOverwriting(name string) bool {
	if overWriteAll {
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
		log.Err(err)
		return false
	}

	overWriteAll = answer == "All"
	return answer == "Yes" || answer == "All"
}

func extractFile(name string, mode int64, src io.Reader) error {
	stat, err := os.Stat(name)
	if err != nil && !os.IsNotExist(err) {
		log.Err(err)
		return err
	}
	if err == nil && stat.IsDir() {
		return fmt.Errorf("%s exists and is a directory", name)
	}

	if err == nil {
		if confirmOverwriting(name) == false {
			return nil
		}
	}

	file, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(mode))
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, src)
	if err != nil {
		return err
	}
	return nil
}

// FetchAndExtractTemplate gather latest version of template for the repo and extract it locally
func FetchAndExtractTemplate(org string, repo string) error {
	url, err := GetReleaseArtifactURL(org, repo)
	if err != nil {
		return err
	}

	resp, err := http.Get(url)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	zipReader, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return err
	}

	tarReader := tar.NewReader(zipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if header == nil {
			continue
		}

		if header.Typeflag == tar.TypeDir {
			err = createFolder(header.Name, header.Mode)
			if err != nil {
				log.Err(err).Msg(fmt.Sprintf("Unable to create %s", header.Name))
			}
		}
		if header.Typeflag == tar.TypeReg {
			err = extractFile(header.Name, header.Mode, tarReader)
			if err != nil {
				log.Err(err).Msg(fmt.Sprintf("Unable to extract %s", header.Name))
			}
		}
	}
	return nil
}
