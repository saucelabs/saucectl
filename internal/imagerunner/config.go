package imagerunner

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/region"
)

var (
	Kind       = "imagerunner"
	APIVersion = "v1alpha"

	ValidWorkloadType = []string{
		"webdriver",
		"other",
	}

	ValidResourceProfilesFormat    = "cXmX"
	ValidResourceProfilesValidator = regexp.MustCompile("^c([0-9]+)m([0-9]+)$")
)

type Project struct {
	config.TypeDef `yaml:",inline" mapstructure:",squash"`
	Defaults       Defaults           `yaml:"defaults" json:"defaults"`
	Sauce          config.SauceConfig `yaml:"sauce,omitempty" json:"sauce"` // Used fields are `region` and `tunnel`.
	Suites         []Suite            `yaml:"suites,omitempty" json:"suites"`
	Artifacts      config.Artifacts   `yaml:"artifacts,omitempty" json:"artifacts"`
	DryRun         bool               `yaml:"-" json:"-"`
	LiveLogs       bool               `yaml:"-" json:"-"`
	Env            map[string]string  `yaml:"env,omitempty" json:"env"`
	EnvFlag        map[string]string  `yaml:"-" json:"-"`
	Reporters      config.Reporters   `yaml:"reporters,omitempty" json:"-"`
}

type Defaults struct {
	Suite `yaml:",inline" mapstructure:",squash"`
}

type Suite struct {
	Name            string            `yaml:"name,omitempty" json:"name"`
	Image           string            `yaml:"image,omitempty" json:"image"`
	ImagePullAuth   ImagePullAuth     `yaml:"imagePullAuth,omitempty" json:"imagePullAuth"`
	EntryPoint      string            `yaml:"entrypoint,omitempty" json:"entrypoint"`
	Files           []File            `yaml:"files,omitempty" json:"files"`
	Artifacts       []string          `yaml:"artifacts,omitempty" json:"artifacts"`
	Env             map[string]string `yaml:"env,omitempty" json:"env"`
	Timeout         time.Duration     `yaml:"timeout,omitempty" json:"timeout"`
	Workload        string            `yaml:"workload,omitempty" json:"workload,omitempty"`
	ResourceProfile string            `yaml:"resourceProfile,omitempty" json:"resourceProfile,omitempty"`
	Metadata        map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	Services        []SuiteService    `yaml:"services,omitempty" json:"services,omitempty"`
}

type SuiteService struct {
	Name            string            `yaml:"name,omitempty" json:"name"`
	Image           string            `yaml:"image,omitempty" json:"image"`
	ImagePullAuth   ImagePullAuth     `yaml:"imagePullAuth,omitempty" json:"imagePullAuth"`
	EntryPoint      string            `yaml:"entrypoint,omitempty" json:"entrypoint"`
	Files           []File            `yaml:"files,omitempty" json:"files"`
	Env             map[string]string `yaml:"env,omitempty" json:"env"`
	ResourceProfile string            `yaml:"resourceProfile,omitempty" json:"resourceProfile,omitempty"`
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

	return p, nil
}

// SetDefaults applies config defaults in case the user has left them blank.
func SetDefaults(p *Project) {
	if p.Kind == "" {
		p.Kind = Kind
	}

	if p.APIVersion == "" {
		p.APIVersion = APIVersion
	}

	if p.Sauce.Concurrency < 1 {
		p.Sauce.Concurrency = 1
	}

	if p.Sauce.Concurrency > 5 {
		log.Warn().Msgf(msg.ImageRunnerMaxConcurrency, p.Sauce.Concurrency)
		p.Sauce.Concurrency = 5
	}

	if p.Defaults.Timeout < 0 {
		p.Defaults.Timeout = 0
	}

	p.Sauce.Tunnel.SetDefaults()
	p.Sauce.Metadata.SetDefaultBuild()

	for i := range p.Suites {
		suite := &p.Suites[i]
		if suite.Timeout <= 0 {
			suite.Timeout = p.Defaults.Timeout
		}

		if suite.Workload == "" {
			suite.Workload = p.Defaults.Workload
		}

		if suite.ResourceProfile == "" {
			suite.ResourceProfile = "c1m1"
		}

		if suite.Env == nil {
			suite.Env = make(map[string]string)
		}
		if suite.Metadata == nil {
			suite.Metadata = make(map[string]string)
		}

		suite.Metadata["name"] = suite.Name
		suite.Metadata["resourceProfile"] = suite.ResourceProfile

		// Precedence: --env flag > root-level env vars > default env vars > suite env vars.
		for _, env := range []map[string]string{p.Defaults.Env, p.Env, p.EnvFlag} {
			for k, v := range env {
				suite.Env[k] = v
			}
		}

		for j := range suite.Services {
			service := &suite.Services[j]
			if service.ResourceProfile == "" {
				service.ResourceProfile = "c1m1"
			}
			suite.Metadata[fmt.Sprintf("resourceProfile-%s", GetCanonicalServiceName(service.Name))] = service.ResourceProfile
			if service.Env == nil {
				service.Env = make(map[string]string)
			}
			// Precedence: --env flag > root-level env vars > default env vars > service env vars.
			for _, env := range []map[string]string{p.Defaults.Env, p.Env, p.EnvFlag} {
				for k, v := range env {
					service.Env[k] = v
				}
			}
		}
	}
}

func Validate(p Project) error {
	regio := region.FromString(p.Sauce.Region)
	if regio == region.None {
		return errors.New(msg.MissingRegion)
	}

	if len(p.Suites) == 0 {
		return errors.New(msg.EmptySuite)
	}

	for _, suite := range p.Suites {
		if suite.Workload == "" {
			return fmt.Errorf(msg.MissingImageRunnerWorkloadType, suite.Name)
		}

		if !sliceContainsString(ValidWorkloadType, suite.Workload) {
			return fmt.Errorf(msg.InvalidImageRunnerWorkloadType, suite.Workload, suite.Name)
		}

		if suite.Image == "" {
			return fmt.Errorf(msg.MissingImageRunnerImage, suite.Name)
		}

		if suite.ResourceProfile != "" && !ValidResourceProfilesValidator.MatchString(suite.ResourceProfile) {
			return fmt.Errorf(msg.InvalidResourceProfile, suite.Name, ValidResourceProfilesFormat)
		}
		if err := ValidateServices(suite.Services, suite.Name); err != nil {
			return err
		}
	}
	return nil
}

func ValidateServices(service []SuiteService, suiteName string) error {
	for _, service := range service {
		if service.Name == "" {
			return fmt.Errorf(msg.MissingServiceName, suiteName)
		}
		if service.Image == "" {
			return fmt.Errorf(msg.MissingServiceImage, service.Name, suiteName)
		}
		if service.ResourceProfile != "" && !ValidResourceProfilesValidator.MatchString(service.ResourceProfile) {
			return fmt.Errorf(msg.InvalidServiceResourceProfile, service.Name, suiteName, ValidResourceProfilesFormat)
		}
	}
	return nil
}

func sliceContainsString(slice []string, val string) bool {
	for _, value := range slice {
		if value == val {
			return true
		}
	}
	return false
}

// FilterSuites filters out suites in the project that don't match the given suite name.
func FilterSuites(p *Project, suiteName string) error {
	for _, s := range p.Suites {
		if s.Name == suiteName {
			p.Suites = []Suite{s}
			return nil
		}
	}
	return fmt.Errorf(msg.SuiteNameNotFound, suiteName)
}
