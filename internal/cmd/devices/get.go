package devices

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/devices"
	"github.com/saucelabs/saucectl/internal/tables"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	var out string

	cmd := &cobra.Command{
		Use:          "get <device-id>",
		Short:        "Get device by id",
		SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 || args[0] == "" {
				return errors.New("no device ID specified")
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
			return getDevice(cmd.Context(), args[0], out)
		},
	}

	flags := cmd.PersistentFlags()
	flags.StringVarP(&out, "out", "o", "text", "Output format to the console. Options: text, json.")

	return cmd
}

func getDevice(ctx context.Context, deviceID, outputFormat string) error {
	device, err := deviceReader.GetDevice(ctx, deviceID)
	if err != nil {
		return fmt.Errorf("failed to get device: %w", err)
	}

	switch outputFormat {
	case JSONOutput:
		if err := renderJSON(device); err != nil {
			return fmt.Errorf("failed to render output: %w", err)
		}
	case TextOutput:
		renderDeviceTable(device)
	}

	return nil
}

func renderDeviceTable(device devices.DeviceDetails) {
	t := table.NewWriter()
	t.SetStyle(tables.DefaultTableStyle)
	t.SuppressEmptyColumns()

	t.AppendHeader(table.Row{"Property", "Value"})
	t.AppendRow(table.Row{"ID", device.ID})
	t.AppendRow(table.Row{"Name", device.Name})
	t.AppendRow(table.Row{"OS", device.OS})
	t.AppendRow(table.Row{"OS Version", device.OSVersion})
	t.AppendRow(table.Row{"Manufacturer", strings.Join(device.Manufacturer, ", ")})
	t.AppendRow(table.Row{"Model Number", device.ModelNumber})
	t.AppendRow(table.Row{"Device Family", device.DeviceFamily})
	t.AppendRow(table.Row{"API Level", device.APILevel})
	t.AppendRow(table.Row{"CPU Type", device.CPUType})
	t.AppendRow(table.Row{"CPU Cores", device.CPUCores})
	t.AppendRow(table.Row{"CPU Frequency", fmt.Sprintf("%d MHz", device.CPUFrequency)})
	t.AppendRow(table.Row{"ABI Type", device.AbiType})
	t.AppendRow(table.Row{"Is ARM", device.IsArm})
	t.AppendRow(table.Row{"RAM Size", fmt.Sprintf("%d MB", device.RAMSize)})
	t.AppendRow(table.Row{"Internal Storage Size", fmt.Sprintf("%d MB", device.InternalStorageSize)})
	t.AppendRow(table.Row{"SD Card Size", fmt.Sprintf("%d MB", device.SdCardSize)})
	t.AppendRow(table.Row{"Screen Size", fmt.Sprintf("%.1f\"", device.ScreenSize)})
	t.AppendRow(table.Row{"Resolution", fmt.Sprintf("%dx%d", device.ResolutionWidth, device.ResolutionHeight)})
	t.AppendRow(table.Row{"DPI", device.Dpi})
	t.AppendRow(table.Row{"DPI Name", device.DpiName})
	t.AppendRow(table.Row{"Pixels Per Point", device.PixelsPerPoint})
	t.AppendRow(table.Row{"Default Orientation", device.DefaultOrientation})
	t.AppendRow(table.Row{"Internal Orientation", device.InternalOrientation})
	t.AppendRow(table.Row{"Has On-Screen Buttons", device.HasOnScreenButtons})
	t.AppendRow(table.Row{"Is Tablet", device.IsTablet})
	t.AppendRow(table.Row{"Is Private", device.IsPrivate})
	t.AppendRow(table.Row{"Is Rooted", device.IsRooted})
	t.AppendRow(table.Row{"Is Key Guard Disabled", device.IsKeyGuardDisabled})
	t.AppendRow(table.Row{"Is Alternative IO Enabled", device.IsAlternativeIoEnabled})
	t.AppendRow(table.Row{"Supports Multi-Touch", device.SupportsMultiTouch})
	t.AppendRow(table.Row{"Supports Mock Locations", device.SupportsMockLocations})
	t.AppendRow(table.Row{"Supports Global Proxy", device.SupportsGlobalProxy})
	t.AppendRow(table.Row{"Supports Appium Web App Testing", device.SupportsAppiumWebAppTesting})
	t.AppendRow(table.Row{"Supports Manual Web Testing", device.SupportsManualWebTesting})
	t.AppendRow(table.Row{"Supports Minicap Socket Connection", device.SupportsMinicapSocketConnection})
	t.AppendRow(table.Row{"Supports XCUITest", device.SupportsXcuiTest})

	t.SuppressEmptyColumns()
	fmt.Println(t.Render())
}
