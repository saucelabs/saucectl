package sauceignore

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

const commentPrefix = "#"
const sauceignore = ".sauceignore"

// ReadIgnoreFile reads .sauceignore file and creates ignore patters.
func ReadIgnoreFile(path string) ([]gitignore.Pattern, error) {
	// In case if .sauceignore file doesn't exists.
	fPath := filepath.Join(path, sauceignore)
	_, err := os.Stat(fPath)
	if err != nil {
		return nil, nil
	}

	f, err := os.Open(fPath)
	ps := []gitignore.Pattern{}
	if err == nil {
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			s := scanner.Text()
			if !strings.HasPrefix(s, commentPrefix) && len(strings.TrimSpace(s)) > 0 {
				ps = append(ps, gitignore.ParsePattern(s, nil))
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	return ps, nil
}

// Matcher defines matcher for sauceignore patterns.
type Matcher interface {
	Match(path []string, isDir bool) bool
}

// TODO
// SauceMatcher ...
type SauceMatcher struct {
	Matcher gitignore.Matcher
}

// TODO
// Match ...
func (sm *SauceMatcher) Match(path []string, isDir bool) bool {
	return sm.Matcher.Match(path, isDir)
}

// TODO
// NewSauceMatcher ...
func NewSauceMatcher(ps []gitignore.Pattern) Matcher {
	if len(ps) == 0 {
		return nil
	}

	return &SauceMatcher{Matcher: gitignore.NewMatcher(ps)}
}
