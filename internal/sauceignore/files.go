package sauceignore

import (
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

// ExcludeSauceIgnorePatterns excludes any match with sauceignore content.
// If loading and parsing the sauceignore content fails, no filtering is applied.
func ExcludeSauceIgnorePatterns(files []string, sauceignoreFile string) []string {
	matcher, err := NewMatcherFromFile(sauceignoreFile)
	if err != nil {
		log.Warn().Err(err).Msgf(
			"An error occurred when filtering specs with %s. No filter will be applied",
			sauceignoreFile,
		)
		return files
	}

	var selectedFiles []string
	for _, filename := range files {
		normalized := filepath.ToSlash(filename)
		if !matcher.Match(strings.Split(normalized, "/"), false) {
			selectedFiles = append(selectedFiles, filename)
		}
	}
	return selectedFiles
}
