package job

import (
	"context"
	"time"

	"github.com/saucelabs/saucectl/internal/report"
)

// StartOptions represents the options for starting a job in the Sauce Labs cloud.
type StartOptions struct {
	// DisplayName is used for local logging purposes only (e.g. console).
	DisplayName string `json:"-"`

	// PrevAttempts contains any previous attempts of the job.
	PrevAttempts []report.Attempt `json:"-"`

	// Timeout is used for local/per-suite timeout.
	Timeout   time.Duration `json:"-"`
	StartTime time.Time     `json:"startTime,omitempty"`

	User      string `json:"username"`
	AccessKey string `json:"accessKey"`

	App       string   `json:"app,omitempty"`
	OtherApps []string `json:"otherApps,omitempty"`

	Suite string `json:"suite,omitempty"`

	Framework string `json:"framework,omitempty"`

	// FrameworkVersion contains the targeted version of the framework.
	// It should not be confused with RunnerVersion.
	FrameworkVersion string `json:"frameworkVersion,omitempty"`

	PlatformName    string `json:"platformName,omitempty"`
	PlatformVersion string `json:"platformVersion,omitempty"`

	Tunnel TunnelOptions `json:"tunnel,omitempty"`

	Experiments map[string]string `json:"experiments,omitempty"`

	// Job Metadata.

	Name  string   `json:"name,omitempty"`
	Build string   `json:"build,omitempty"`
	Tags  []string `json:"tags,omitempty"`

	// Job Access Control.

	Visibility string `json:"public,omitempty"`

	// Thresholds & Retries.

	Attempt          int `json:"-"`
	CurrentPassCount int `json:"-"`
	PassThreshold    int `json:"-"`

	Retries    int        `json:"-"`
	SmartRetry SmartRetry `json:"-"`

	// Cypress & Playwright & TestCafe only.

	BrowserName      string `json:"browserName,omitempty"`
	BrowserVersion   string `json:"browserVersion,omitempty"`
	TimeZone         string `json:"timeZone,omitempty"`
	RunnerVersion    string `json:"runnerVersion,omitempty"`
	ScreenResolution string `json:"screenResolution,omitempty"`

	// RDC & VMD only.

	TestApp           string                 `json:"testApp,omitempty"`
	DeviceName        string                 `json:"deviceName,omitempty"`
	DeviceOrientation string                 `json:"deviceOrientation"`
	TestOptions       map[string]interface{} `json:"testOptions,omitempty"`

	// RDC only.

	AppSettings       AppSettings `json:"appSettings,omitempty"`
	DeviceID          string      `json:"deviceId,omitempty"`
	DeviceHasCarrier  bool        `json:"deviceHasCarrier,omitempty"`
	DevicePrivateOnly bool        `json:"devicePrivateOnly,omitempty"`
	DeviceType        string      `json:"deviceType,omitempty"`
	RealDevice        bool        `json:"realDevice,omitempty"`
	TestsToRun        []string    `json:"testsToRun,omitempty"`
	TestsToSkip       []string    `json:"testsToSkip,omitempty"`
	RealDeviceKind    string      `json:"realDeviceKind,omitempty"`

	// VMD specific settings.

	ARMRequired bool              `json:"armRequired,omitempty"`
	Env         map[string]string `json:"-"`

	// CLI.

	ConfigFilePath string                 `json:"-"`
	CLIFlags       map[string]interface{} `json:"-"`
}

// AppSettings represents app settings for real device
type AppSettings struct {
	AudioCapture    bool            `json:"audio_capture,omitempty"`
	Instrumentation Instrumentation `json:"instrumentation,omitempty"`
}

// Instrumentation represents instrumentation settings for real device
type Instrumentation struct {
	NetworkCapture bool `json:"network_capture,omitempty"`
}

// TunnelOptions represents the options that configure the usage of a tunnel when running tests in the Sauce Labs cloud.
type TunnelOptions struct {
	ID     string `json:"id"`
	Parent string `json:"parent,omitempty"`
}

// SmartRetry represents the retry strategy.
type SmartRetry struct {
	FailedOnly bool `json:"-"`
}

// Starter is the interface for starting jobs.
type Starter interface {
	StartJob(ctx context.Context, opts StartOptions) (jobID string, isRDC bool, err error)
}
