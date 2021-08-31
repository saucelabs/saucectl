package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStandardizeVersionFormat(t *testing.T) {
	assert.Equal(t, "5.6.0", StandardizeVersionFormat("v5.6.0"))
	assert.Equal(t, "5.6.0", StandardizeVersionFormat("5.6.0"))
}

func TestCleanNpmPackages(t *testing.T) {
	packages := map[string]string{
		"cypress": "7.7.0",
		"mocha":   "1.2.3",
	}

	cleaned := CleanNpmPackages(packages, []string{"cypress"})
	assert.NotContains(t, cleaned, "cypress")
	assert.Contains(t, cleaned, "mocha")

	packages = map[string]string{}

	cleaned = CleanNpmPackages(packages, []string{"somepackage"})
	assert.NotNil(t, cleaned)
	assert.Len(t, cleaned, 0)

	packages = nil
	cleaned = CleanNpmPackages(packages, []string{})
	assert.Nil(t, cleaned)
}
