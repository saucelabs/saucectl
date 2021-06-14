package mocks

import (
	"context"
	"github.com/saucelabs/saucectl/internal/devices"
)

type FakeDevicesReader struct {
	GetDevicesFn func (context.Context, string) ([]devices.Device, error)
}

func (fdr *FakeDevicesReader) GetDevices(ctx context.Context, OS string) ([]devices.Device, error) {
	return fdr.GetDevicesFn(ctx, OS)
}
