package cypress

import (
	"errors"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/cypress/suite"
	v1 "github.com/saucelabs/saucectl/internal/cypress/v1"
	"github.com/saucelabs/saucectl/internal/saucereport"
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
	GetBeforeExec() []string
	GetReporters() config.Reporters
	GetNotifications() config.Notifications
	GetNpm() config.Npm
	SetNpmStrictSSL(strictSSL *bool)
	SetCLIFlags(map[string]interface{})
	GetSuites() []suite.Suite
	GetKind() string
	SetRunnerVersion(string)
	GetSuite() suite.Suite
	GetAPIVersion() string
	GetSmartRetry(suiteName string) config.SmartRetry
	FilterFailedTests(suiteName string, report saucereport.SauceReport) error
	IsSmartRetried() bool
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
	if version == "v1alpha" {
		return nil, errors.New("cypress v1alpha is no longer supported")
	}
	return v1.FromFile(cfgPath)
}
