package sauce

import (
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/cli/progress"
	"github.com/saucelabs/saucectl/internal/archive/zip"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/storage"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Runner represents the Sauce Labs cloud implementation for cypress.
type Runner struct {
	Project         cypress.Project
	ProjectUploader storage.ProjectUploader
}

// RunProject runs the tests defined in cypress.Project.
func (r *Runner) RunProject() (int, error) {
	r.uploadProject()
	log.Error().Msg("Not yet implemented.") // TODO remove debug
	return 1, nil
}

func (r *Runner) uploadProject() error {
	// Archive the project files.
	tempDir, err := ioutil.TempDir(os.TempDir(), "saucectl-app-payload")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	zipName := filepath.Join(tempDir, "app.zip")
	z, err := zip.NewWriter(zipName)
	if err != nil {
		return err
	}
	defer z.Close()

	files := []string{
		r.Project.Cypress.ConfigFile,
		r.Project.Cypress.ProjectPath,
	}

	if r.Project.Cypress.EnvFile != "" {
		files = append(files, r.Project.Cypress.EnvFile)
	}

	// TODO consolidate sauce-runner logic with the docker part
	rcPath := filepath.Join(tempDir, "sauce-runner.json")
	rcFile, err := os.OpenFile(rcPath, os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer rcFile.Close()
	if err = json.NewEncoder(rcFile).Encode(r.Project); err != nil {
		return err
	}
	if err := rcFile.Close(); err != nil {
		return fmt.Errorf("failed to close file stream when writing sauce runner config: %v", err)
	}
	files = append(files, rcPath)

	for _, f := range files {
		if err := z.Add(f, ""); err != nil {
			return err
		}
	}
	if err := z.Close(); err != nil {
		return err
	}

	// Upload the project files.
	progress.Show("Uploading project")
	resp, err := r.ProjectUploader.Upload(zipName)
	progress.Stop()
	if err != nil {
		return err
	}
	log.Info().Str("fileID", resp.ID).Msg("Successfully uploaded project.")
	return nil
}
