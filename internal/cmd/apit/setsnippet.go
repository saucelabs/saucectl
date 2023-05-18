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
	cmd := &cobra.Command{
		Use:   "set-snippet NAME FILE_NAME|- [--project PROJECT_NAME]",
		Short: "Set a vault snippet",
		Long: `
Set/update a snippet in a project's vault. If a snippet NAME is already in the vault,
the value will be updated, otherwise a new snippet will be added. You can set a snippet's
value by providing a path to a file defining the snippet or use "-" to read from stdin.

Use [--project] to specify a project by its name or run without [--project] to choose from
a list of projects
`,
		SilenceUsage: true,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || (args[0] == "" || args[1] == "") {
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
			name := args[0]
			fileName := args[1]

			b, err := readSnippet(fileName)
			if err != nil {
				return err
			}

			updateVault := apitest.Vault{
				Variables: []apitest.VaultVariable{},
				Snippets: map[string]string{
					name: string(b),
				},
			}

			err = apitesterClient.PutVault(context.Background(), selectedProject.Hooks[0].Identifier, updateVault)
			if err != nil {
				return err
			}
			return nil
		},
	}

	return cmd
}

func readSnippet(fileName string) ([]byte, error) {
	var r io.Reader
	var err error
	if fileName == "-" {
		r = os.Stdin
	} else {
		r, err = os.Open(fileName)
		if err != nil {
			return []byte{}, err
		}
	}

	return io.ReadAll(r)
}
