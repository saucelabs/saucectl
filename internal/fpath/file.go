package fpath

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/rs/zerolog/log"
)

const (
	FindByRegex        = "regex"
	FindByShellPattern = "shellpattern"
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

// Walk evaluates each glob pattern of paths and walks through each directory.
// Returns a list of files that match the given regex pattern.
func Walk(paths []string, pattern string) ([]string, error) {
	var files []string

	r, err := regexp.Compile(pattern)
	if err != nil {
		return files, err
	}

	paths = Globs(paths)
	for _, f := range paths {
		if info, err := os.Stat(f); err == nil {
			if info.IsDir() {
				ff, err := List(f, pattern)
				if err != nil {
					return files, err
				}
				files = append(files, ff...)
			}
			if !info.IsDir() && r.MatchString(info.Name()) {
				files = append(files, f)
			}
		}
	}

	return files, nil
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

// DeepCopy performs a deep copy of src to target, creating all folders leading up to target if necessary.
func DeepCopy(src string, target string) error {
	prefix := filepath.Dir(target)
	if err := os.MkdirAll(prefix, os.ModePerm); err != nil {
		return err
	}

	finfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !finfo.IsDir() {
		input, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		return os.WriteFile(target, input, 0644)
	}

	fis, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, ff := range fis {
		nSrc := filepath.Join(src, ff.Name())
		nTarget := filepath.Join(target, ff.Name())
		if err := DeepCopy(nSrc, nTarget); err != nil {
			return err
		}
	}

	return nil
}

// FindFiles returns a list of files as identified by the sources. Source pattern interpretation (e.g. regex or glob) is controlled by matchBy.
func FindFiles(rootDir string, sources []string, matchBy string) ([]string, error) {
	files := []string{}
	if err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		// Normalize path separators, since the target execution environment may not support backslashes.
		pathSlashes := filepath.ToSlash(path)
		relSlashes, err := filepath.Rel(rootDir, pathSlashes)
		if err != nil {
			return err
		}

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
