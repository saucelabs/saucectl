package testcafe

import (
	"errors"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/config"
	"gopkg.in/yaml.v2"
)

// Project represents the testcafe project configuration.
type Project struct {
	config.TypeDef `yaml:",inline"`
	ShowConsoleLog bool
	DryRun         bool               `yaml:"-" json:"-"`
	Sauce          config.SauceConfig `yaml:"sauce,omitempty" json:"sauce"`
	Suites         []Suite            `yaml:"suites,omitempty" json:"suites"`
	BeforeExec     []string           `yaml:"beforeExec,omitempty" json:"beforeExec"`
	Docker         config.Docker      `yaml:"docker,omitempty" json:"docker"`
	Testcafe       Testcafe           `yaml:"testcafe,omitempty" json:"testcafe"`
	Npm            config.Npm         `yaml:"npm,omitempty" json:"npm"`
	RootDir        string             `yaml:"rootDir,omitempty" json:"rootDir"`
	RunnerVersion  string             `yaml:"runnerVersion,omitempty" json:"runnerVersion"`
}

// Suite represents the testcafe test suite configuration.
type Suite struct {
	Name               string            `yaml:"name,omitempty" json:"name"`
	BrowserName        string            `yaml:"browserName,omitempty" json:"browserName"`
	BrowserVersion     string            `yaml:"browserVersion,omitempty" json:"browserVersion"`
	Src                []string          `yaml:"src,omitempty" json:"src"`
	Screenshots        Screenshots       `yaml:"screenshots,omitempty" json:"screenshots"`
	PlatformName       string            `yaml:"platformName,omitempty" json:"platformName"`
	ScreenResolution   string            `yaml:"screenResolution,omitempty" json:"screenResolution"`
	Env                map[string]string `yaml:"env,omitempty" json:"env"`
	TsConfigPath       string            `yaml:"tsConfigPath,omitempty" json:"tsConfigPath"`
	ClientScripts      []string          `yaml:"clientScripts,omitempty" json:"clientScripts"`
	SkipJsErrors       bool              `yaml:"skipJsErrors,omitempty" json:"skipJsErrors"`
	QuarantineMode     bool              `yaml:"quarantineMode,omitempty" json:"quarantineMode"`
	SkipUncaughtErrors bool              `yaml:"skipUncaughtErrors,omitempty" json:"skipUncaughtErrors"`
	SelectorTimeout    int               `yaml:"selectorTimeout,omitempty" json:"selectorTimeout"`
	AssertionTimeout   int               `yaml:"assertionTimeout,omitempty" json:"assertionTimeout"`
	PageLoadTimeout    int               `yaml:"pageLoadTimeout,omitempty" json:"pageLoadTimeout"`
	Speed              float64           `yaml:"speed,omitempty" json:"speed"`
	StopOnFirstFail    bool              `yaml:"stopOnFirstFail,omitempty" json:"stopOnFirstFail"`
	DisablePageCaching bool              `yaml:"disablePageCaching,omitempty" json:"disablePageCaching"`
	DisableScreenshots bool              `yaml:"disableScreenshots,omitempty" json:"disableScreenshots"`
	DisableVideo       bool              `yaml:"disableVideo,omitempty" json:"disableVideo"` // This field is for sauce, not for native testcafe config.
}

// Screenshots represents screenshots configuration.
type Screenshots struct {
	TakeOnFails bool `yaml:"takeOnFails,omitempty" json:"takeOnFails"`
	FullPage    bool `yaml:"fullPage,omitempty" json:"fullPage"`
}

// Testcafe represents the configuration for testcafe.
type Testcafe struct {
	// ProjectPath is the path for testing project.
	ProjectPath string `yaml:"projectPath,omitempty" json:"projectPath"`

	// Version represents the testcafe framework version.
	Version string `yaml:"version,omitempty" json:"version"`
}

// FromFile creates a new testcafe project based on the filepath.
func FromFile(cfgPath string) (Project, error) {
	var p Project

	f, err := os.Open(cfgPath)
	defer f.Close()
	if err != nil {
		return p, fmt.Errorf("failed to locate project config: %v", err)
	}
	if err = yaml.NewDecoder(f).Decode(&p); err != nil {
		return p, fmt.Errorf("failed to parse project config: %v", err)
	}

	if p.Testcafe.ProjectPath == "" && p.RootDir == "" {
		return p, fmt.Errorf("could not find 'rootDir' in config yml, 'rootDir' must be set to specify project files")
	} else if p.Testcafe.ProjectPath != "" && p.RootDir == "" {
		log.Warn().Msg("'testcafe.projectPath' is deprecated. Consider using 'rootDir' instead")
		p.RootDir = p.Testcafe.ProjectPath
	} else if p.Testcafe.ProjectPath != "" && p.RootDir != "" {
		log.Info().Msgf(
			"Found both 'testcafe.projectPath=%s' and 'rootDir=%s' in config. 'projectPath' is deprecated, so defaulting to rootDir '%s'",
			p.Testcafe.ProjectPath, p.RootDir, p.RootDir,
		)
	}

	p.Testcafe.Version = config.StandardizeVersionFormat(p.Testcafe.Version)
	if p.Testcafe.Version == "" {
		return p, errors.New("missing framework version. Check available versions here: https://docs.staging.saucelabs.net/testrunner-toolkit#supported-frameworks-and-browsers")
	}

	// Set default docker file transfer to mount
	if p.Docker.FileTransfer == "" {
		p.Docker.FileTransfer = config.DockerFileMount
	}

	if p.Docker.Image != "" {
		log.Info().Msgf(
			"Ignoring framework version for Docker, using provided image %s (only applicable to docker mode)",
			p.Docker.Image)
	}

	if p.Sauce.Concurrency < 1 {
		// Default concurrency is 2
		p.Sauce.Concurrency = 2
	}

	for i, s := range p.Suites {
		env := map[string]string{}
		for k, v := range s.Env {
			env[k] = os.ExpandEnv(v)
		}
		p.Suites[i].Env = env
		setDefaultValues(&p.Suites[i])
	}
	return p, nil
}

func setDefaultValues(suite *Suite) {
	// If value is 0, it's assumed that the value has not been filled.
	// So we define it to the default value: 1 (full speed).
	// Expected values for TestCafe are between .01 and 1.
	if suite.Speed < .01 || suite.Speed > 1 {
		suite.Speed = 1
	}
	// Set default timeout. ref: https://devexpress.github.io/testcafe/documentation/reference/configuration-file.html#selectortimeout
	if suite.SelectorTimeout == 0 {
		suite.SelectorTimeout = 10000
	}
	if suite.AssertionTimeout == 0 {
		suite.AssertionTimeout = 3000
	}
	if suite.PageLoadTimeout == 0 {
		suite.PageLoadTimeout = 3000
	}
}
