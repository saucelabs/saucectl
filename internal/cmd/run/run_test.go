package run

import (
	"github.com/saucelabs/saucectl/internal/espresso"
	"reflect"
	"testing"

	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/saucelabs/saucectl/internal/testcafe"
	"github.com/stretchr/testify/assert"
)

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
