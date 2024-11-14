package apit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/saucelabs/saucectl/internal/apitest"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
)

func DownloadFileCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download-file FILENAME [--project PROJECT_NAME]",
		Short: "Download a vault file",
		Long: `Download a file from a project's vault.

Use [--project] to specify the project by its name or run without [--project] to choose from a list of projects.
`,
		SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 || args[0] == "" {
				return errors.New("no file name specified")
			}
			return nil
		},
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			err := http.CheckProxy()
			if err != nil {
				return fmt.Errorf("invalid HTTP_PROXY value")
			}

			tracker := segment.DefaultTracker

			go func() {
				tracker.Collect(
					cases.Title(language.English).String(cmds.FullName(cmd)),
					usage.Flags(cmd.Flags()),
				)
				_ = tracker.Close()
			}()
			return nil
		},
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]
			files, err := apitesterClient.ListVaultFiles(context.Background(), selectedProject.ID)
			if err != nil {
				return err
			}

			file, err := findMatchingFile(name, files)
			if err != nil {
				return fmt.Errorf("project %q has no vault drive file with name %q", selectedProject.ProjectMeta.Name, name)
			}

			bodyReader, err := apitesterClient.GetVaultFileContent(context.Background(), selectedProject.ID, file.ID)
			if err != nil {
				return err
			}

			err = saveFileToDisk(bodyReader, name)
			if err != nil {
				return err
			}
			fmt.Printf("File %q has been successfully retrieved.\n", name)
			return nil
		},
	}
	return cmd
}

func saveFileToDisk(rc io.ReadCloser, fileName string) error {
	defer rc.Close()

	fd, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer fd.Close()

	_, err = io.Copy(fd, rc)
	return err
}

func findMatchingFile(fileName string, files []apitest.VaultFile) (apitest.VaultFile, error) {
	for _, file := range files {
		if file.Name == fileName {
			return file, nil
		}
	}
	return apitest.VaultFile{}, fmt.Errorf("no file named %s found", fileName)
}
