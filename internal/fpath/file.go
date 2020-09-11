package fpath

import (
	"github.com/rs/zerolog/log"
	"os"
	"path/filepath"
	"regexp"
)

// Globs returns the names of all files matching the glob patterns.
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

// List returns a list of files matching the regexp pattern.
// Unlike filepath.Glob, this method will inspect all subdirectories of dir.
func List(dir string, pattern string) ([]string, error) {
	var ll []string

	r, err := regexp.Compile(pattern)
	if err != nil {
		return ll, err
	}

	err = filepath.Walk(dir, func(p string, i os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if r.MatchString(p) {
			ll = append(ll, p)
		}
		return nil
	})

	return ll, err
}
