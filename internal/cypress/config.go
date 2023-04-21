package cypress

import (
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/cypress/suite"
	v1 "github.com/saucelabs/saucectl/internal/cypress/v1"
	"github.com/saucelabs/saucectl/internal/cypress/v1alpha"
)

// Config descriptors.
var (
	// Kind represents the type definition of this config.
	Kind = "cypress"
)

type Project interface {
	FilterSuites(suiteName string) error
	CleanPackages()
	ApplyFlags(selectedSuite string) error
	AppendTags([]string)
	Validate() error
	IsSharded() bool

	SetDefaults()
	GetSuiteCount() int
	GetVersion() string
	GetRunnerVersion() string
	SetVersion(version string)
	GetSauceCfg() config.SauceConfig
	IsDryRun() bool
	GetRootDir() string
	GetSuiteNames() string
	GetCfgPath() string
	GetCLIFlags() map[string]interface{}
	GetArtifactsCfg() config.Artifacts
	IsShowConsoleLog() bool
	GetDocker() config.Docker
	GetBeforeExec() []string
	GetReporter() config.Reporters
	GetNotifications() config.Notifications
	GetNpm() config.Npm
	SetCLIFlags(map[string]interface{})
	GetSuites() []suite.Suite
	GetKind() string
	SetRunnerVersion(string)
	GetSuite() suite.Suite
	GetAPIVersion() string
	GetSmartRetry(suiteName string) config.SmartRetry
	SetTestGrep(suiteIndex int, tests []string)
}

type project struct {
	config.TypeDef `yaml:",inline" mapstructure:",squash"`
}

func getVersion(cfgPath string) (string, error) {
	var c project
	if err := config.Unmarshal(cfgPath, &c); err != nil {
		return "", err
	}
	return c.APIVersion, nil
}

func FromFile(cfgPath string) (Project, error) {
	version, err := getVersion(cfgPath)
	if err != nil {
		return nil, err
	}
	if version == v1alpha.APIVersion {
		return v1alpha.FromFile(cfgPath)
	}
	return v1.FromFile(cfgPath)
}

// SplitSuites divided Suites to dockerSuites and sauceSuites
func SplitSuites(project Project) (Project, Project) {
	if project.GetAPIVersion() == v1alpha.APIVersion {
		var dockerSuites []v1alpha.Suite
		var sauceSuites []v1alpha.Suite
		p := project.(*v1alpha.Project)
		for _, s := range p.Suites {
			if s.Mode == "docker" || (s.Mode == "" && p.Defaults.Mode == "docker") {
				dockerSuites = append(dockerSuites, s)
			} else {
				sauceSuites = append(sauceSuites, s)
			}
		}

		dockerProject := *p
		dockerProject.Suites = dockerSuites
		sauceProject := *p
		sauceProject.Suites = sauceSuites

		return &dockerProject, &sauceProject
	}

	var dockerSuites []v1.Suite
	var sauceSuites []v1.Suite
	p := project.(*v1.Project)
	for _, s := range p.Suites {
		if s.Mode == "docker" || (s.Mode == "" && p.Defaults.Mode == "docker") {
			dockerSuites = append(dockerSuites, s)
		} else {
			sauceSuites = append(sauceSuites, s)
		}
	}

	dockerProject := *p
	dockerProject.Suites = dockerSuites
	sauceProject := *p
	sauceProject.Suites = sauceSuites

	return &dockerProject, &sauceProject
}
