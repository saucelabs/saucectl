package devices

import "context"

// Device describes a real device that can be used to run tests.
type Device struct {
	Name string
	OS   string
}

// Reader is the interface for retrieving available devices.
type Reader interface {
	GetDevices(ctx context.Context, OS string) ([]Device, error)
}
