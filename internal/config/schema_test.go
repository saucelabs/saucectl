package config

import (
	"os"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// compileBundledSchema compiles the committed, generated api/saucectl.schema.json
// (the same artifact ValidateSchema fetches at runtime) directly from disk so the
// schema's semantics can be asserted offline.
func compileBundledSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	f, err := os.Open("../../api/saucectl.schema.json")
	require.NoError(t, err)
	defer f.Close()

	c := jsonschema.NewCompiler()
	require.NoError(t, c.AddResource("saucectl.schema.json", f))
	schema, err := c.Compile("saucectl.schema.json")
	require.NoError(t, err)
	return schema
}

// hasPlatformNameError reports whether the validation error tree contains a
// failure rooted at a suite's platformName (i.e. the platform enum rejected the value).
func hasPlatformNameError(err error) bool {
	ve, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return false
	}
	var walk func(e *jsonschema.ValidationError) bool
	walk = func(e *jsonschema.ValidationError) bool {
		if len(e.Causes) == 0 {
			return e.InstanceLocation == "/suites/0/platformName"
		}
		for _, cause := range e.Causes {
			if walk(cause) {
				return true
			}
		}
		return false
	}
	return walk(ve)
}

func playwrightConfigWithPlatform(platform string) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "v1alpha",
		"kind":       "playwright",
		"sauce":      map[string]interface{}{"region": "us-west-1"},
		"playwright": map[string]interface{}{"version": "1.58.1"},
		"suites": []interface{}{
			map[string]interface{}{
				"name":         "suite",
				"platformName": platform,
				"testMatch":    []interface{}{".*"},
				"params":       map[string]interface{}{},
			},
		},
	}
}

// TestSchema_PlaywrightPlatformName guards INT-574: the bundled schema must accept
// the macOS versions listed in playwright.schema.json's platformName enum (14/15/26),
// while still rejecting unknown platforms. This regresses if the bundler reverts from
// dereference() to bundle() (which intersects the per-framework enum with a stale
// shared $ref and silently drops 14/15/26).
func TestSchema_PlaywrightPlatformName(t *testing.T) {
	schema := compileBundledSchema(t)

	cases := []struct {
		platform string
		valid    bool
	}{
		{"macOS 12", true},
		{"macOS 13", true},
		{"macOS 14", true},
		{"macOS 15", true},
		{"macOS 26", true},
		{"Windows 10", true},
		{"Windows 11", true},
		{"macOS 99", false},
		{"definitely-not-a-platform", false},
	}

	for _, tc := range cases {
		t.Run(tc.platform, func(t *testing.T) {
			err := schema.Validate(playwrightConfigWithPlatform(tc.platform))
			if tc.valid {
				assert.Falsef(t, hasPlatformNameError(err),
					"platformName %q should be accepted by the schema, got: %v", tc.platform, err)
			} else {
				assert.Truef(t, hasPlatformNameError(err),
					"platformName %q should be rejected by the schema", tc.platform)
			}
		})
	}
}
