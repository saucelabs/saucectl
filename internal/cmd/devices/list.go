package devices

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/saucelabs/saucectl/internal/devices"
	"github.com/saucelabs/saucectl/internal/devices/devicestatus"
	"github.com/saucelabs/saucectl/internal/tables"
	"github.com/spf13/cobra"
	"os"
	"strings"
)

const (
	JSONOutput = "json"
	TextOutput = "text"
)

type DeviceWithStatus struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	OS     string `json:"os"`
	Status string `json:"status"`
}

func ListCommand() *cobra.Command {
	var out string
	var page int
	var size int
	var nameFilter string
	var osFilter string
	var addStatus bool

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

			return list(cmd.Context(), out, page, size, nameFilter, osFilter, addStatus)
		},
	}

	flags := cmd.PersistentFlags()
	flags.StringVarP(&out, "out", "o", "text", "Output format to the console. Options: text, json.")
	flags.IntVarP(&page, "page", "p", 0, "Page for pagination. Default is 0.")
	flags.IntVarP(&size, "size", "s", 20, "Per page for pagination. Default is 20.")
	flags.StringVarP(&nameFilter, "name", "n", "", "Filter devices by name.")
	flags.StringVar(&osFilter, "os", "", "Filter devices by OS.")
	flags.BoolVar(&addStatus, "statuses", false, "Fetch status for devices.")

	return cmd
}

func list(ctx context.Context, format string, page int, size int, nameFilter string, osFilter string, addStatus bool) error {
	devs, err := devicesReader.GetDevices(ctx)
	if err != nil {
		return fmt.Errorf("failed to get devices: %w", err)
	}

	var filtered = filterDevices(devs, nameFilter, osFilter)

	from := min(size*page, len(filtered))
	to := min(size*(page+1), len(filtered))
	paginated := filtered[from:to]

	switch format {
	case "json":
		var toRender any
		if addStatus {
			res, err := getDevicesWithStatuses(ctx, paginated)
			if err != nil {
				return err
			}
			toRender = res
		} else {
			toRender = paginated
		}

		if err := renderJSON(toRender); err != nil {
			return fmt.Errorf("failed to render output: %w", err)
		}
	case "text":
		var toDisplay []DeviceWithStatus
		if addStatus {
			devsWithStatuses, err := getDevicesWithStatuses(ctx, paginated)
			if err != nil {
				return err
			}
			toDisplay = devsWithStatuses
		} else {
			toDisplay = getDevicesWithEmptyStatuses(devs)
		}
		renderListTable(toDisplay, from+1, to, len(filtered), addStatus)
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

func getDevicesWithStatuses(ctx context.Context, devs []devices.Device) ([]DeviceWithStatus, error) {
	statuses, err := devicesStatusesReader.GetDevicesStatuses(ctx)
	if err != nil {
		return []DeviceWithStatus{}, fmt.Errorf("failed to get devices statuses: %w", err)
	}

	var result []DeviceWithStatus
	for _, dev := range devs {
		var searchedStatus devices.DeviceStatus
		for _, status := range statuses {
			if status.ID == dev.ID {
				searchedStatus = status
			}
		}

		result = append(result, DeviceWithStatus{
			ID:     dev.ID,
			Name:   dev.Name,
			OS:     dev.OS,
			Status: devicestatus.StatusToStr(searchedStatus.Status),
		})
	}

	return result, nil
}

func getDevicesWithEmptyStatuses(devs []devices.Device) []DeviceWithStatus {
	var result []DeviceWithStatus
	for _, dev := range devs {
		result = append(result, DeviceWithStatus{
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

func renderListTable(devices []DeviceWithStatus, from int, to int, total int, status bool) {
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
		fmt.Sprintf("showing %d-%d devices out of %d", from, to, total),
	})

	fmt.Println(t.Render())
}

func writeDevices(t table.Writer, devices []DeviceWithStatus) {
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

func writeDevicesWithStatus(t table.Writer, devices []DeviceWithStatus) {
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
