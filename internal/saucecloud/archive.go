package saucecloud

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/archive/zip"
	"github.com/saucelabs/saucectl/internal/sauceignore"
)

// cache represents a store that can be used to cache return values of functions.
type cache struct {
	store map[string]string
}

func newCache() cache {
	return cache{
		store: make(map[string]string),
	}
}

// lookup attempts to find the value for a key in the cache and returns if there's a hit, otherwise it executes the closure fn and returns its results.
func (c cache) lookup(key string, fn func() (string, error)) (string, error) {
	var err error
	val, ok := c.store[key]
	if !ok {
		val, err = fn()
		if err == nil {
			c.store[key] = val
		}
	}
	return val, err
}

func archive(src string, targetDir string, archiveType archiveType) (string, error) {
	switch archiveType {
	case ipaArchive:
		return archiveAppToIpa(src, targetDir)
	case zipArchive:
		return archiveAppToZip(src, targetDir)
	case xctestArchive:
		return src, nil
	}
	return "", fmt.Errorf("unknown archive type: %s", archiveType)
}

func archiveAppToZip(appPath string, targetDir string) (string, error) {
	if strings.HasSuffix(appPath, ".zip") {
		return appPath, nil
	}

	log.Info().Msgf("Archiving %s to .zip", path.Base(appPath))

	fileName := fmt.Sprintf("%s.zip", strings.TrimSuffix(path.Base(appPath), ".app"))
	zipName := filepath.Join(targetDir, fileName)
	arch, err := zip.NewFileWriter(zipName, sauceignore.NewMatcher([]sauceignore.Pattern{}))
	if err != nil {
		return "", err
	}
	defer arch.Close()

	_, _, err = arch.Add(appPath, "")
	if err != nil {
		return "", err
	}
	return zipName, nil
}

// archiveAppToIpa generates a valid IPA file from a .app folder.
func archiveAppToIpa(appPath string, targetDir string) (string, error) {
	if strings.HasSuffix(appPath, ".ipa") {
		return appPath, nil
	}

	log.Info().Msgf("Archiving %s to .ipa", path.Base(appPath))

	fileName := fmt.Sprintf("%s.ipa", strings.TrimSuffix(path.Base(appPath), ".app"))
	zipName := filepath.Join(targetDir, fileName)
	arch, err := zip.NewFileWriter(zipName, sauceignore.NewMatcher([]sauceignore.Pattern{}))
	if err != nil {
		return "", err
	}
	defer arch.Close()

	_, _, err = arch.Add(appPath, "Payload/")
	if err != nil {
		return "", err
	}
	return zipName, nil
}
