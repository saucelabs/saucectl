package suite

import "time"

// Suite represents the general test suite configuration.
type Suite struct {
	Name             string            `yaml:"name,omitempty" json:"name"`
	Browser          string            `yaml:"browser,omitempty" json:"browser"`
	BrowserVersion   string            `yaml:"browserVersion,omitempty" json:"browserVersion"`
	PlatformName     string            `yaml:"platformName,omitempty" json:"platformName"`
	ScreenResolution string            `yaml:"screenResolution,omitempty" json:"screenResolution"`
	Mode             string            `yaml:"mode,omitempty" json:"-"`
	Timeout          time.Duration     `yaml:"timeout,omitempty" json:"timeout"`
	Shard            string            `yaml:"shard,omitempty" json:"-"`
	Headless         bool              `yaml:"headless,omitempty" json:"headless"`
	PreExec          []string          `yaml:"preExec,omitempty" json:"preExec"`
	TimeZone         string            `yaml:"timeZone,omitempty" json:"timeZone"`
	Env              map[string]string `yaml:"env,omitempty" json:"env"`
}
