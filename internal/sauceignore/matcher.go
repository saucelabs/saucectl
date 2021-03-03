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
func ReadIgnoreFile(path string) ([]Pattern, error) {
	fPath := filepath.Join(path, sauceignore)
	f, err := os.Open(fPath)
	if err != nil {
		// In case if .sauceignore file doesn't exists.
		if os.IsNotExist(err) {
			return []Pattern{}, nil
		}
		return []Pattern{}, err
	}
	defer f.Close()

	ps := []Pattern{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		s := scanner.Text()
		if !strings.HasPrefix(s, commentPrefix) && len(strings.TrimSpace(s)) > 0 {
			ps = append(ps, NewPattern(s, nil))
		}
	}

	return ps, nil
}

// Pattern defines a single sauceignore pattern.
type Pattern struct {
	P      string
	Domain []string
}

// NewPattern create new Pattern.
func NewPattern(p string, domain []string) Pattern {
	return Pattern{P: p, Domain: domain}
}

func convPtrnsToGitignorePtrns(pp []Pattern) []gitignore.Pattern {
	res := make([]gitignore.Pattern, len(pp))
	for i := 0; i < len(pp); i++ {
		res[i] = gitignore.ParsePattern(pp[i].P, pp[i].Domain)
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
