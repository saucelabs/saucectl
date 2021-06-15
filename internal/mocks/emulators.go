package mocks

import (
	"context"
	"github.com/saucelabs/saucectl/internal/vmd"
)

// FakeEmulatorsReader is a mock for the vmd.Reader interface.
type FakeEmulatorsReader struct {
	GetVirtualDevicesFn func(context.Context, string) ([]vmd.VirtualDevice, error)

}

// GetVirtualDevices is a wrapper around GetVirtualDevicesFn.
func (fer *FakeEmulatorsReader) GetVirtualDevices(ctx context.Context, kind string) ([]vmd.VirtualDevice, error) {
	return fer.GetVirtualDevicesFn(ctx, kind)
}
