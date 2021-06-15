package vmd

import "context"

// VirtualDevice describes a virtual device that can be used to run tests.
type VirtualDevice struct {
	Name string
	OS   string
}

// Constants for virtual device kinds.
const (
	IOSSimulator    = "ios-simulator"
	AndroidEmulator = "android-emulator"
)

// Reader is the interface for getting available virtual devices.
type Reader interface {
	GetVirtualDevices(ctx context.Context, kind string) ([]VirtualDevice, error)
}
