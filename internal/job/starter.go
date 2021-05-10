package job

import (
	"context"
)

// TestOptions represents the espresso test filter options configuration.
type TestOptions struct {
	NotClass   []string `yaml:"notClass,omitempty" json:"notClass"`
	Class      []string `yaml:"class,omitempty" json:"class"`
	Package    string   `yaml:"package,omitempty" json:"package"`
	Size       string   `yaml:"size,omitempty" json:"size"`
	Annotation string   `yaml:"annotation,omitempty" json:"annotation"`
}

// StartOptions represents the options for starting a job in the Sauce Labs cloud.
type StartOptions struct {
	// DisplayName is used for local logging purposes only (e.g. console).
	DisplayName string `json:"-"`

	User           string `json:"username"`
	AccessKey      string `json:"accessKey"`
	App            string `json:"app,omitempty"`
	Suite          string `json:"suite,omitempty"`
	Framework      string `json:"framework,omitempty"`
	ConfigFilePath string `json:"-"`

	// FrameworkVersion contains the targeted version of the framework
	// It should not be confused with automation tool (like jest/folio).
	// This is currently supported only for frameworks available on Sauce Cloud:
	// Currently supported: Cypress.
	FrameworkVersion string `json:"frameworkVersion,omitempty"`

	BrowserName       string            `json:"browserName,omitempty"`
	BrowserVersion    string            `json:"browserVersion,omitempty"`
	PlatformName      string            `json:"platformName,omitempty"`
	PlatformVersion   string            `json:"platformVersion,omitempty"`
	DeviceName        string            `json:"deviceName,omitempty"`
	DeviceOrientation string            `json:"deviceOrientation"`
	Name              string            `json:"name,omitempty"`
	Build             string            `json:"build,omitempty"`
	Tags              []string          `json:"tags,omitempty"`
	Tunnel            TunnelOptions     `json:"tunnel,omitempty"`
	ScreenResolution  string            `json:"screenResolution,omitempty"`
	RunnerVersion     string            `json:"runnerVersion,omitempty"`
	Experiments       map[string]string `json:"experiments,omitempty"`
	TestOptions       TestOptions       `json:"testOptions,omitempty"`
}

// TunnelOptions represents the options that configure the usage of a tunnel when running tests in the Sauce Labs cloud.
type TunnelOptions struct {
	ID     string `json:"id"`
	Parent string `json:"parent,omitempty"`
}

// Starter is the interface for starting jobs.
type Starter interface {
	StartJob(ctx context.Context, opts StartOptions) (jobID string, err error)
}

// The different device selectors possible for a RDC Job.
const (
	RDCTypeDynamicDeviceQuery = "DynamicDeviceQuery"
	RDCTypeHardcodedDeviceQuery = "HardcodedDeviceQuery"
)

// RDCDeviceQuery represents the device query for RDC tests.
type RDCDeviceQuery struct {
	Type               string `json:"type,omitempty"`
	DeviceDescriptorID string `json:"device_descriptor_id,omitempty"`
	RequestDeviceType  string `json:"requested_device_type,omitempty"`
}

// RDCStarterOptions represents the options for starting a job on RDC Cloud.
type RDCStarterOptions struct {
	TestFramework string            `json:"test_framework"`
	AppID         string            `json:"app_id"`
	TestAppID     string            `json:"test_app_id"`
	DeviceQuery   RDCDeviceQuery    `json:"device_query,omitempty"`
	TestOptions   map[string]string `json:"test_options,omitempty"`
	TestName      string            `json:"test_name,omitempty"`
}
