package hto

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func LogsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs <runID>",
		Short: "Fetch the logs for an HTO run",
		RunE: func(cmd *cobra.Command, args []string) error {
			return exec(args[0])
		},
	}

	return cmd
}

func exec(runID string) error {
	log, err := imagerunnerClient.GetLogs(context.Background(), runID)
	if err != nil {
		return err
	}
	fmt.Println(log)
	return nil
}
