package authoring

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoadLockfile_MissingFileIsNotAnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.yml")

	lf, err := LoadLockfile(path)
	assert.NoError(t, err)
	assert.Equal(t, 1, lf.Version)
	assert.NotNil(t, lf.Entries)
	assert.Empty(t, lf.Entries)
}

func TestLockfile_SaveAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".sauce", "authoring-lock.yml")

	lf := Lockfile{
		Version: 1,
		Entries: map[string]Entry{
			"specs/login.md": {
				Hash:          "abc123",
				TestCaseID:    "64f0a1b2c3d4e5f6a7b8c9d0",
				TestSuiteID:   "11111111-1111-1111-1111-111111111111",
				TestSuiteName: "regression",
				SyncedAt:      time.Now().UTC().Round(time.Second),
			},
		},
	}

	assert.NoError(t, lf.Save(path))

	// Confirm the parent directory was created and the file exists.
	_, err := os.Stat(path)
	assert.NoError(t, err)

	loaded, err := LoadLockfile(path)
	assert.NoError(t, err)
	assert.Equal(t, lf.Version, loaded.Version)
	assert.Equal(t, lf.Entries["specs/login.md"].Hash, loaded.Entries["specs/login.md"].Hash)
	assert.Equal(t, lf.Entries["specs/login.md"].TestCaseID, loaded.Entries["specs/login.md"].TestCaseID)
	assert.Equal(t, lf.Entries["specs/login.md"].TestSuiteID, loaded.Entries["specs/login.md"].TestSuiteID)
}

func TestLockfile_Changed(t *testing.T) {
	lf := Lockfile{
		Entries: map[string]Entry{
			"specs/login.md": {Hash: "abc123"},
		},
	}

	tests := []struct {
		name        string
		specPath    string
		currentHash string
		want        bool
	}{
		{"unknown spec is always changed", "specs/new.md", "whatever", true},
		{"unchanged hash is not changed", "specs/login.md", "abc123", false},
		{"different hash is changed", "specs/login.md", "def456", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, lf.Changed(tt.specPath, tt.currentHash))
		})
	}
}

func TestHashSpec_DetectsContentChanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.md")

	assert.NoError(t, os.WriteFile(path, []byte("go to the login page and sign in"), 0o644))
	h1, err := HashSpec(path)
	assert.NoError(t, err)
	assert.NotEmpty(t, h1)

	// Same content -> same hash.
	h2, err := HashSpec(path)
	assert.NoError(t, err)
	assert.Equal(t, h1, h2)

	// Changed content -> different hash.
	assert.NoError(t, os.WriteFile(path, []byte("go to the login page and sign in with 2FA"), 0o644))
	h3, err := HashSpec(path)
	assert.NoError(t, err)
	assert.NotEqual(t, h1, h3)
}
