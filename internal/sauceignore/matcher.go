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

// ReadIgnoreFile reads .sauceignore file and creates ignore patters if .sauceignore file is exists.
func ReadIgnoreFile(path string) ([]gitignore.Pattern, error) {
	fPath := filepath.Join(path, sauceignore)
	f, err := os.Open(fPath)
	if err != nil {
		// In case if .sauceignore file doesn't exists.
		if os.IsNotExist(err) {
			return []gitignore.Pattern{}, nil
		}
		return []gitignore.Pattern{}, err
	}
	defer f.Close()

	ps := []gitignore.Pattern{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		s := scanner.Text()
		if !strings.HasPrefix(s, commentPrefix) && len(strings.TrimSpace(s)) > 0 {
			ps = append(ps, gitignore.ParsePattern(s, nil))
		}
	}

	return ps, nil
}

// Matcher defines matcher for sauceignore patterns.
type Matcher interface {
	Match(path []string, isDir bool) bool
}

type matcher struct {
	matcher gitignore.Matcher
}

// Match matches patterns.
func (m *matcher) Match(path []string, isDir bool) bool {
	return m.matcher.Match(path, isDir)
}

// NewMatcher constructs a new matcher.
func NewMatcher(ps []gitignore.Pattern) Matcher {
	if len(ps) == 0 {
		return nil
	}

	return &matcher{matcher: gitignore.NewMatcher(ps)}
}
