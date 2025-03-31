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

type Filter struct {
	Name   string
	Os     string
	Status string
}

type ListOptions struct {
	Page         int
	Size         int
	Status       bool
	OutputFormat string
	Filter       Filter
}

func ListCommand() *cobra.Command {
	var out string
	var page int
	var size int
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

			if statusFilter != "" {
				addStatus = true
			}

			options := ListOptions{
				Page:         page,
				Size:         size,
				Status:       addStatus,
				OutputFormat: out,
				Filter: Filter{
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
	flags.IntVarP(&page, "page", "p", 0, "Page for pagination. Default is 0.")
	flags.IntVarP(&size, "size", "s", 0, "Per page for pagination. Default is all.")
	flags.StringVarP(&nameFilter, "name", "n", "", "Filter devices by name.")
	flags.StringVar(&osFilter, "os", "", "Filter devices by OS.")
	flags.BoolVar(&addStatus, "statuses", false, "Fetch status for devices.")
	flags.StringVar(&statusFilter, "status", "", "Filter devices by status. Implies --statuses if not set.")

	return cmd
}

func list(ctx context.Context, options ListOptions) error {
	devs, err := devicesReader.GetDevices(ctx)
	if err != nil {
		return fmt.Errorf("failed to get devices: %w", err)
	}

	var devsWithStatuses []DeviceWithStatus
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

	var paginated []DeviceWithStatus
	var from, to int
	if options.Size > 0 {
		from = min(options.Size*options.Page, len(filtered))
		to = min(options.Size*(options.Page+1), len(filtered))
		paginated = filtered[from:to]
	} else {
		from = 0
		to = len(filtered)
		paginated = filtered
	}

	switch options.OutputFormat {
	case "json":
		if err := renderJSON(paginated); err != nil {
			return fmt.Errorf("failed to render output: %w", err)
		}
	case "text":
		renderListTable(paginated, from+1, to, len(filtered), options.Status)
	}

	return nil
}

func filterDevices(devs []DeviceWithStatus, filter Filter) []DeviceWithStatus {
	var filtered []DeviceWithStatus
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
