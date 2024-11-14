package apit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/saucelabs/saucectl/internal/apitest"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func SetSnippetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-snippet NAME FILE_NAME|- [--project PROJECT_NAME]",
		Short: "Set a vault snippet",
		Long: `Set/update a snippet in a project's vault. If a snippet NAME is already in the vault,
the value will be updated, otherwise a new snippet will be added. You can set a snippet's
value by providing a path to a file defining the snippet or use "-" to read from stdin.

Use [--project] to specify a project by its name or run without [--project] to choose from a list of projects.
`,
		Example: `saucectl apit vault set-snippet snip1 ./snippet1.xml --project "smoke tests"  # from a file
cat snippet2.xml | saucectl apit vault set-snippet snip2 - --project "smoke tests"  # from stdin
`,
		SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 || args[0] == "" {
				return errors.New("no snippet name specified")
			}
			if len(args) == 1 || args[1] == "" {
				return errors.New("no filename specified")
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
					cmds.FullName(cmd),
					usage.Flags(cmd.Flags()),
				)
				_ = tracker.Close()
			}()
			return nil
		},
		RunE: func(_ *cobra.Command, args []string) error {
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
