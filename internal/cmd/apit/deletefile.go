package apit

import (
	"context"
	"errors"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
)

func DeleteFileCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete-file FILENAME [--project PROJECT_NAME]",
		Short: "Delete a file in vault",
		Long: `Delete a file in a project's vault.

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

			tracker := segment.DefaultClient

			go func() {
				tracker.Collect(
					cmds.FullName(cmd),
					usage.Flags(cmd.Flags()),
				)
				_ = tracker.Close()
			}()
			return nil
		},
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]

			confirmed, err := confirmDelete(name)
			if err != nil {
				return err
			}
			if !confirmed {
				fmt.Printf("File %q has NOT been deleted.\n", name)
				return nil
			}

			err = apitesterClient.DeleteVaultFile(context.Background(), selectedProject.ID, []string{name})
			if err != nil {
				return err
			}
			fmt.Printf("File %q has been successfully deleted.\n", name)
			return nil
		},
	}
	return cmd
}

func confirmDelete(fileName string) (bool, error) {
	var selection bool
	prompt := &survey.Confirm{
		Message: fmt.Sprintf("Do you really want to delete %q ?", fileName),
	}
	err := survey.AskOne(prompt, &selection)
	return selection, err
}
