package new

import (
	"github.com/saucelabs/saucectl/cli/command"
	"github.com/spf13/cobra"
)

var (
	newUse         = "new"
	newShort       = "*DEPRECATED*"
	newLong        = "*DEPRECATED*"
	newExample     = "saucectl new"
	deprecationMsg = `please refer to our examples for how setup saucectl for your project:

- https://github.com/saucelabs/saucectl-cypress-example
- https://github.com/saucelabs/saucectl-espresso-example
- https://github.com/saucelabs/saucectl-playwright-example
- https://github.com/saucelabs/saucectl-puppeteer-example
- https://github.com/saucelabs/saucectl-testcafe-example`
)

// Command creates the `new` command
func Command(cli *command.SauceCtlCli) *cobra.Command {
	cmd := &cobra.Command{
		Deprecated: deprecationMsg,
		Use:        newUse,
		Short:      newShort,
		Long:       newLong,
		Example:    newExample,
		Run: func(cmd *cobra.Command, args []string) {
		},
	}

	return cmd
}
