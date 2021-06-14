package vmd

import "context"

// VirtualDevice describes a virtual device that can be used to run tests.
type VirtualDevice struct {
	Name string
	OS   string
}

const (
	IOSSimulator    = "ios-simulator"
	AndroidEmulator = "android-emulator"
)

type Reader interface {
	GetVirtualDevices(ctx context.Context, kind string) ([]VirtualDevice, error)
}
