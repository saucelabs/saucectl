package authoring

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/config"
)

func TestFromFile_PreservesCapabilityKeyCaseAndNesting(t *testing.T) {
	// Two things this locks in: the viper fork keeps key case (a stock viper
	// lowercases "browserName" to "browsername"), and nested maps decode to
	// something SetDefaults can turn into JSON-marshalable capabilities.
	dir := t.TempDir()
	cfg := filepath.Join(dir, "authoring.yml")
	yaml := `apiVersion: v1alpha
kind: authoring
sauce:
  region: us-west-1
  concurrency: 3
  metadata:
    build: "nightly"
defaults:
  timeout: 10m
suites:
  - name: "Regression on Chrome"
    testSuiteName: "Checkout Regression"
    targets:
      - capabilities:
          browserName: chrome
          platformName: "Windows 11"
          sauce:options:
            name: x
            screenResolution: 1920x1080
  - name: "Two cases"
    testCases: [6a9ba94e5c93b6d220adc17d, 6a88b8ee7760d47c61f9e1fa]
    timeout: 2m
artifacts:
  download:
    when: fail
    match: ["*.mp4"]
    directory: ./artifacts
reporters:
  junit:
    enabled: true
`
	if err := os.WriteFile(cfg, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := FromFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	SetDefaults(&p)
	if err := Validate(p); err != nil {
		t.Fatal(err)
	}

	if p.Kind != Kind || p.Sauce.Concurrency != 3 || p.Sauce.Metadata.Build != "nightly" {
		t.Errorf("project = %+v", p)
	}
	if p.Suites[0].Timeout != 10*time.Minute || p.Suites[1].Timeout != 2*time.Minute {
		t.Errorf("timeouts = %v, %v", p.Suites[0].Timeout, p.Suites[1].Timeout)
	}
	if !p.Reporters.JUnit.Enabled || p.Artifacts.Download.When != config.WhenFail {
		t.Errorf("reporters/artifacts not decoded: %+v %+v", p.Reporters, p.Artifacts)
	}

	caps := p.Suites[0].Targets[0].Capabilities
	if caps["browserName"] != "chrome" || caps["platformName"] != "Windows 11" {
		t.Errorf("capability key case not preserved: %v", caps)
	}
	opts, ok := caps["sauce:options"].(map[string]any)
	if !ok || opts["screenResolution"] != "1920x1080" {
		t.Errorf("nested capabilities not normalised: %#v", caps["sauce:options"])
	}
	if _, err := json.Marshal(caps); err != nil {
		t.Errorf("capabilities must marshal to JSON: %v", err)
	}
	if len(p.Suites[1].TestCases) != 2 {
		t.Errorf("testCases = %v", p.Suites[1].TestCases)
	}
}

func TestSetDefaults(t *testing.T) {
	p := Project{Suites: []Suite{{Name: "a", TestSuiteID: "x"}}}
	SetDefaults(&p)
	if p.Kind != Kind || p.APIVersion != APIVersion {
		t.Errorf("type def defaults: %+v", p.TypeDef)
	}
	if p.Sauce.Concurrency != defaultConcurrency {
		t.Errorf("concurrency = %d", p.Sauce.Concurrency)
	}
	if p.Sauce.Metadata.Build == "" {
		t.Error("build name must default")
	}
	if p.Suites[0].Timeout != DefaultSuiteTimeout {
		t.Errorf("suite timeout = %v, want %v", p.Suites[0].Timeout, DefaultSuiteTimeout)
	}

	long := Project{Sauce: config.SauceConfig{Metadata: config.Metadata{Build: string(make([]byte, 150))}}}
	SetDefaults(&long)
	if len(long.Sauce.Metadata.Build) != maxBuildNameLength {
		t.Errorf("build name not truncated to %d: %d", maxBuildNameLength, len(long.Sauce.Metadata.Build))
	}
}

func TestValidate(t *testing.T) {
	valid := func() Project {
		return Project{
			Sauce:  config.SauceConfig{Region: "us-west-1"},
			Suites: []Suite{{Name: "a", TestSuiteID: "x"}},
		}
	}

	tests := []struct {
		name    string
		mutate  func(*Project)
		wantErr bool
	}{
		{name: "valid", mutate: func(*Project) {}},
		{name: "missing region", mutate: func(p *Project) { p.Sauce.Region = "" }, wantErr: true},
		{name: "no suites", mutate: func(p *Project) { p.Suites = nil }, wantErr: true},
		{name: "suite without name", mutate: func(p *Project) { p.Suites[0].Name = "" }, wantErr: true},
		{name: "no reference", mutate: func(p *Project) { p.Suites[0].TestSuiteID = "" }, wantErr: true},
		{name: "two references", mutate: func(p *Project) { p.Suites[0].TestSuiteName = "n" }, wantErr: true},
		{name: "by name", mutate: func(p *Project) { p.Suites[0].TestSuiteID = ""; p.Suites[0].TestSuiteName = "n" }},
		{name: "by cases", mutate: func(p *Project) { p.Suites[0].TestSuiteID = ""; p.Suites[0].TestCases = []string{"c"} }},
		{name: "empty case id", mutate: func(p *Project) { p.Suites[0].TestSuiteID = ""; p.Suites[0].TestCases = []string{""} }, wantErr: true},
		{name: "target without capabilities", mutate: func(p *Project) { p.Suites[0].Targets = []Target{{}} }, wantErr: true},
		{name: "duplicate suite names", mutate: func(p *Project) { p.Suites = append(p.Suites, Suite{Name: "a", TestSuiteID: "y"}) }, wantErr: true},
		{name: "negative timeout", mutate: func(p *Project) { p.Suites[0].Timeout = -1 }, wantErr: true},
		{name: "unsupported settings only warn", mutate: func(p *Project) {
			p.Sauce.Retries = 2
			p.Sauce.Metadata.Tags = []string{"t"}
			p.Sauce.Tunnel.Owner = "o"
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := valid()
			tt.mutate(&p)
			err := Validate(p)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFilterSuites(t *testing.T) {
	p := Project{Suites: []Suite{{Name: "a"}, {Name: "b"}}}
	if err := FilterSuites(&p, "b"); err != nil || len(p.Suites) != 1 || p.Suites[0].Name != "b" {
		t.Errorf("filter: %v %+v", err, p.Suites)
	}
	if err := FilterSuites(&p, "zzz"); err == nil {
		t.Error("expected error for unknown suite")
	}
}

func TestNormalizeValue(t *testing.T) {
	in := map[string]any{
		"a":    map[any]any{"b": map[any]any{"c": 1}, 2: "two"},
		"list": []any{map[any]any{"d": true}},
	}
	out := normalizeCapabilities(in)
	if _, err := json.Marshal(out); err != nil {
		t.Fatalf("normalised capabilities must marshal: %v", err)
	}
	a := out["a"].(map[string]any)
	if a["2"] != "two" || a["b"].(map[string]any)["c"] != 1 {
		t.Errorf("nested conversion wrong: %#v", out)
	}
}
