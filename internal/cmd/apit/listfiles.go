package apit

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/usage"
)

func ListFilesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-files [--project PROJECT_NAME]",
		Short: "List vault files",
		Long: `Get a list of files from a project's vault.

Use [--project] to specify the project by its name or run without [--project] to choose from a list of projects.
`,
		SilenceUsage: true,
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			err := http.CheckProxy()
			if err != nil {
				return fmt.Errorf("invalid HTTP_PROXY value")
			}

			tracker := usage.DefaultClient

			go func() {
				tracker.Collect(
					cmds.FullName(cmd),
					usage.Flags(cmd.Flags()),
				)
				_ = tracker.Close()
			}()
			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			files, err := apitesterClient.ListVaultFiles(context.Background(), selectedProject.ID)
			if err != nil {
				return err
			}

			for _, file := range files {
				fmt.Printf("%s\n", file.Name)
			}
			return nil
		},
	}
	return cmd
}
