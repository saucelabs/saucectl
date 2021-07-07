package run

import (
	"testing"

	"github.com/saucelabs/saucectl/internal/playwright"
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
