package job

import "context"

// StartOptions represents the options for starting a job in the Sauce Labs cloud.
type StartOptions struct {
	User      string `json:"username"`
	AccessKey string `json:"accessKey"`
	App       string `json:"app,omitempty"`
	Suite     string `json:"suite,omitempty"`
	Framework string `json:"framework,omitempty"`

	// FrameworkVersion contains the targeted version of the framework
	// It should not be confused with automation tool (like jest/folio).
	// This is currenty supported only for frameworks available on Sauce Cloud:
	// Currently supported: Cypress.
	FrameworkVersion string `json:"frameworkVersion,omitempty"`

	BrowserName      string            `json:"browserName,omitempty"`
	BrowserVersion   string            `json:"browserVersion,omitempty"`
	PlatformName     string            `json:"platformName,omitempty"`
	PlatformVersion  string            `json:"platformVersion,omitempty"`
	DeviceName  	 string            `json:"deviceName,omitempty"`
	Name             string            `json:"name,omitempty"`
	Build            string            `json:"build,omitempty"`
	Tags             []string          `json:"tags,omitempty"`
	Tunnel           TunnelOptions     `json:"tunnel,omitempty"`
	ScreenResolution string            `json:"screenResolution,omitempty"`
	RunnerVersion    string            `json:"runnerVersion,omitempty"`
	Experiments      map[string]string `json:"experiments,omitempty"`
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
