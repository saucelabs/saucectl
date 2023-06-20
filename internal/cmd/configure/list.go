package configure

import (
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/spf13/cobra"
)

func ListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "list",
		Aliases: []string{
			"ls",
		},
		Short: "Showing the current credentials and resources",
		Run: func(cmd *cobra.Command, args []string) {
			creds := credentials.Get()
			printCreds(creds)
		},
	}

	return cmd
}
