package new

import (
	"github.com/spf13/cobra"
)

var (
	newUse         = "new"
	newShort       = "*DEPRECATED*"
	newLong        = "*DEPRECATED*"
	newExample     = "saucectl new"
	deprecationMsg = `please refer to our examples for how to setup saucectl for your project:

- https://github.com/saucelabs/saucectl-cypress-example
- https://github.com/saucelabs/saucectl-espresso-example
- https://github.com/saucelabs/saucectl-playwright-example
- https://github.com/saucelabs/saucectl-puppeteer-example
- https://github.com/saucelabs/saucectl-testcafe-example
- https://github.com/saucelabs/saucectl-xcuitest-example`
)

// Command creates the `new` command
func Command() *cobra.Command {
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
