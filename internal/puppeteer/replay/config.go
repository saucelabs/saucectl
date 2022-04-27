package replay

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/fpath"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/region"
)

// Config descriptors.
var (
	// Kind represents the type definition of this config.
	Kind = "puppeteer-replay"

	// APIVersion represents the supported config version.
	APIVersion = "v1alpha"
)

// Project represents the replay project configuration.
type Project struct {
	config.TypeDef `yaml:",inline" mapstructure:",squash"`
	ConfigFilePath string             `yaml:"-" json:"-"`
	DryRun         bool               `yaml:"-" json:"-"`
	ShowConsoleLog bool               `yaml:"showConsoleLog" json:"-"`
	Defaults       config.Defaults    `yaml:"defaults,omitempty" json:"defaults"`
	Sauce          config.SauceConfig `yaml:"sauce,omitempty" json:"sauce"`
	// Suite is only used as a workaround to parse adhoc suites that are created via CLI args.
	Suite         Suite                `yaml:"suite,omitempty" json:"-"`
	Suites        []Suite              `yaml:"suites,omitempty" json:"suites"`
	Artifacts     config.Artifacts     `yaml:"artifacts,omitempty" json:"artifacts"`
	Reporters     config.Reporters     `yaml:"reporters,omitempty" json:"-"`
	Notifications config.Notifications `yaml:"notifications,omitempty" json:"-"`
}

// Suite represents the playwright test suite configuration.
type Suite struct {
	Name           string        `yaml:"name,omitempty" json:"name"`
	Timeout        time.Duration `yaml:"timeout,omitempty" json:"timeout"`
	Recording      string        `yaml:"recording,omitempty" json:"recording,omitempty"`
	BrowserName    string        `yaml:"browserName,omitempty" json:"browserName,omitempty"`
	BrowserVersion string        `yaml:"browserVersion,omitempty" json:"browserVersion,omitempty"`
	Platform       string        `yaml:"platform,omitempty" json:"platform,omitempty"`

	Recordings []string `yaml:"recordings,omitempty" json:"-"`
}

// FromFile creates a new replay Project based on the filepath cfgPath.
func FromFile(cfgPath string) (Project, error) {
	var p Project

	if err := config.Unmarshal(cfgPath, &p); err != nil {
		return p, err
	}

	p.ConfigFilePath = cfgPath

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
		p.Sauce.Concurrency = 2
	}

	if p.Defaults.Timeout < 0 {
		p.Defaults.Timeout = 0
	}

	p.Sauce.Tunnel.SetDefaults()
	p.Sauce.Metadata.SetDefaultBuild()

	for k := range p.Suites {
		s := &p.Suites[k]
		if s.Platform == "" {
			s.Platform = "Windows 10"
		}

		rgx := regexp.MustCompile(`^(?i)chrome$`)
		if s.BrowserName == "" || rgx.MatchString(s.BrowserName) {
			s.BrowserName = "googlechrome"
		}

		if s.Timeout <= 0 {
			s.Timeout = p.Defaults.Timeout
		}
	}
}

// Validate validates basic configuration of the project and returns an error if any of the settings contain illegal
// values. This is not an exhaustive operation and further validation should be performed both in the client and/or
// server side depending on the workflow that is executed.
func Validate(p *Project) error {
	reg := region.FromString(p.Sauce.Region)
	if reg == region.None {
		return errors.New(msg.MissingRegion)
	}

	rgx := regexp.MustCompile(`^(?i)(google)?chrome$`)
	for _, s := range p.Suites {
		if !rgx.MatchString(s.BrowserName) {
			return fmt.Errorf("browser %s is not supported, please use chrome instead or leave empty for defaults", s.BrowserName)
		}
	}

	return nil
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

// ShardSuites automatically shards the suites for each recording.
func ShardSuites(suites []Suite) ([]Suite, error) {
	var shardedSuites []Suite
	for _, s := range suites {
		testFiles, err := fpath.FindFiles(".", s.Recordings, fpath.FindByShellPattern)
		if err != nil {
			return []Suite{}, err
		}
		if len(testFiles) == 0 {
			msg.SuiteSplitNoMatch(s.Name, ".", s.Recordings)
			return shardedSuites, fmt.Errorf("suite '%s' patterns have no matching files", s.Name)
		}
		for _, f := range testFiles {
			if !strings.HasSuffix(f, ".json") {
				log.Warn().Msgf("Suite '%s' specifies non-json file '%s' as recording. Skipping.", s.Name, f)
				continue
			}
			replica := s
			replica.Name = fmt.Sprintf("%s - %s", s.Name, f)
			replica.Recording = f
			shardedSuites = append(shardedSuites, replica)
		}
	}

	if len(shardedSuites) == 0 {
		return shardedSuites, errors.New("no viable suites found")
	}

	return shardedSuites, nil
}
