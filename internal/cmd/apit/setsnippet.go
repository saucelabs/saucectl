package apit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/saucelabs/saucectl/internal/apitest"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/spf13/cobra"
)

func SetSnippetCommand() *cobra.Command {
	var file string
	var project string
	cmd := &cobra.Command{
		Use:          "set-snippet <snippetName>",
		Short:        "Set a vault snippet",
		SilenceUsage: true,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || (args[0] == "") {
				// TODO: Give useful error message
				return errors.New("no project name specified")
			}
			return nil
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			err := http.CheckProxy()
			if err != nil {
				return fmt.Errorf("invalid HTTP_PROXY value")
			}

			// tracker := segment.DefaultTracker

			// go func() {
			// 	tracker.Collect(
			// 		cases.Title(language.English).String(cmds.FullName(cmd)),
			// 		usage.Properties{}.SetFlags(cmd.Flags()),
			// 	)
			// 	_ = tracker.Close()
			// }()
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return setSnippet(project, args[0], file)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "The name of the project.")
	cmd.Flags().StringVar(&file, "file", "", "A file that defines a vault snippet. Use '-' to read from stdin.")
	cmd.MarkFlagRequired("file")
	cmd.MarkFlagRequired("project")

	return cmd
}

func setSnippet(projectName string, name string, fileName string) error {
	var r io.Reader
	var err error
	if fileName == "-" {
		r = os.Stdin
	} else {
		r, err = os.Open(fileName)
	}

	if err != nil {
		return err
	}

	b, err := io.ReadAll(r)

	project, err := resolve(projectName)
	if err != nil {
		return err
	}

	updateVault := apitest.Vault{
		Variables: []apitest.VaultVariable{},
		Snippets: map[string]string{
			name: string(b),
		},
	}

    err = apitesterClient.PutVault(context.Background(), project.Hooks[0].Identifier, updateVault)
    if err != nil {
    	return err
    }
	return nil
}
