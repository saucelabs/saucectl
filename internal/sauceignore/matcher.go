package sauceignore

import (
	"bufio"
	"os"
	"strings"

	"github.com/saucelabs/saucectl/internal/msg"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

const commentPrefix = "#"

// PatternsFromFile reads .sauceignore file and creates ignore patters if .sauceignore file is exists.
func PatternsFromFile(path string) ([]Pattern, error) {
	if path == "" {
		return []Pattern{}, nil
	}

	f, err := os.Open(path)
	if err != nil {
		// In case .sauceignore file doesn't exists.
		if os.IsNotExist(err) {
			msg.LogSauceIgnoreNotExist()
			return []Pattern{}, nil
		}
		return []Pattern{}, err
	}
	defer f.Close()

	var ps []Pattern
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		s := scanner.Text()
		if !strings.HasPrefix(s, commentPrefix) && len(strings.TrimSpace(s)) > 0 {
			ps = append(ps, NewPattern(s))
		}
	}

	return ps, nil
}

// Pattern defines a single sauceignore pattern.
type Pattern struct {
	P string
}

// NewPattern create new Pattern.
func NewPattern(p string) Pattern {
	return Pattern{P: p}
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
func NewMatcher(patterns []Pattern) Matcher {
	pp := make([]gitignore.Pattern, len(patterns))
	for i, p := range patterns {
		pp[i] = gitignore.ParsePattern(p.P, nil)
	}

	return &matcher{matcher: gitignore.NewMatcher(pp)}
}

// NewMatcherFromFile constructs a new matcher from file.
func NewMatcherFromFile(path string) (Matcher, error) {
	ps, err := PatternsFromFile(path)
	if err != nil {
		return nil, err
	}

	return NewMatcher(ps), nil
}
