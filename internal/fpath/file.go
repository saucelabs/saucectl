package fpath

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/ryanuber/go-glob"
)

type MatchPattern string

const (
	FindByRegex        MatchPattern = "regex"
	FindByShellPattern MatchPattern = "shellpattern"
)

// FindFiles returns a list of files as identified by the sources. Source pattern interpretation (e.g. regex or glob) is controlled by matchBy.
func FindFiles(rootDir string, sources []string, matchBy MatchPattern) ([]string, error) {
	fmt.Println("rootDir, sources, matchby: ", rootDir, sources, matchBy)
	var files []string
	if err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		// Normalize path separators, since the target execution environment may not support backslashes.
		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}
		relSlashes := filepath.ToSlash(rel)
		for _, pattern := range sources {
			patternSlashes := filepath.ToSlash(pattern)
			var ok bool
			var err error
			if matchBy == FindByShellPattern {
				ok, err = doublestar.Match(patternSlashes, relSlashes)
			}
			if matchBy == FindByRegex {
				ok, err = regexp.MatchString(patternSlashes, relSlashes)
			}
			if err != nil {
				return fmt.Errorf("test file pattern '%s' is not supported: %s", patternSlashes, err)
			}

			if ok {
				rel, err := filepath.Rel(rootDir, path)
				if err != nil {
					return err
				}
				rel = filepath.ToSlash(rel)
				files = append(files, rel)
			}
		}
		return nil
	}); err != nil {
		return files, err
	}

	return files, nil
}

// ExcludeFiles returns file list which excluding given excluded file list
func ExcludeFiles(testFiles, excludedList []string) []string {
	var files []string
	for _, t := range testFiles {
		excluded := false
		for _, e := range excludedList {
			if t == e {
				excluded = true
				break
			}
		}
		if !excluded {
			files = append(files, t)
		}
	}

	return files
}

// MatchFiles returns matched file by specified pattern
func MatchFiles(files []string, match []string) []string {
	var res []string
	for _, f := range files {
		for _, pattern := range match {
			if glob.Glob(pattern, f) {
				res = append(res, f)
				break
			}
		}
	}

	return res
}
