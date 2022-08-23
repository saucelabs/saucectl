package apif

import "github.com/saucelabs/saucectl/internal/config"

// Config descriptors.
var (
	// Kind represents the type definition of this config.
	Kind = "apif"

	// APIVersion represents the supported config version.
	APIVersion = "v1alpha"
)

// Project represents the apif project configuration.
type Project struct {
	config.TypeDef `yaml:",inline" mapstructure:",squash"`
	ConfigFilePath string             `yaml:"-" json:"-"`
	Suites         []Suite            `yaml:"suites,omitempty"`
	Sauce          config.SauceConfig `yaml:"sauce,omitempty"`
}

// Suite represents the apif suite configuration.
type Suite struct {
	Name   string   `yaml:"name,omitempty"`
	HookId string   `yaml:"hookId,omitempty"`
	Tags   []string `yaml:"tags,omitempty"`
	Tests  []string `yaml:"tests,omitempty"`
}

// FromFile creates a new apif Project based on the filepath cfgPath.
func FromFile(cfgPath string) (Project, error) {
	var p Project

	if err := config.Unmarshal(cfgPath, &p); err != nil {
		return p, err
	}
	p.ConfigFilePath = cfgPath

	return p, nil
}

func SetDefaults(p *Project) {
	if p.Kind == "" {
		p.Kind = Kind
	}

	if p.APIVersion == "" {
		p.APIVersion = APIVersion
	}

	if p.Sauce.Concurrency < 1 {
		p.Sauce.Concurrency = 2
	}

	p.Sauce.Tunnel.SetDefaults()
}
