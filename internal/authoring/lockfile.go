package authoring

import (
	"os"
	"path/filepath"
	"time"

	"github.com/saucelabs/saucectl/internal/hashio"
	syaml "github.com/saucelabs/saucectl/internal/yaml"
	"gopkg.in/yaml.v2"
)

// DefaultLockfileName is the conventional name for the authoring lockfile.
// It is generated and maintained exclusively by `saucectl author`; it is not
// meant to be hand-edited, but it is meant to be committed to version
// control so that CI and every contributor share the same view of which
// spec produced which test case.
const DefaultLockfileName = ".sauce/authoring-lock.yml"

// Entry records the last-synced state of a single spec file.
type Entry struct {
	// Hash is the content hash of the spec file as of the last successful
	// sync. Used to detect whether the spec has changed since.
	Hash string `yaml:"hash"`
	// TestCaseID is the Sauce Labs test case ID that was generated from this
	// spec.
	TestCaseID string `yaml:"testCaseId"`
	// TestSuiteID is the suite the test case was added to.
	TestSuiteID string `yaml:"testSuiteId"`
	// TestSuiteName is the human-readable name of TestSuiteID, kept
	// alongside the ID for readability/debuggability of the lockfile.
	TestSuiteName string `yaml:"testSuiteName"`
	// SyncedAt is when this entry was last written.
	SyncedAt time.Time `yaml:"syncedAt"`
}

// Lockfile is the on-disk representation of every spec that has been
// authored so far, keyed by the spec's path relative to the lockfile.
type Lockfile struct {
	// Version allows the format to evolve later without breaking existing
	// lockfiles.
	Version int `yaml:"version"`
	// Entries maps a spec file's relative path to its last-synced state.
	Entries map[string]Entry `yaml:"entries"`
}

// LoadLockfile reads the lockfile at path. A missing file is not an error --
// it's treated as an empty lockfile, since the very first run of `saucectl
// author` in a project won't have one yet.
func LoadLockfile(path string) (Lockfile, error) {
	lf := Lockfile{Version: 1, Entries: map[string]Entry{}}

	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return lf, nil
		}
		return lf, err
	}

	if err := yaml.Unmarshal(b, &lf); err != nil {
		return lf, err
	}

	if lf.Entries == nil {
		lf.Entries = map[string]Entry{}
	}
	if lf.Version == 0 {
		lf.Version = 1
	}

	return lf, nil
}

// Save writes the lockfile to path, creating parent directories as needed.
func (lf Lockfile) Save(path string) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return syaml.WriteFile(path, lf, 0o644)
}

// HashSpec computes the content hash used to detect whether a spec file has
// changed since it was last authored.
func HashSpec(path string) (string, error) {
	return hashio.HashContent(path)
}

// Changed reports whether the spec at specPath has no recorded entry, or its
// current content hash no longer matches the recorded one.
func (lf Lockfile) Changed(specPath, currentHash string) bool {
	entry, ok := lf.Entries[specPath]
	if !ok {
		return true
	}
	return entry.Hash != currentHash
}
