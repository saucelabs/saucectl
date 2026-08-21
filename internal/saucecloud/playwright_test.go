package saucecloud

import (
	"testing"

	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/stretchr/testify/assert"
)

func TestResolveBrowserVersion(t *testing.T) {
	meta := framework.Metadata{
		Platforms: []framework.Platform{
			{
				PlatformName: "macos 13",
				BrowserNames: []string{"playwright-chromium", "playwright-firefox", "playwright-webkit"},
				BrowserDefaults: map[string]string{
					"playwright-webkit": "18.4",
				},
			},
			{
				PlatformName: "macos 15",
				BrowserNames: []string{"playwright-chromium", "playwright-firefox", "playwright-webkit"},
			},
		},
		BrowserDefaults: map[string]string{
			"playwright-chromium": "145.0.7632.6.",
			"playwright-firefox":  "146.0.1",
			"playwright-webkit":   "26.0.",
		},
	}

	r := &PlaywrightRunner{Project: &playwright.Project{}}

	tests := []struct {
		name       string
		platform   string
		browserKey string
		expected   string
	}{
		{
			name:       "per-platform override for webkit on macos 13",
			platform:   "macos 13",
			browserKey: "playwright-webkit",
			expected:   "18.4",
		},
		{
			name:       "fallback to top-level for webkit on macos 15 (no per-platform default)",
			platform:   "macos 15",
			browserKey: "playwright-webkit",
			expected:   "26.0.",
		},
		{
			name:       "fallback to top-level for chromium on macos 13 (not overridden)",
			platform:   "macos 13",
			browserKey: "playwright-chromium",
			expected:   "145.0.7632.6.",
		},
		{
			name:       "case-insensitive platform match",
			platform:   "Macos 13",
			browserKey: "playwright-webkit",
			expected:   "18.4",
		},
		{
			name:       "empty platform falls back to top-level",
			platform:   "",
			browserKey: "playwright-webkit",
			expected:   "26.0.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.resolveBrowserVersion(meta, tt.platform, tt.browserKey)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPlaywright_GetSuiteNames(t *testing.T) {
	runner := &PlaywrightRunner{
		Project: &playwright.Project{
			Suites: []playwright.Suite{
				{Name: "suite1"},
				{Name: "suite2"},
				{Name: "suite3"},
			},
		},
	}

	assert.Equal(t, []string{"suite1", "suite2", "suite3"}, runner.getSuiteNames())
}
