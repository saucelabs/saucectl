package imagerunner

import (
	"github.com/saucelabs/saucectl/internal/config"
	"time"
)

var (
	Kind       = "imagerunner"
	APIVersion = "v1alpha"
)

type Project struct {
	config.TypeDef `yaml:",inline" mapstructure:",squash"`
	ConfigFilePath string             `yaml:"-" json:"-"` // TODO This field is irrelevant. Delete.
	Defaults       Defaults           `yaml:"defaults" json:"defaults"`
	Sauce          config.SauceConfig `yaml:"sauce,omitempty" json:"sauce"` // The only field that's used within 'sauce' is region.
	Suites         []Suite            `yaml:"suites,omitempty" json:"suites"`
	Artifacts      config.Artifacts   `yaml:"artifacts,omitempty" json:"artifacts"`
}

type Defaults struct {
	Suite `yaml:",inline" mapstructure:",squash"`
}

type Suite struct {
	Name          string            `yaml:"name,omitempty" json:"name"`
	Image         string            `yaml:"image,omitempty" json:"image"`
	ImagePullAuth ImagePullAuth     `yaml:"imagePullAuth,omitempty" json:"imagePullAuth"`
	EntryPoint    string            `yaml:"entrypoint,omitempty" json:"entrypoint"`
	Files         []File            `yaml:"files,omitempty" json:"files"`
	Artifacts     []string          `yaml:"artifacts,omitempty" json:"artifacts"`
	Env           map[string]string `yaml:"env,omitempty" json:"env"`
	Timeout       time.Duration     `yaml:"timeout,omitempty" json:"timeout"`
}

type ImagePullAuth struct {
	User  string `yaml:"user,omitempty" json:"user"`
	Token string `yaml:"token,omitempty" json:"token"`
}

type File struct {
	Src string `yaml:"src,omitempty" json:"src"`
	Dst string `yaml:"dst,omitempty" json:"dst"`
}

func FromFile(cfgPath string) (Project, error) {
	var p Project

	if err := config.Unmarshal(cfgPath, &p); err != nil {
		return p, err
	}

	p.ConfigFilePath = cfgPath

	return p, nil
}
