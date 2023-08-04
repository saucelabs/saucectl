package vmrunner

import (
	"time"

	"github.com/saucelabs/saucectl/internal/config"
)

// Config descriptors.
var (
	// Kind represents the type definition of this config.
	Kind = "vmrunner"

	// APIVersion represents the supported config version.
	APIVersion = "v1alpha"
)

// Project represents the vmrunner project configuration.
type Project struct {
	config.TypeDef `yaml:",inline" mapstructure:",squash"`
	ConfigFilePath string `yaml:"-" json:"-"`
	DryRun         bool   `yaml:"-" json:"-"`

	RootDir       string `yaml:"rootDir,omitempty" json:"rootDir"`
	Workload      string `yaml:"workload,omitempty" json:"workload"`
	RunnerVersion string `yaml:"runnerVersion,omitempty" json:"runnerVersion"`

	Sauce  config.SauceConfig `yaml:"sauce,omitempty" json:"sauce"`
	Suites []Suite            `yaml:"suites,omitempty" json:"suites"`

	Artifacts config.Artifacts `yaml:"artifacts,omitempty" json:"artifacts"`

	Reporters config.Reporters `yaml:"reporters,omitempty" json:"-"`

	Runtime Runtime `yaml:"runtime,omitempty" json:"runtime"`
}

type Runtime struct {
	Node             Node `yaml:"node,omitempty" json:"node"`
	StoreCache       bool `yaml:"storeCache,omitempty" json:"storeCache"`
	SkipCacheRestore bool `yaml:"skipCacheRestore,omitempty" json:"skipCacheRestore"`
}

type Node struct {
	Version string `yaml:"version,omitempty" json:"version"`
}

// Suite represents the testcafe test suite configuration.
type Suite struct {
	Name             string        `yaml:"name,omitempty" json:"name"`
	Browser          string        `yaml:"browser,omitempty" json:"browser"`
	BrowserVersion   string        `yaml:"browserVersion,omitempty" json:"browserVersion"`
	Platform         string        `yaml:"platform,omitempty" json:"platform"`
	ScreenResolution string        `yaml:"screenResolution,omitempty" json:"screenResolution"`
	TimeZone         string        `json:"timeZone,omitempty"`
	Timeout          time.Duration `yaml:"timeout,omitempty" json:"timeout"`
	Exec             []string      `yaml:"exec,omitempty" json:"exec"`
}

// FromFile creates a new testcafe project based on the filepath.
func FromFile(cfgPath string) (Project, error) {
	var p Project
	if err := config.Unmarshal(cfgPath, &p); err != nil {
		return p, err
	}
	return p, nil
}
