package fpath

import (
	"github.com/rs/zerolog/log"
	"io/ioutil"
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
			if !info.IsDir() && r.MatchString(f) {
				files = append(files, f)
				continue
			}

			ff, err := List(f, pattern)
			if err != nil {
				return files, err
			}
			files = append(files, ff...)
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
		input, err := ioutil.ReadFile(src)
		if err != nil {
			return err
		}
		return ioutil.WriteFile(target, input, 0644)
	}

	fis, err := ioutil.ReadDir(src)
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
