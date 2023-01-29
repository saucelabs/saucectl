package mocks

import (
	"context"
	"github.com/saucelabs/saucectl/internal/devices"
)

// FakeDevicesReader is a mock for the devices.Reader interface.
type FakeDevicesReader struct {
	GetDevicesFn func (context.Context, string) ([]devices.Device, error)
}

// GetDevices is a wrapper around GetDevicesFn.
func (fdr *FakeDevicesReader) GetDevices(ctx context.Context, OS string) ([]devices.Device, error) {
	return fdr.GetDevicesFn(ctx, OS)
}
