package run

import (
	"testing"

	"github.com/docker/docker/pkg/testutil/assert"
	"gotest.tools/v3/fs"
)

func TestFailingCases(t *testing.T) {
	dir := fs.NewDir(t, "fixtures",
		fs.WithFile("invalid_config.yaml", "foo", fs.WithMode(0755)),
		fs.WithFile("valid_config.yaml", "apiVersion: 1.2", fs.WithMode(0755)))

	cases := []struct {
		Name       string
		Input      string
		ShouldPass bool
	}{
		{"With nil filename", "", false},
		{"With non existing config", "/dont/exist", false},
		{"With non invalid config", dir.Path() + "/invalid_config.yaml", false},
		{"With valid config", dir.Path() + "/valid_config.yaml", true},
	}

	var configFile Configuration
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			config, err := configFile.readFromFilePath(tc.Input)
			if !tc.ShouldPass {
				if err == nil {
					t.Error("No error was returned for failing test case")
				}

				return
			}

			if err != nil {
				t.Error("Error was returned for passing test case")
			}

			assert.Equal(t, config.APIVersion, "1.2")
		})
	}

	dir.Remove()
}
