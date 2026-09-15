package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/saucelabs/saucectl/internal/aiauthoring"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/tables"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
)

const (
	JSONOutput = "json"
	TextOutput = "text"
)

func TestCasesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "testcases",
		Aliases: []string{
			"tc",
		},
		Short:        "Manage saved AI-authored test cases",
		SilenceUsage: true,
	}

	cmd.AddCommand(
		ListTestCasesCommand(),
		GetTestCaseCommand(),
		RenameTestCaseCommand(),
		DeleteTestCaseCommand(),
	)

	return cmd
}

func ListTestCasesCommand() *cobra.Command {
	var out string
	var search string
	var limit int
	var skip int

	cmd := &cobra.Command{
		Use: "list",
		Aliases: []string{
			"ls",
		},
		Short:        "Returns the list of saved test cases",
		SilenceUsage: true,
		PreRun:       collectUsage,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if out != JSONOutput && out != TextOutput {
				return errors.New("unknown output format")
			}

			list, err := testCaseService.ListTestCases(cmd.Context(), aiauthoring.ListOptions{
				Search: search,
				Limit:  limit,
				Skip:   skip,
			})
			if err != nil {
				return fmt.Errorf("failed to list test cases: %w", err)
			}

			if out == JSONOutput {
				return renderJSON(list)
			}
			renderTestCasesTable(list)

			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&out, "out", "o", "text", "Output format to the console. Options: text, json.")
	flags.StringVar(&search, "search", "", "Filter test cases by a search term.")
	flags.IntVar(&limit, "limit", 0, "Maximum number of test cases to return.")
	flags.IntVar(&skip, "skip", 0, "Number of test cases to skip, for paging.")

	return cmd
}

func GetTestCaseCommand() *cobra.Command {
	var out string

	cmd := &cobra.Command{
		Use:          "get <testCaseID>",
		Short:        "Returns the details of a saved test case",
		SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 || args[0] == "" {
				return errors.New("no test case ID specified")
			}
			return nil
		},
		PreRun: collectUsage,
		RunE: func(cmd *cobra.Command, args []string) error {
			if out != JSONOutput && out != TextOutput {
				return errors.New("unknown output format")
			}

			tc, err := testCaseService.GetTestCase(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to get test case: %w", err)
			}

			if out == JSONOutput {
				return renderJSON(tc)
			}
			renderTestCase(tc)

			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&out, "out", "o", "text", "Output format to the console. Options: text, json.")

	return cmd
}

func RenameTestCaseCommand() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:          "rename <testCaseID>",
		Short:        "Renames a saved test case",
		SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 || args[0] == "" {
				return errors.New("no test case ID specified")
			}
			return nil
		},
		PreRun: collectUsage,
		RunE: func(cmd *cobra.Command, args []string) error {
			tc, err := testCaseService.RenameTestCase(cmd.Context(), args[0], name)
			if err != nil {
				return fmt.Errorf("failed to rename test case: %w", err)
			}

			fmt.Printf("Renamed test case %s to %q\n", tc.ID, tc.Name)

			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&name, "name", "n", "", "The new name for the test case.")
	_ = cmd.MarkFlagRequired("name")

	return cmd
}

func DeleteTestCaseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "delete <testCaseID>",
		Short:        "Deletes a saved test case",
		SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 || args[0] == "" {
				return errors.New("no test case ID specified")
			}
			return nil
		},
		PreRun: collectUsage,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := testCaseService.DeleteTestCase(cmd.Context(), args[0]); err != nil {
				return fmt.Errorf("failed to delete test case: %w", err)
			}

			fmt.Printf("Deleted test case %s\n", args[0])

			return nil
		},
	}

	return cmd
}

func collectUsage(cmd *cobra.Command, _ []string) {
	tracker := usage.DefaultClient

	go func() {
		tracker.Collect(
			cmds.FullName(cmd),
			usage.Flags(cmd.Flags()),
		)
		_ = tracker.Close()
	}()
}

func renderJSON(val any) error {
	return json.NewEncoder(os.Stdout).Encode(val)
}

func renderTestCasesTable(list aiauthoring.List) {
	if len(list.Items) == 0 {
		println("No test cases found")
		return
	}

	t := table.NewWriter()
	t.SetStyle(tables.DefaultTableStyle)
	t.SuppressEmptyColumns()

	t.AppendHeader(table.Row{
		"ID", "Name", "Status", "Created", "Creator",
	})

	for _, item := range list.Items {
		// the order of values must match the order of the header
		t.AppendRow(table.Row{
			item.ID,
			item.Name,
			item.Status,
			item.Created(),
			item.CreatorUserName,
		})
	}

	t.AppendFooter(table.Row{
		fmt.Sprintf("showing %d of %d test cases", len(list.Items), list.Total),
	})

	fmt.Println(t.Render())
}

func renderTestCase(tc aiauthoring.TestCase) {
	fields := []struct {
		label string
		value string
	}{
		{"ID", tc.ID},
		{"Name", tc.Name},
		{"Status", tc.Status},
		{"Description", tc.Description},
		{"Framework", tc.Framework},
		{"Created", tc.Created()},
		{"Updated", tc.Updated()},
		{"Creator", tc.CreatorUserName},
		{"Team", tc.TeamID},
		{"Suite", suiteLabel(tc)},
		{"Test URL", testURL(tc)},
	}

	for _, f := range fields {
		if f.value == "" {
			continue
		}
		fmt.Printf("%-12s %s\n", f.label+":", f.value)
	}
}

func suiteLabel(tc aiauthoring.TestCase) string {
	if tc.TestSuiteName != "" {
		return tc.TestSuiteName
	}
	return tc.TestSuiteID
}

func testURL(tc aiauthoring.TestCase) string {
	if tc.RunSettings == nil {
		return ""
	}
	return tc.RunSettings.TestURL
}
