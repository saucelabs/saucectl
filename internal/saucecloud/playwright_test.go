package saucecloud

import (
	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPlaywright_GetSuiteNames(t *testing.T) {
	runner := &PlaywrightRunner{
		Project: playwright.Project{
			Suites: []playwright.Suite{
				{Name: "suite1"},
				{Name: "suite2"},
				{Name: "suite3"},
			},
		},
	}

	assert.Equal(t, "suite1, suite2, suite3", runner.getSuiteNames())
}
