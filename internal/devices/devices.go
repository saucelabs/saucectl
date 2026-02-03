package devices

import (
	"context"

	"github.com/saucelabs/saucectl/internal/devices/devicestatus"
)

// Device describes a real device that can be used to run tests.
type Device struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	OS        string `json:"os"`
	OSVersion string `json:"osVersion"`
}

type DeviceStatus struct {
	ID              string
	Status          devicestatus.Status
	InUseBy         []string
	IsPrivateDevice bool
}

type DeviceWithStatus struct {
	ID        string              `json:"id"`
	Name      string              `json:"name"`
	OS        string              `json:"os"`
	OSVersion string              `json:"osVersion"`
	Status    devicestatus.Status `json:"status"`
}

// DeviceDetails describes detailed specifications of a real device.
type DeviceDetails struct {
	AbiType                         string   `json:"abiType,omitempty"`
	APILevel                        int      `json:"apiLevel,omitempty"`
	CPUCores                        int      `json:"cpuCores,omitempty"`
	CPUFrequency                    int      `json:"cpuFrequency,omitempty"`
	CPUType                         string   `json:"cpuType,omitempty"`
	DefaultOrientation              string   `json:"defaultOrientation,omitempty"`
	DeviceFamily                    string   `json:"deviceFamily,omitempty"`
	Dpi                             int      `json:"dpi,omitempty"`
	DpiName                         string   `json:"dpiName,omitempty"`
	HasOnScreenButtons              bool     `json:"hasOnScreenButtons,omitempty"`
	ID                              string   `json:"id"`
	InternalOrientation             string   `json:"internalOrientation,omitempty"`
	InternalStorageSize             int      `json:"internalStorageSize,omitempty"`
	IsAlternativeIoEnabled          bool     `json:"isAlternativeIoEnabled,omitempty"`
	IsArm                           bool     `json:"isArm,omitempty"`
	IsKeyGuardDisabled              bool     `json:"isKeyGuardDisabled,omitempty"`
	IsPrivate                       bool     `json:"isPrivate,omitempty"`
	IsRooted                        bool     `json:"isRooted,omitempty"`
	IsTablet                        bool     `json:"isTablet,omitempty"`
	Manufacturer                    []string `json:"manufacturer,omitempty"`
	ModelNumber                     string   `json:"modelNumber,omitempty"`
	Name                            string   `json:"name"`
	OS                              string   `json:"os"`
	OSVersion                       string   `json:"osVersion"`
	PixelsPerPoint                  int      `json:"pixelsPerPoint,omitempty"`
	RAMSize                         int      `json:"ramSize,omitempty"`
	ResolutionHeight                int      `json:"resolutionHeight,omitempty"`
	ResolutionWidth                 int      `json:"resolutionWidth,omitempty"`
	ScreenSize                      float64  `json:"screenSize,omitempty"`
	SdCardSize                      int      `json:"sdCardSize,omitempty"`
	SupportsAppiumWebAppTesting     bool     `json:"supportsAppiumWebAppTesting,omitempty"`
	SupportsGlobalProxy             bool     `json:"supportsGlobalProxy,omitempty"`
	SupportsManualWebTesting        bool     `json:"supportsManualWebTesting,omitempty"`
	SupportsMinicapSocketConnection bool     `json:"supportsMinicapSocketConnection,omitempty"`
	SupportsMockLocations           bool     `json:"supportsMockLocations,omitempty"`
	SupportsMultiTouch              bool     `json:"supportsMultiTouch,omitempty"`
	SupportsXcuiTest                bool     `json:"supportsXcuiTest,omitempty"`
}

// Reader is the interface for retrieving available devices.
type Reader interface {
	GetDevices(ctx context.Context) ([]Device, error)
}

// SingleReader is the interface for retrieving a single device by ID.
type SingleReader interface {
	GetDevice(ctx context.Context, id string) (DeviceDetails, error)
}

// StatusReader is the interface for retrieving available devices' statuses.
type StatusReader interface {
	GetDevicesStatuses(ctx context.Context) ([]DeviceStatus, error)
	GetDevicesWithStatuses(ctx context.Context) ([]DeviceWithStatus, error)
}

// ByOSReader is the interface for retrieving available devices by OS.
type ByOSReader interface {
	GetDevicesByOS(ctx context.Context, OS string) ([]Device, error)
}
