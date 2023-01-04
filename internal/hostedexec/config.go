package hostedexec

import "github.com/saucelabs/saucectl/internal/config"

var (
	Kind       = "htexec"
	APIVersion = "v1alpha"
)

type Project struct {
	config.TypeDef `yaml:",inline" mapstructure:",squash"`
	ConfigFilePath string             `yaml:"-" json:"-"`
	Defaults       config.Defaults    `yaml:"defaults" json:"defaults"`
	Sauce          config.SauceConfig `yaml:"sauce,omitempty" json:"sauce"`
	Suites         []Suite            `yaml:"suites,omitempty" json:"suites"`
}

type Suite struct {
	Name       string            `yaml:"name,omitempty" json:"name"`
	Image      string            `yaml:"image,omitempty" json:"image"`
	EntryPoint string            `yaml:"entrypoint,omitempty" json:"entrypoint"`
	Files      []File            `yaml:"files,omitempty" json:"files"`
	Artifacts  []string          `yaml:"artifacts,omitempty" json:"artifacts"`
	Env        map[string]string `yaml:"env,omitempty" json:"env"`
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
