package author

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/saucelabs/saucectl/internal/authoring"
	syaml "github.com/saucelabs/saucectl/internal/yaml"
	"github.com/spf13/cobra"
)

// runConfig mirrors the shape internal/authoringrun.Project expects to read
// back via `saucectl run -c <path>`. It's a separate, minimal struct (rather
// than reusing authoringrun.Project directly) so this package doesn't need
// to depend on internal/config or internal/authoringrun just to serialize a
// few fields.
type runConfig struct {
	APIVersion string           `yaml:"apiVersion"`
	Kind       string           `yaml:"kind"`
	Sauce      runConfigSauce   `yaml:"sauce"`
	Suites     []runConfigSuite `yaml:"suites"`
}

type runConfigSauce struct {
	Region      string `yaml:"region"`
	Concurrency int    `yaml:"concurrency"`
}

type runConfigSuite struct {
	Name        string `yaml:"name"`
	TestSuiteID string `yaml:"testSuiteId"`
}

// maybeWriteRunConfig writes/updates the --config-out file (if set) from
// every suite currently referenced in the lockfile, so it naturally
// aggregates across multiple `author`/`author sync` invocations that target
// different suites over time, rather than only reflecting whichever suite
// this particular invocation touched.
func maybeWriteRunConfig(cmd *cobra.Command, flags *sharedFlags, lf authoring.Lockfile) error {
	if flags.configOut == "" {
		return nil
	}

	region, err := cmd.Flags().GetString("region")
	if err != nil || region == "" {
		region = "us-west-1"
	}

	seen := map[string]string{} // testSuiteId -> name
	for _, entry := range lf.Entries {
		if entry.TestSuiteID == "" {
			continue
		}
		seen[entry.TestSuiteID] = entry.TestSuiteName
	}

	suites := make([]runConfigSuite, 0, len(seen))
	for id, name := range seen {
		suites = append(suites, runConfigSuite{Name: name, TestSuiteID: id})
	}
	sort.Slice(suites, func(i, j int) bool { return suites[i].Name < suites[j].Name })

	cfg := runConfig{
		APIVersion: "v1alpha",
		Kind:       "authoring",
		Sauce: runConfigSauce{
			Region:      region,
			Concurrency: flags.concurrency,
		},
		Suites: suites,
	}

	if dir := filepath.Dir(flags.configOut); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", flags.configOut, err)
		}
	}

	if err := syaml.WriteFile(flags.configOut, cfg, 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", flags.configOut, err)
	}

	return nil
}
