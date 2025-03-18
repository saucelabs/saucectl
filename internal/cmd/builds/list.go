package builds

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/saucelabs/saucectl/internal/build"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
)

const (
	JSONOutput = "json"
	TextOutput = "text"
)

func ListCommand() *cobra.Command {
	var out string
	var page int
	var size int
	var status string
	var nameFilter string

	cmd := &cobra.Command{
		Use: "list <vdc|rdc>",
		Aliases: []string{
			"ls",
		},
		Short:        "Returns the list of builds",
		SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("missing or invalid arguments: <vdc|rdc>")
			}

			src := build.Source(args[0])
			if src != build.SourceRDC && src != build.SourceVDC {
				return errors.New("invalid build resource. Options: vdc, rdc")
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
			if page < 0 {
				return errors.New("invalid page")
			}
			if size < 0 {
				return errors.New("invalid size")
			}
			if out != JSONOutput && out != TextOutput {
				return errors.New("unknown output format")
			}
			var isStatusValid bool

			stat := build.Status(status)
			for _, s := range build.AllStatuses {
				if s == stat {
					isStatusValid = true
					break
				}
			}

			if status != "" && !isStatusValid {
				strs := make([]string, len(build.AllStatuses))
				for i, n := range build.AllStatuses {
					strs[i] = string(n)
				}
				return fmt.Errorf("unknown status. Options: %s", strings.Join(strs, ", "))
			}

			src := build.Source(args[0])

			return list(cmd.Context(), out, page, size, stat, src, nameFilter)
		},
	}
	flags := cmd.PersistentFlags()
	flags.StringVarP(&out, "out", "o", "text", "Output format to the console. Options: text, json.")
	flags.IntVarP(&page, "page", "p", 0, "Page for pagination. Default is 0.")
	flags.IntVarP(&size, "size", "s", 20, "Per page for pagination. Default is 20.")
	flags.StringVarP(&nameFilter, "name", "n", "", "Filter builds by name. Must match full build name.")
	flags.StringVar(&status, "status", "", "Filter builds using status. Options: running, error, failed, complete, success.")

	return cmd
}

func list(ctx context.Context, format string, page int, size int, status build.Status, source build.Source, name string) error {
	user, err := userService.User(ctx)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	opts := build.ListBuildsOptions{
		UserID: user.ID,
		Page:   page,
		Size:   size,
		Status: status,
		Source: source,
		Name:   name,
	}

	builds, err := buildsService.ListBuilds(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to get builds: %w", err)
	}

	switch format {
	case "json":
		if err := renderJSON(builds); err != nil {
			return fmt.Errorf("failed to render output: %w", err)
		}
	case "text":
		renderListTable(builds)
	}

	return nil
}

func renderJSON(val any) error {
	return json.NewEncoder(os.Stdout).Encode(val)
}
