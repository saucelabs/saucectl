package run

import (
	"github.com/saucelabs/saucectl/internal/espresso"
	"reflect"
	"testing"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/testcafe"
	"github.com/stretchr/testify/assert"
)

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
			gFlags.sauceAPI = tt.sauceAPI
			if got := apiBaseURL(tt.args.r); got != tt.want {
				t.Errorf("apiBaseURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplyDefaultValues(t *testing.T) {
	type args struct {
		region      string
		sauceignore string
	}
	tests := []struct {
		args            args
		wantRegion      string
		wantSauceignore string
	}{
		{args: args{region: "", sauceignore: ""}, wantRegion: defaultRegion, wantSauceignore: defaultSauceignore},
		{args: args{region: "dummy-region", sauceignore: "/path/to/.sauceignore2"}, wantRegion: "dummy-region",
			wantSauceignore: "/path/to/.sauceignore2"},
	}
	for _, tt := range tests {
		sauce := config.SauceConfig{
			Region:      tt.args.region,
			Sauceignore: tt.args.sauceignore,
		}
		applyDefaultValues(&sauce)
		assert.Equal(t, tt.wantRegion, sauce.Region)
		assert.Equal(t, tt.wantSauceignore, sauce.Sauceignore)
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
		gFlags.suiteName = tt.filterName
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
		gFlags.suiteName = tt.filterName
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

func TestFilterTestcafeSuite(t *testing.T) {
	testCase := []struct {
		name      string
		config    *testcafe.Project
		suiteName string
		expConfig testcafe.Project
		expErr    string
	}{
		{
			name: "filter out suite according to suiteName",
			config: &testcafe.Project{Suites: []testcafe.Suite{
				{
					Name: "suite1",
				},
				{
					Name: "suite2",
				},
			}},
			suiteName: "suite1",
			expConfig: testcafe.Project{Suites: []testcafe.Suite{
				{
					Name: "suite1",
				},
			}},
		},
		{
			name: "no required suite name in config",
			config: &testcafe.Project{Suites: []testcafe.Suite{
				{
					Name: "suite1",
				},
				{
					Name: "suite2",
				},
			}},
			suiteName: "suite3",
			expConfig: testcafe.Project{Suites: []testcafe.Suite{
				{
					Name: "suite1",
				},
				{
					Name: "suite2",
				},
			}},
			expErr: "suite name 'suite3' is invalid",
		},
	}

	for _, tc := range testCase {
		t.Run(tc.name, func(t *testing.T) {
			gFlags.suiteName = tc.suiteName
			err := filterTestcafeSuite(tc.config)
			if err != nil {
				assert.Equal(t, tc.expErr, err.Error())
			}
			assert.True(t, reflect.DeepEqual(*tc.config, tc.expConfig))
		})
	}
}

func TestFilterEspressoSuite(t *testing.T) {
	testCase := []struct {
		name      string
		config    *espresso.Project
		suiteName string
		expConfig espresso.Project
		expErr    string
	}{
		{
			name: "filter out suite according to suiteName",
			config: &espresso.Project{Suites: []espresso.Suite{
				{
					Name: "suite1",
				},
				{
					Name: "suite2",
				},
			}},
			suiteName: "suite1",
			expConfig: espresso.Project{Suites: []espresso.Suite{
				{
					Name: "suite1",
				},
			}},
		},
		{
			name: "no required suite name in config",
			config: &espresso.Project{Suites: []espresso.Suite{
				{
					Name: "suite1",
				},
				{
					Name: "suite2",
				},
			}},
			suiteName: "suite3",
			expConfig: espresso.Project{Suites: []espresso.Suite{
				{
					Name: "suite1",
				},
				{
					Name: "suite2",
				},
			}},
			expErr: "suite name 'suite3' is invalid",
		},
	}

	for _, tc := range testCase {
		t.Run(tc.name, func(t *testing.T) {
			gFlags.suiteName = tc.suiteName
			err := filterEspressoSuite(tc.config)
			if err != nil {
				assert.Equal(t, tc.expErr, err.Error())
			}
			assert.True(t, reflect.DeepEqual(*tc.config, tc.expConfig))
		})
	}
}

func TestValidateFiles(t *testing.T) {
	testCase := []struct {
		name   string
		files  []string
		expErr string
	}{
		{
			name: "files are all existed",
			files: []string{
				"cmd.go", "cmd_test.go",
			},
			expErr: "",
		},
		{
			name: "one of the files is not existed",
			files: []string{
				"cmd.go", "test_file_not_existed.go",
			},
			expErr: "stat test_file_not_existed.go: no such file or directory",
		},
	}

	for _, tc := range testCase {
		t.Run(tc.name, func(t *testing.T) {
			err := validateFiles(tc.files)
			if err != nil {
				assert.Equal(t, tc.expErr, err.Error())
			}
		})
	}
}
