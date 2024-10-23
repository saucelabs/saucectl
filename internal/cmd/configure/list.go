package configure

import (
	"fmt"

	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/spf13/cobra"
)

func ListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "list",
		Aliases: []string{
			"ls",
		},
		Short: "Shows the current credentials and their origin.",
		Run: func(_ *cobra.Command, _ []string) {
			creds := credentials.Get()
			if creds.Username == "" || creds.AccessKey == "" {
				fmt.Println(`Credentials have not been set. Please use "saucectl configure" to set your credentials.`)
				return
			}
			printCreds(creds)
		},
	}

	return cmd
}
