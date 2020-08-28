package fpath

import (
	"github.com/rs/zerolog/log"
	"path/filepath"
)

// Globs returns the names of all files matching the patterns.
// Effectively syntactic sugar for filepath.Glob() to support multiple patterns.
func Globs(patterns []string) []string {
	var files []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			log.Warn().Str("p", pattern).Msg("Skipping over malformed pattern. Some of your test files will be missing.")
			continue
		}

		files = append(files, matches...)
	}

	return files
}
