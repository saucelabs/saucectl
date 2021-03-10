package saucecloud

import (
	"testing"

	"github.com/saucelabs/saucectl/internal/testcafe"
	"github.com/stretchr/testify/assert"
)

func TestTestcafe_GetSuiteNames(t *testing.T) {
	runner := &TestcafeRunner{
		Project: testcafe.Project{
			Suites: []testcafe.Suite{
				{Name: "suite1"},
				{Name: "suite2"},
				{Name: "suite3"},
			},
		},
	}

	assert.Equal(t, "suite1, suite2, suite3", runner.getSuiteNames())
}
