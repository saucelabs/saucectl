package author

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/authoring"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

func newTestCmd(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("region", "us-west-1", "")
	return cmd
}

func TestMaybeWriteRunConfig_NoopWhenDisabled(t *testing.T) {
	cmd := newTestCmd(t)
	flags := &sharedFlags{configOut: ""}

	err := maybeWriteRunConfig(cmd, flags, authoring.Lockfile{})
	assert.NoError(t, err)
}

func TestMaybeWriteRunConfig_AggregatesAllSuitesInLockfile(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, ".sauce", "config.yml")

	cmd := newTestCmd(t)
	flags := &sharedFlags{configOut: out, concurrency: 5}

	lf := authoring.Lockfile{
		Entries: map[string]authoring.Entry{
			"specs/login.md": {
				TestCaseID:    "tc-1",
				TestSuiteID:   "suite-a",
				TestSuiteName: "regression",
				SyncedAt:      time.Now(),
			},
			"specs/checkout.md": {
				TestCaseID:    "tc-2",
				TestSuiteID:   "suite-a",
				TestSuiteName: "regression",
				SyncedAt:      time.Now(),
			},
			"specs/smoke/home.md": {
				TestCaseID:    "tc-3",
				TestSuiteID:   "suite-b",
				TestSuiteName: "smoke",
				SyncedAt:      time.Now(),
			},
		},
	}

	assert.NoError(t, maybeWriteRunConfig(cmd, flags, lf))

	b, err := os.ReadFile(out)
	assert.NoError(t, err)

	var got runConfig
	assert.NoError(t, yaml.Unmarshal(b, &got))

	assert.Equal(t, "v1alpha", got.APIVersion)
	assert.Equal(t, "authoring", got.Kind)
	assert.Equal(t, "us-west-1", got.Sauce.Region)
	assert.Equal(t, 5, got.Sauce.Concurrency)
	assert.Len(t, got.Suites, 2, "duplicate suite IDs across multiple specs should collapse into one entry")

	names := map[string]string{}
	for _, s := range got.Suites {
		names[s.TestSuiteID] = s.Name
	}
	assert.Equal(t, "regression", names["suite-a"])
	assert.Equal(t, "smoke", names["suite-b"])
}
