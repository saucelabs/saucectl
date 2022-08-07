package suite

import (
	"time"

	"github.com/saucelabs/saucectl/internal/insights"
)

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

func SortByHistory(suites []Suite, history insights.TestHistory) []Suite {
	hash := map[string]Suite{}
	for _, s := range suites {
		hash[s.Name] = s
	}
	res := []Suite{}
	for _, s := range history.TestCases {
		if v, ok := hash[s.Name]; ok {
			res = append(res, v)
		}
	}
	return res
}
