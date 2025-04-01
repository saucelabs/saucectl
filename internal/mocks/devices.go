package mocks

import (
	"context"

	"github.com/saucelabs/saucectl/internal/devices"
)

// FakeDevicesReader is a mock for the devices.ByOSReader interface.
type FakeDevicesReader struct {
	GetDevicesFn func(context.Context, string) ([]devices.Device, error)
}

// GetDevicesByOS is a wrapper around GetDevicesFn.
func (fdr *FakeDevicesReader) GetDevicesByOS(ctx context.Context, OS string) ([]devices.Device, error) {
	return fdr.GetDevicesFn(ctx, OS)
}
