package download

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/config"
)

// ShouldDownloadArtifact returns true if it should download artifacts, otherwise false
func ShouldDownloadArtifact(jobID string, passed, timedOut, async bool, cfg config.ArtifactDownload) bool {
	if jobID == "" || timedOut || async {
		return false
	}
	if cfg.When == config.WhenAlways {
		return true
	}
	if cfg.When == config.WhenFail && !passed {
		return true
	}
	if cfg.When == config.WhenPass && passed {
		return true
	}

	return false
}

// Cleanup removes previous downloaded artifacts
func Cleanup(directory string) {
	err := os.RemoveAll(directory)
	if err != nil {
		log.Err(err).Msg("Unable to cleanup previous artifacts")
	}
}

// GetDirName returns a target folder for downloading artifacts.
// It creates a targetDir from suiteName by replacing / \ . and space with hyphen and underscore.
// If there isn't targetDir under artifacts directory, it returns targetDir. e.g. `chrome_test`
// If there is an existing targetDir, it finds the maxVersion from the suffix and adds one to that. e.g. `chrome_test.1`
func GetDirName(suiteName string, cfg config.ArtifactDownload) (string, error) {
	suiteName = strings.NewReplacer("/", "-", "\\", "-", ".", "-", " ", "_").Replace(suiteName)
	// If targetDir doesn't exist, no need to find maxVersion and return
	targetDir := filepath.Join(cfg.Directory, suiteName)
	if _, err := os.Open(targetDir); os.IsNotExist(err) {
		return targetDir, nil
	}
	// Find the maxVersion of downloaded artifacts in artifacts dir
	f, err := os.Open(cfg.Directory)
	if err != nil {
		return "", nil
	}
	files, err := f.ReadDir(0)
	if err != nil {
		return "", err
	}
	maxVersion := 0
	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		fileName := strings.Split(file.Name(), ".")
		if len(fileName) != 2 || fileName[0] != suiteName {
			continue
		}

		version, err := strconv.Atoi(fileName[1])
		if err != nil {
			return "", err
		}
		if version > maxVersion {
			maxVersion = version
		}
	}
	suiteName = fmt.Sprintf("%s.%d", suiteName, maxVersion+1)

	return filepath.Join(cfg.Directory, suiteName), nil
}
