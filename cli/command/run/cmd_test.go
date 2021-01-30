package run

import (
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/playwright"
	"path/filepath"
	"testing"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/runner"
	"github.com/saucelabs/saucectl/internal/region"

	"github.com/stretchr/testify/assert"
	"gotest.tools/v3/fs"
)

func TestNewRunCommand(t *testing.T) {
	testCases := []struct {
		name           string
		filter         string
		configFileName string
		configFile     string
		expErr         bool
		expResult      int
	}{
		{
			name:           "it can run successfully",
			configFileName: `config.yaml`,
			configFile:     "apiVersion: 1.2\nimage:\n  base: test",
			expResult:      123,
		},
		{
			name:           "it failed to parse config",
			configFileName: `config.yaml`,
			configFile:     "===",
			expErr:         true,
			expResult:      1,
		},
		{
			name:           "it doesn't filter suite when not required",
			configFileName: `config.yaml`,
			configFile:     "apiVersion: 1.2\nimage:\n  base: test",
			expResult:      123,
		},
		{
			name:           "it can filter out suite name",
			filter:         "filtersuite",
			configFileName: `config.yaml`,
			configFile:     "apiVersion: 1.2\nsuites:\n  - name: filtersuite\n  - name: suite2",
			expResult:      0,
		},
		{
			name:           "it failed with non-existed suite name",
			filter:         "non_existed_name",
			configFileName: `config.yaml`,
			configFile:     "apiVersion: 1.2\nsuites:\n  - name: filtersuite\n  - name: suite2",
			expErr:         true,
			expResult:      1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := fs.NewDir(t, "fixtures",
				fs.WithFile(tc.configFileName, tc.configFile, fs.WithMode(0755)))
			cli := command.SauceCtlCli{}
			cmd := Command(&cli)
			assert.Equal(t, cmd.Use, runUse)
			runner.ConfigPath = filepath.Join(dir.Path(), tc.configFileName)
			if err := cmd.Flags().Set("config", filepath.Join(dir.Path(), tc.configFileName)); err != nil {
				t.Fatal(err)
			}
			suiteName = tc.filter
			if tc.filter != "" {
				cmd.Flags().Lookup("suite").Changed = true
			}
			var args []string
			code, err := Run(cmd, &cli, args)
			if err != nil {
				assert.True(t, tc.expErr)
			} else {
				assert.False(t, tc.expErr)
				assert.Equal(t, tc.expResult, code)
			}
			t.Cleanup(func() {
				suiteName = ""
				runner.ConfigPath = "/home/seluser/config.yaml"
			})
		})
	}
}

func Test_apiBaseURL(t *testing.T) {
	type args struct {
		r region.Region
	}
	tests := []struct {
		name     string
		args     args
		sauceAPI string
		want     string
	}{
		{
			name:     "region based",
			args:     args{r: region.EUCentral1},
			sauceAPI: "",
			want:     region.EUCentral1.APIBaseURL(),
		},
		{
			name:     "override",
			args:     args{r: region.USWest1},
			sauceAPI: "https://nowhere.saucelabs.com",
			want:     "https://nowhere.saucelabs.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sauceAPI = tt.sauceAPI
			if got := apiBaseURL(tt.args.r); got != tt.want {
				t.Errorf("apiBaseURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplyDefaultValues(t *testing.T) {
	tests := []struct {
		beginRegion string
		wantRegion  string
	}{
		{beginRegion: "", wantRegion: defaultRegion},
		{beginRegion: "dummy-region", wantRegion: "dummy-region"},
	}
	for _, tt := range tests {
		sauce := config.SauceConfig{
			Region: tt.beginRegion,
		}
		applyDefaultValues(&sauce)
		assert.Equal(t, tt.wantRegion, sauce.Region)
	}
}

func TestFilterCypressSuite(t *testing.T) {
	s1 := cypress.Suite{Name: "suite1"}
	s2 := cypress.Suite{Name: "suite2"}
	s3 := cypress.Suite{Name: "suite3"}
	s4 := cypress.Suite{Name: "suite4"}

	tests := []struct {
		filterName    string
		wantErr       bool
		expectedCount int
		expectUniq    string
	}{
		{filterName: "", wantErr: false, expectedCount: 4, expectUniq: ""},
		{filterName: "impossible", wantErr: true, expectedCount: 0, expectUniq: ""},
		{filterName: "suite1", wantErr: false, expectedCount: 1, expectUniq: "suite1"},
		{filterName: "suite4", wantErr: false, expectedCount: 1, expectUniq: "suite4"},
	}

	for _, tt := range tests {
		p := &cypress.Project{
			Suites: []cypress.Suite{s1, s2, s3, s4},
		}
		suiteName = tt.filterName
		err := filterCypressSuite(p)
		if tt.wantErr {
			assert.NotNil(t, err, "error not received")
			continue
		}
		assert.Equal(t, tt.expectedCount, len(p.Suites), "suite count mismatch")
		if tt.expectUniq != "" {
			assert.Equal(t, tt.expectUniq, p.Suites[0].Name, "suite name mismatch")
		}
	}
}

func TestFilterPlaywrightSuite(t *testing.T) {
	s1 := playwright.Suite{Name: "suite1"}
	s2 := playwright.Suite{Name: "suite2"}
	s3 := playwright.Suite{Name: "suite3"}
	s4 := playwright.Suite{Name: "suite4"}

	tests := []struct {
		filterName    string
		wantErr       bool
		expectedCount int
		expectUniq    string
	}{
		{filterName: "", wantErr: false, expectedCount: 4, expectUniq: ""},
		{filterName: "impossible", wantErr: true, expectedCount: 0, expectUniq: ""},
		{filterName: "suite1", wantErr: false, expectedCount: 1, expectUniq: "suite1"},
		{filterName: "suite4", wantErr: false, expectedCount: 1, expectUniq: "suite4"},
	}

	for _, tt := range tests {
		p := &playwright.Project{
			Suites: []playwright.Suite{s1, s2, s3, s4},
		}
		suiteName = tt.filterName
		err := filterPlaywrightSuite(p)
		if tt.wantErr {
			assert.NotNil(t, err, "error not received")
			continue
		}
		assert.Equal(t, tt.expectedCount, len(p.Suites), "suite count mismatch")
		if tt.expectUniq != "" {
			assert.Equal(t, tt.expectUniq, p.Suites[0].Name, "suite name mismatch")
		}
	}
}

func TestCreateCIProvider(t *testing.T) {
	enableCIProviders()
}
