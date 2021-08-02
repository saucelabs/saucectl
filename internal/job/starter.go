package job

import (
	"context"
	"time"
)

// TestOptions represents the espresso test filter options configuration.
type TestOptions struct {
	NotClass            []string `yaml:"notClass,omitempty" json:"notClass"`
	Class               []string `yaml:"class,omitempty" json:"class"`
	Package             string   `yaml:"package,omitempty" json:"package"`
	Size                string   `yaml:"size,omitempty" json:"size"`
	Annotation          string   `yaml:"annotation,omitempty" json:"annotation"`
	ShardIndex          *int     `yaml:"shardIndex,omitempty" json:"shardIndex"`
	NumShards           *int     `yaml:"numShards,omitempty" json:"numShards"`
	ClearPackageData    *bool    `yaml:"clearPackageData,omitempty" json:"clearPackageData"`
	UseTestOrchestrator *bool    `yaml:"useTestOrchestrator,omitempty" json:"useTestOrchestrator"`
}

// StartOptions represents the options for starting a job in the Sauce Labs cloud.
type StartOptions struct {
	// DisplayName is used for local logging purposes only (e.g. console).
	DisplayName string `json:"-"`

	// Timeout is used for local/per-suite timeout.
	Timeout time.Duration `json:"-"`

	User           string                 `json:"username"`
	AccessKey      string                 `json:"accessKey"`
	App            string                 `json:"app,omitempty"`
	Suite          string                 `json:"suite,omitempty"`
	OtherApps      []string               `json:"otherApps,omitempty"`
	Framework      string                 `json:"framework,omitempty"`
	ConfigFilePath string                 `json:"-"`
	CLIFlags       map[string]interface{} `json:"-"`

	// FrameworkVersion contains the targeted version of the framework
	// It should not be confused with automation tool (like jest/folio).
	// This is currently supported only for frameworks available on Sauce Cloud:
	// Currently supported: Cypress.
	FrameworkVersion string `json:"frameworkVersion,omitempty"`

	BrowserName       string            `json:"browserName,omitempty"`
	BrowserVersion    string            `json:"browserVersion,omitempty"`
	PlatformName      string            `json:"platformName,omitempty"`
	PlatformVersion   string            `json:"platformVersion,omitempty"`
	DeviceID          string            `json:"deviceId,omitempty"`
	DeviceName        string            `json:"deviceName,omitempty"`
	DeviceOrientation string            `json:"deviceOrientation"`
	DevicePrivateOnly bool              `json:"devicePrivateOnly,omitempty"`
	DeviceType        string            `json:"deviceType,omitempty"`
	DeviceHasCarrier  bool              `json:"deviceHasCarrier,omitempty"`
	RealDevice        bool              `json:"realDevice,omitempty"`
	Name              string            `json:"name,omitempty"`
	Build             string            `json:"build,omitempty"`
	Tags              []string          `json:"tags,omitempty"`
	Tunnel            TunnelOptions     `json:"tunnel,omitempty"`
	ScreenResolution  string            `json:"screenResolution,omitempty"`
	RunnerVersion     string            `json:"runnerVersion,omitempty"`
	Experiments       map[string]string `json:"experiments,omitempty"`
	TestOptions       TestOptions       `json:"testOptions,omitempty"`
	TestsToRun        []string          `json:"testsToRun,omitempty"`
	TestsToSkip       []string          `json:"testsToSkip,omitempty"`
}

// TunnelOptions represents the options that configure the usage of a tunnel when running tests in the Sauce Labs cloud.
type TunnelOptions struct {
	ID     string `json:"id"`
	Parent string `json:"parent,omitempty"`
}

// Starter is the interface for starting jobs.
type Starter interface {
	StartJob(ctx context.Context, opts StartOptions) (jobID string, isRDC bool, err error)
}
