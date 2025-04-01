package devices

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/devices"
	"github.com/saucelabs/saucectl/internal/devices/devicestatus"
	"github.com/saucelabs/saucectl/internal/tables"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
)

const (
	JSONOutput = "json"
	TextOutput = "text"
)

type filter struct {
	Name         string
	Os           string
	Status       devicestatus.Status
	FilterStatus bool
}

type listOptions struct {
	Status       bool
	OutputFormat string
	Filter       filter
}

func ListCommand() *cobra.Command {
	var out string
	var nameFilter string
	var osFilter string
	var addStatus bool
	var statusFilter string

	cmd := &cobra.Command{
		Use: "list",
		Aliases: []string{
			"ls",
		},
		Short:        "Returns the list of devices",
		SilenceUsage: true,
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
		RunE: func(cmd *cobra.Command, _ []string) error {
			if out != JSONOutput && out != TextOutput {
				return errors.New("unknown output format")
			}

			var status devicestatus.Status
			filterStatus := false
			if statusFilter != "" {
				res, err := devicestatus.Make(statusFilter)
				if err != nil {
					return err
				}

				addStatus = true
				filterStatus = true
				status = res
			}

			options := listOptions{
				Status:       addStatus,
				OutputFormat: out,
				Filter: filter{
					Name:         nameFilter,
					Os:           osFilter,
					Status:       status,
					FilterStatus: filterStatus,
				},
			}

			return list(cmd.Context(), options)
		},
	}

	flags := cmd.PersistentFlags()
	flags.StringVarP(&out, "out", "o", "text", "OutputFormat format to the console. Options: text, json.")
	flags.StringVarP(&nameFilter, "name", "n", "", "Filter devices by name.")
	flags.StringVar(&osFilter, "os", "", "Filter devices by OS.")
	flags.BoolVar(&addStatus, "statuses", false, "Fetch status for devices.")
	flags.StringVar(&statusFilter, "status", "", "Filter devices by status. Implies --statuses if not set.")

	return cmd
}

func list(ctx context.Context, options listOptions) error {
	devs, err := devicesReader.GetDevices(ctx)
	if err != nil {
		return fmt.Errorf("failed to get devices: %w", err)
	}

	var devsWithStatuses []devices.DeviceWithStatus
	if options.Status {
		res, err := devicesStatusesReader.GetDevicesWithStatuses(ctx)
		if err != nil {
			return fmt.Errorf("failed to get devices: %w", err)
		}
		devsWithStatuses = res
	} else {
		devsWithStatuses = getDevicesWithEmptyStatuses(devs)
	}

	var filtered = filterDevices(devsWithStatuses, options.Filter)

	switch options.OutputFormat {
	case "json":
		if err := renderJSON(filtered); err != nil {
			return fmt.Errorf("failed to render output: %w", err)
		}
	case "text":
		renderListTable(filtered, len(filtered))
	}

	return nil
}

func filterDevices(devs []devices.DeviceWithStatus, filter filter) []devices.DeviceWithStatus {
	var filtered []devices.DeviceWithStatus
	for _, dev := range devs {
		if filter.Name != "" && !strings.Contains(strings.ToLower(dev.Name), strings.ToLower(filter.Name)) {
			continue
		}

		if filter.Os != "" && !strings.Contains(strings.ToLower(dev.OS), strings.ToLower(filter.Os)) {
			continue
		}

		if filter.FilterStatus && dev.Status != filter.Status {
			continue
		}

		filtered = append(filtered, dev)
	}
	return filtered
}

func getDevicesWithEmptyStatuses(devs []devices.Device) []devices.DeviceWithStatus {
	var result []devices.DeviceWithStatus
	for _, dev := range devs {
		result = append(result, devices.DeviceWithStatus{
			ID:   dev.ID,
			Name: dev.Name,
			OS:   dev.OS,
		})
	}
	return result
}

func renderJSON(val any) error {
	return json.NewEncoder(os.Stdout).Encode(val)
}

func renderListTable(devices []devices.DeviceWithStatus, total int) {
	if len(devices) == 0 {
		println("No devices found")
		return
	}

	t := table.NewWriter()
	t.SetStyle(tables.DefaultTableStyle)
	t.SuppressEmptyColumns()

	t.AppendHeader(table.Row{
		"Name", "OS", "Status",
	})

	for _, item := range devices {
		// the order of values must match the order of the header
		t.AppendRow(table.Row{
			item.Name,
			item.OS,
			item.Status,
		})
	}

	t.SuppressEmptyColumns()
	t.AppendFooter(table.Row{
		fmt.Sprintf("showing %d devices", total),
	})

	fmt.Println(t.Render())
}
