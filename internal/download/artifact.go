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

// Cleanup removes previous downloaded artifacts
func Cleanup(directory string) {
	err := os.RemoveAll(directory)
	if err != nil {
		log.Err(err).Msg("Unable to cleanup previous artifacts")
	}
}

// GetDirName returns a target folder that's based on a combination of suiteName and the configured artifact download folder.
// The suiteName is sanitized by undergoing character replacements that are safe to be used as a directory name.
// If the determined target directory already exists, a running number is added as a suffix.
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
