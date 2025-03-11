package builds

import (
	"context"
	"errors"
	"fmt"
	"github.com/saucelabs/saucectl/internal/build"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	var out string
	var byJob bool
	var jobSource string

	cmd := &cobra.Command{
		Use:          "get",
		Short:        "Get build by build or job ID",
		SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 || args[0] == "" {
				return errors.New("no build specified")
			}
			return nil
		},
		PreRun: func(cmd *cobra.Command, _ []string) {
			tracker := usage.DefaultClient

			go func() {
				tracker.Collect(
					cmds.FullName(cmd),
					usage.Flags(cmd.Flags()),
				)
				_ = tracker.Close()
			}()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if out != JSONOutput && out != TextOutput {
				return errors.New("unknown output format")
			}

			src := build.Source(jobSource)
			if src != build.SourceRDC && src != build.SourceVDC {
				return errors.New("invalid job resource. Options: vdc, rdc")
			}

			return get(cmd.Context(), args[0], byJob, src, out)
		},
	}
	flags := cmd.PersistentFlags()
	flags.BoolVarP(&byJob, "job-id", "", false, "Find the build by providing a job ID instead of a build ID.")
	flags.StringVarP(&out, "out", "o", "text", "Output format to the console. Options: text, json.")
	flags.StringVar(&jobSource, "source", "", "Job source from saucelabs. Options: vdc, rdc.")

	return cmd
}

func get(ctx context.Context, ID string, byJob bool, Source build.Source, outputFormat string) error {
	b, err := buildsService.GetBuild(ctx, build.GetBuildOptions{
		ID:     ID,
		Source: Source,
		ByJob:  byJob,
	})
	if err != nil {
		return fmt.Errorf("failed to get build: %w", err)
	}

	switch outputFormat {
	case "json":
		if err := renderJSON(b); err != nil {
			return fmt.Errorf("failed to render output: %w", err)
		}
	case "text":
		renderListTable([]build.Build{b})
	}

	return nil
}
