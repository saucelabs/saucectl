package playwright

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/config"
	"gopkg.in/yaml.v2"
)

// Project represents the playwright project configuration.
type Project struct {
	config.TypeDef `yaml:",inline"`
	ShowConsoleLog bool
	ConfigFilePath string             `yaml:"-" json:"-"`
	DryRun         bool               `yaml:"-" json:"-"`
	Sauce          config.SauceConfig `yaml:"sauce,omitempty" json:"sauce"`
	Playwright     Playwright         `yaml:"playwright,omitempty" json:"playwright"`
	Suites         []Suite            `yaml:"suites,omitempty" json:"suites"`
	BeforeExec     []string           `yaml:"beforeExec,omitempty" json:"beforeExec"`
	Docker         config.Docker      `yaml:"docker,omitempty" json:"docker"`
	Npm            config.Npm         `yaml:"npm,omitempty" json:"npm"`
	RootDir        string             `yaml:"rootDir,omitempty" json:"rootDir"`
	RunnerVersion  string             `yaml:"runnerVersion,omitempty" json:"runnerVersion"`
	Artifacts      config.Artifacts   `yaml:"artifacts,omitempty" json:"artifacts"`
	Defaults       config.Defaults    `yaml:"defaults,omitempty" json:"defaults"`
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
	Mode              string            `yaml:"mode,omitempty" json:"-"`
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
	if err != nil {
		return Project{}, fmt.Errorf("failed to locate project config: %v", err)
	}
	defer f.Close()

	if err := yaml.NewDecoder(f).Decode(&p); err != nil {
		return Project{}, fmt.Errorf("failed to parse project config: %v", err)
	}

	if err := checkSupportedBrowsers(&p); err != nil {
		return Project{}, err
	}

	p.ConfigFilePath = cfgPath

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

		if s.PlatformName == "" {
			s.PlatformName = "Windows 10"
		}
	}

	return p, nil
}

// SplitSuites divided Suites to dockerSuites and sauceSuites
func SplitSuites(p Project) (Project, Project) {
	var dockerSuites []Suite
	var sauceSuites []Suite
	for _, s := range p.Suites {
		if s.Mode == "docker" {
			dockerSuites = append(dockerSuites, s)
		} else {
			sauceSuites = append(sauceSuites, s)
		}
	}

	dockerProject := p
	dockerProject.Suites = dockerSuites
	sauceProject := p
	sauceProject.Suites = sauceSuites

	return dockerProject, sauceProject
}

func checkSupportedBrowsers(p *Project) error {
	supportedBrowsers := map[string]struct{}{
		"chromium": struct{}{},
		"firefox":  struct{}{},
		"webkit":   struct{}{},
	}
	supportedBrwsList := []string{"chromium", "firefox", "webkit"}
	errMsg := "browserName: %s is not supported. List of supported browsers: %s"

	if p.Playwright.Params.BrowserName != "" {
		if _, ok := supportedBrowsers[p.Playwright.Params.BrowserName]; !ok {
			return fmt.Errorf(errMsg, p.Playwright.Params.BrowserName, strings.Join(supportedBrwsList, ", "))
		}
	}

	for _, suite := range p.Suites {
		if suite.Params.BrowserName != "" {
			if _, ok := supportedBrowsers[suite.Params.BrowserName]; !ok {
				return fmt.Errorf(errMsg, suite.Params.BrowserName, strings.Join(supportedBrwsList, ", "))
			}
		}
	}

	return nil
}
