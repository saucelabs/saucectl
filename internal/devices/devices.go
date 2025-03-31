package devices

import (
	"context"

	"github.com/saucelabs/saucectl/internal/devices/devicestatus"
)

// Device describes a real device that can be used to run tests.
type Device struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	OS   string `json:"os"`
}

type DeviceStatus struct {
	ID              string
	Status          devicestatus.Status
	InUseBy         []string
	IsPrivateDevice bool
}

// Reader is the interface for retrieving available devices.
type Reader interface {
	GetDevices(ctx context.Context) ([]Device, error)
}

// StatusReader is the interface for retrieving available devices' statuses.
type StatusReader interface {
	GetDevicesStatuses(ctx context.Context) ([]DeviceStatus, error)
}

// ByOSReader is the interface for retrieving available devices by OS.
type ByOSReader interface {
	GetDevicesByOS(ctx context.Context, OS string) ([]Device, error)
}
