package sauce

import (
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/cli/progress"
	"github.com/saucelabs/saucectl/internal/archive/zip"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/jsonio"
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
	// Archive the project files.
	tempDir, err := ioutil.TempDir(os.TempDir(), "saucectl-app-payload")
	if err != nil {
		return 1, err
	}
	defer os.RemoveAll(tempDir)

	zipName, err := r.archiveProject(tempDir)
	if err != nil {
		return 1, err
	}

	r.uploadProject(zipName)
	log.Error().Msg("Not yet implemented.") // TODO remove debug
	return 1, nil
}

func (r *Runner) archiveProject(tempDir string) (string, error) {
	zipName := filepath.Join(tempDir, "app.zip")
	z, err := zip.NewWriter(zipName)
	if err != nil {
		return "", err
	}
	defer z.Close()

	files := []string{
		r.Project.Cypress.ConfigFile,
		r.Project.Cypress.ProjectPath,
	}

	if r.Project.Cypress.EnvFile != "" {
		files = append(files, r.Project.Cypress.EnvFile)
	}

	rcPath := filepath.Join(tempDir, "sauce-runner.json")
	if err := jsonio.WriteFile(rcPath, r.Project); err != nil {
		return "", err
	}
	files = append(files, rcPath)

	for _, f := range files {
		if err := z.Add(f, ""); err != nil {
			return "", err
		}
	}

	return zipName, z.Close()
}

func (r *Runner) uploadProject(filename string) error {
	progress.Show("Uploading project")
	resp, err := r.ProjectUploader.Upload(filename)
	progress.Stop()
	if err != nil {
		return err
	}
	log.Info().Str("fileID", resp.ID).Msg("Project uploaded.")
	return nil
}
