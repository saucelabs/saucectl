package playwright

import (
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/config"
	"gopkg.in/yaml.v2"
	"os"
)

// Project represents the playwright project configuration.
type Project struct {
	config.TypeDef `yaml:",inline"`
	ShowConsoleLog bool
	DryRun         bool               `yaml:"-" json:"-"`
	Sauce          config.SauceConfig `yaml:"sauce,omitempty" json:"sauce"`
	Playwright     Playwright         `yaml:"playwright,omitempty" json:"playwright"`
	Suites         []Suite            `yaml:"suites,omitempty" json:"suites"`
	BeforeExec     []string           `yaml:"beforeExec,omitempty" json:"beforeExec"`
	Docker         config.Docker      `yaml:"docker,omitempty" json:"docker"`
	Npm            config.Npm         `yaml:"npm,omitempty" json:"npm"`
	RootDir        string             `yaml:"rootDir,omitempty" json:"rootDir"`
	RunnerVersion  string             `yaml:"runnerVersion,omitempty" json:"runnerVersion"`
}

// Playwright represents crucial playwright configuration that is required for setting up a project.
type Playwright struct {
	// Deprecated. ProjectPath is succeeded by Project.RootDir.
	ProjectPath string      `yaml:"projectPath,omitempty" json:"projectPath,omitempty"`
	Version     string      `yaml:"version,omitempty" json:"version,omitempty"`
	Params      SuiteConfig `yaml:"params,omitempty" json:"params,omitempty"`
}

// Suite represents the playwright test suite configuration.
type Suite struct {
	Name              string            `yaml:"name,omitempty" json:"name"`
	PlaywrightVersion string            `yaml:"playwrightVersion,omitempty" json:"playwrightVersion,omitempty"`
	TestMatch         string            `yaml:"testMatch,omitempty" json:"testMatch,omitempty"`
	PlatformName      string            `yaml:"platformName,omitempty" json:"platformName,omitempty"`
	Params            SuiteConfig       `yaml:"params,omitempty" json:"param,omitempty"`
	ScreenResolution  string            `yaml:"screenResolution,omitempty" json:"screenResolution,omitempty"`
	Env               map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
}

// SuiteConfig represents the configuration specific to a suite
type SuiteConfig struct {
	BrowserName         string `yaml:"browserName,omitempty" json:"browserName,omitempty"`
	HeadFul             bool   `yaml:"headful,omitempty" json:"headful,omitempty"`
	ScreenshotOnFailure bool   `yaml:"screenshotOnFailure,omitempty" json:"screenshotOnFailure,omitempty"`
	SlowMo              int    `yaml:"slowMo,omitempty" json:"slowMo,omitempty"`
	Video               bool   `yaml:"video,omitempty" json:"video,omitempty"`
}

// FromFile creates a new playwright Project based on the filepath cfgPath.
func FromFile(cfgPath string) (Project, error) {
	var p Project

	f, err := os.Open(cfgPath)
	defer f.Close()
	if err != nil {
		return Project{}, fmt.Errorf("failed to locate project config: %v", err)
	}

	if err = yaml.NewDecoder(f).Decode(&p); err != nil {
		return Project{}, fmt.Errorf("failed to parse project config: %v", err)
	}

	p.Playwright.Version = config.StandardizeVersionFormat(p.Playwright.Version)

	if p.Playwright.Version == "" {
		return p, errors.New("missing framework version. Check available versions here: https://docs.staging.saucelabs.net/testrunner-toolkit#supported-frameworks-and-browsers")
	}

	// Default project path
	if p.Playwright.ProjectPath == "" && p.RootDir == "" {
		return Project{}, fmt.Errorf("could not find 'rootDir' in config yml, 'rootDir' must be set to specify project files")
	} else if p.Playwright.ProjectPath != "" && p.RootDir == "" {
		log.Warn().Msg("'playwright.projectPath' is deprecated. Use 'rootDir' instead.")
		p.RootDir = p.Playwright.ProjectPath
	} else if p.Playwright.ProjectPath != "" && p.RootDir != "" {
		log.Warn().Msgf(
			"Found both 'playwright.projectPath=%s' and 'rootDir=%s' in config. 'projectPath' is deprecated, so defaulting to rootDir '%s'",
			p.Playwright.ProjectPath, p.RootDir, p.RootDir,
		)
	}

	// Default mode to Mount
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
	}

	return p, nil
}
