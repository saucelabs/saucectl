package job

import "context"

// StartOptions represents the options for starting a job.
type StartOptions struct {
	User           string   `json:"username"`
	AccessKey      string   `json:"accessKey"`
	App            string   `json:"app,omitempty"`
	Suite          string   `json:"suite,omitempty"`
	Framework      string   `json:"framework,omitempty"`
	BrowserName    string   `json:"browserName,omitempty"`
	BrowserVersion string   `json:"browserVersion,omitempty"`
	PlatformName   string   `json:"platformName,omitempty"`
	Name           string   `json:"name,omitempty"`
	Build          string   `json:"build,omitempty"`
	Tags           []string `json:"tags,omitempty"`
}

// Starter is the interface for starting jobs.
type Starter interface {
	StartJob(ctx context.Context, opts StartOptions) (jobID string, err error)
}
