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

type deviceWithStatus struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	OS     string `json:"os"`
	Status string `json:"status"`
}

type filter struct {
	Name   string
	Os     string
	Status string
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

			if statusFilter != "" {
				_, err := devicestatus.StrToStatus(statusFilter)
				if err != nil {
					return err
				}

				addStatus = true
			}

			options := listOptions{
				Status:       addStatus,
				OutputFormat: out,
				Filter: filter{
					Name:   nameFilter,
					Os:     osFilter,
					Status: statusFilter,
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

	var devsWithStatuses []deviceWithStatus
	if options.Status {
		res, err := getDevicesWithStatuses(ctx, devs)
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
		renderListTable(filtered, len(filtered), options.Status)
	}

	return nil
}

func filterDevices(devs []deviceWithStatus, filter filter) []deviceWithStatus {
	var filtered []deviceWithStatus
	for _, dev := range devs {
		if filter.Name != "" && !strings.Contains(strings.ToLower(dev.Name), strings.ToLower(filter.Name)) {
			continue
		}

		if filter.Os != "" && !strings.Contains(strings.ToLower(dev.OS), strings.ToLower(filter.Os)) {
			continue
		}

		if filter.Status != "" && !strings.Contains(strings.ToLower(dev.Status), strings.ToLower(filter.Status)) {
			continue
		}

		filtered = append(filtered, dev)
	}
	return filtered
}

func getDevicesWithStatuses(ctx context.Context, devs []devices.Device) ([]deviceWithStatus, error) {
	statuses, err := devicesStatusesReader.GetDevicesStatuses(ctx)
	if err != nil {
		return []deviceWithStatus{}, fmt.Errorf("failed to get devices statuses: %w", err)
	}

	var result []deviceWithStatus
	for _, dev := range devs {
		var searchedStatus devices.DeviceStatus
		for _, status := range statuses {
			if status.ID == dev.ID {
				searchedStatus = status
			}
		}

		result = append(result, deviceWithStatus{
			ID:     dev.ID,
			Name:   dev.Name,
			OS:     dev.OS,
			Status: devicestatus.StatusToStr(searchedStatus.Status),
		})
	}

	return result, nil
}

func getDevicesWithEmptyStatuses(devs []devices.Device) []deviceWithStatus {
	var result []deviceWithStatus
	for _, dev := range devs {
		result = append(result, deviceWithStatus{
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

func renderListTable(devices []deviceWithStatus, total int, status bool) {
	if len(devices) == 0 {
		println("No devices found")
		return
	}

	t := table.NewWriter()
	t.SetStyle(tables.DefaultTableStyle)
	t.SuppressEmptyColumns()

	if status {
		writeDevicesWithStatus(t, devices)
	} else {
		writeDevices(t, devices)
	}

	t.SuppressEmptyColumns()
	t.AppendFooter(table.Row{
		fmt.Sprintf("showing %d devices", total),
	})

	fmt.Println(t.Render())
}

func writeDevices(t table.Writer, devices []deviceWithStatus) {
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
}

func writeDevicesWithStatus(t table.Writer, devices []deviceWithStatus) {
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
}
