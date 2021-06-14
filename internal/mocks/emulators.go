package mocks

import (
	"context"
	"github.com/saucelabs/saucectl/internal/vmd"
)

type FakeEmulatorsReader struct {
	GetVirtualDevicesFn func(context.Context, string) ([]vmd.VirtualDevice, error)

}

func (fer *FakeEmulatorsReader) GetVirtualDevices(ctx context.Context, kind string) ([]vmd.VirtualDevice, error) {
	return fer.GetVirtualDevicesFn(ctx, kind)
}
