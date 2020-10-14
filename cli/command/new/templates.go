package new

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
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
	overWriteAll = false
	templateFileName = "template.tar.gz"
)

func GetReleaseArtifactURL(org string, repo string) (string, error) {
	ctx := context.Background()
	ghClient := github.NewClient(nil)
	release, _, err := ghClient.Repositories.GetLatestRelease(ctx, org, repo)
	if err != nil {
		return "", err
	}

	for _, asset := range release.Assets {
		if *asset.Name ==  templateFileName {
			return asset.GetBrowserDownloadURL(), nil
		}
	}
	return "", errors.New(fmt.Sprintf("No %s found", templateFileName))
}

func CreateFolder(name string, mode int64) error {
	os.MkdirAll(name, os.FileMode(mode))
	return nil
}

func ConfirmOverwriting(name string) bool {
	if overWriteAll {
		return true
	}

	answers := struct {
		Overwrite string
	}{}
	question := []*survey.Question{
		{
			Name: "overwrite",
			Prompt: &survey.Select{
				Message: fmt.Sprintf("Overwrite %s:", name),
				Options: []string{"All", "Yes", "No"},
				Default: "No",
			},
		},
	}
	err := survey.Ask(question, &answers)
	if err != nil {
		log.Err(err)
		return false
	}

	overWriteAll = answers.Overwrite == "all"
	return answers.Overwrite == "yes" || answers.Overwrite == "all"
}

func ExtractFile(name string, mode int64, src io.Reader) error {
	stat, err := os.Stat(name)
	if err != nil && !os.IsNotExist(err) {
		log.Err(err)
		return err
	}
	if err == nil && stat.IsDir() {
		return errors.New(fmt.Sprintf("%s exists and is a directory", name))
	}

	if err == nil {
		if ConfirmOverwriting(name) == false {
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
			err = CreateFolder(header.Name, header.Mode)
			if err != nil {
				log.Err(err).Msg(fmt.Sprintf("Unable to create %s", header.Name))
			}
		}
		if header.Typeflag == tar.TypeReg {
			err = ExtractFile(header.Name, header.Mode, tarReader)
			if err != nil {
				log.Err(err).Msg(fmt.Sprintf("Unable to extract %s", header.Name))
			}
		}
	}
	return nil
}