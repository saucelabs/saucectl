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
	Timeout time.Duration `json:"-"`

	User           string                 `json:"username"`
	AccessKey      string                 `json:"accessKey"`
	App            string                 `json:"app,omitempty"`
	TestApp        string                 `json:"testApp,omitempty"`
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

	Attempt           int                    `json:"-"`
	CurrentPassCount  int                    `json:"-"`
	BrowserName       string                 `json:"browserName,omitempty"`
	BrowserVersion    string                 `json:"browserVersion,omitempty"`
	PlatformName      string                 `json:"platformName,omitempty"`
	PlatformVersion   string                 `json:"platformVersion,omitempty"`
	DeviceID          string                 `json:"deviceId,omitempty"`
	DeviceName        string                 `json:"deviceName,omitempty"`
	DeviceOrientation string                 `json:"deviceOrientation"`
	DevicePrivateOnly bool                   `json:"devicePrivateOnly,omitempty"`
	DeviceType        string                 `json:"deviceType,omitempty"`
	DeviceHasCarrier  bool                   `json:"deviceHasCarrier,omitempty"`
	RealDevice        bool                   `json:"realDevice,omitempty"`
	Name              string                 `json:"name,omitempty"`
	Build             string                 `json:"build,omitempty"`
	Tags              []string               `json:"tags,omitempty"`
	Tunnel            TunnelOptions          `json:"tunnel,omitempty"`
	ScreenResolution  string                 `json:"screenResolution,omitempty"`
	Retries           int                    `json:"-"`
	PassThreshold     int                    `json:"-"`
	SmartRetry        SmartRetry             `json:"-"`
	RunnerVersion     string                 `json:"runnerVersion,omitempty"`
	Experiments       map[string]string      `json:"experiments,omitempty"`
	TestOptions       map[string]interface{} `json:"testOptions,omitempty"`
	TestsToRun        []string               `json:"testsToRun,omitempty"`
	TestsToSkip       []string               `json:"testsToSkip,omitempty"`
	StartTime         time.Time              `json:"startTime,omitempty"`
	AppSettings       AppSettings            `json:"appSettings,omitempty"`
	RealDeviceKind    string                 `json:"realDeviceKind,omitempty"`
	TimeZone          string                 `json:"timeZone,omitempty"`
	Visibility        string                 `json:"public,omitempty"`
	Env               map[string]string      `json:"-"`

	// VMD specific settings.

	ARMRequired bool `json:"armRequired,omitempty"`
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
