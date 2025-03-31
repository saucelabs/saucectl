package devices

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/saucelabs/saucectl/internal/devices"
	"github.com/saucelabs/saucectl/internal/tables"
	"github.com/spf13/cobra"
	"os"
	"strings"
)

const (
	JSONOutput = "json"
	TextOutput = "text"
)

func ListCommand() *cobra.Command {
	var out string
	var page int
	var size int
	var nameFilter string
	var osFilter string

	cmd := &cobra.Command{
		Use: "list",
		Aliases: []string{
			"ls",
		},
		Short:        "Returns the list of devices",
		SilenceUsage: true,
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

			return list(cmd.Context(), out, page, size, nameFilter, osFilter)
		},
	}

	flags := cmd.PersistentFlags()
	flags.StringVarP(&out, "out", "o", "text", "Output format to the console. Options: text, json.")
	flags.IntVarP(&page, "page", "p", 0, "Page for pagination. Default is 0.")
	flags.IntVarP(&size, "size", "s", 20, "Per page for pagination. Default is 20.")
	flags.StringVarP(&nameFilter, "name", "n", "", "Filter devices by name.")
	flags.StringVar(&osFilter, "os", "", "Filter devices by OS.")

	return cmd
}

func list(ctx context.Context, format string, page int, size int, nameFilter string, osFilter string) error {
	devs, err := devicesReader.GetDevices(ctx)
	if err != nil {
		return fmt.Errorf("failed to get devs: %w", err)
	}

	var filtered = filterDevices(devs, nameFilter, osFilter)

	from := min(size*page, len(filtered))
	to := min(size*(page+1), len(filtered))
	paginated := filtered[from:to]

	switch format {
	case "json":
		if err := renderJSON(paginated); err != nil {
			return fmt.Errorf("failed to render output: %w", err)
		}
	case "text":
		renderListTable(paginated, from+1, to, len(filtered))
	}

	return nil
}

func filterDevices(devs []devices.Device, nameFilter string, osFilter string) []devices.Device {
	var filtered []devices.Device
	for _, dev := range devs {
		if nameFilter != "" && !strings.Contains(strings.ToLower(dev.Name), strings.ToLower(nameFilter)) {
			continue
		}

		if osFilter != "" && !strings.Contains(strings.ToLower(dev.OS), strings.ToLower(osFilter)) {
			continue
		}

		filtered = append(filtered, dev)
	}
	return filtered
}

func renderJSON(val any) error {
	return json.NewEncoder(os.Stdout).Encode(val)
}

func renderListTable(devices []devices.Device, from int, to int, total int) {
	if len(devices) == 0 {
		println("No devices found")
		return
	}

	t := table.NewWriter()
	t.SetStyle(tables.DefaultTableStyle)
	t.SuppressEmptyColumns()

	t.AppendHeader(table.Row{
		"Name", "OS",
	})

	for _, item := range devices {
		// the order of values must match the order of the header
		t.AppendRow(table.Row{
			item.Name,
			item.OS,
		})
	}
	t.SuppressEmptyColumns()
	t.AppendFooter(table.Row{
		fmt.Sprintf("showing %d-%d devices out of %d", from, to, total),
	})

	fmt.Println(t.Render())
}
