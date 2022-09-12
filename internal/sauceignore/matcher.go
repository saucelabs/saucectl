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
	// make sure that sauce-runner.json is never accidentally ignored by the user
	ps = append(ps, Pattern{"!sauce-runner.json"})

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

func convPtrnsToGitignorePtrns(pp []Pattern) []gitignore.Pattern {
	res := make([]gitignore.Pattern, len(pp))
	for i := 0; i < len(pp); i++ {
		res[i] = gitignore.ParsePattern(pp[i].P, nil)
	}

	return res
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
func NewMatcher(ps []Pattern) Matcher {
	gps := convPtrnsToGitignorePtrns(ps)
	return &matcher{matcher: gitignore.NewMatcher(gps)}
}

// NewMatcherFromFile constructs a new matcher from file.
func NewMatcherFromFile(path string) (Matcher, error) {
	ps, err := PatternsFromFile(path)
	if err != nil {
		return nil, err
	}

	return NewMatcher(ps), nil
}

// Dedupe takes a list of patterns and returns them back sans any duplicates.
func Dedupe(patterns []Pattern) []Pattern {
	hash := make(map[Pattern]struct{})
	var list []Pattern
	for _, p := range patterns {
		if _, ok := hash[p]; !ok {
			hash[p] = struct{}{}
			list = append(list, p)
		}
	}
	return list
}
