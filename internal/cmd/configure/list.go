package configure

import (
	"fmt"

	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/spf13/cobra"
)

func ListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "list",
		Aliases: []string{
			"ls",
		},
		Short: "Shows the current credentials and their origin.",
		Run: func(cmd *cobra.Command, args []string) {
			creds := credentials.Get(region.None)
			if !creds.IsSet() {
				fmt.Println(`Credentials have not been set. Please use "saucectl configure" to set your credentials.`)
				return
			}
			printCreds(creds)
		},
	}

	return cmd
}
