// Package grep implements functions to parse and filter spec files like playwright is doing.
//
// See https://playwright.dev/docs/test-annotations#tag-tests for details.
package grep

import (
	"io/fs"
	"regexp"

	"github.com/saucelabs/saucectl/internal/playwright/code"
)

// MatchFiles finds the files whose contents match the grep expression in the title parameter
func MatchFiles(sys fs.FS, files []string, grep string, grepInvert string) (matched []string, unmatched []string) {
	grepRE, grepInvertRE := compileRE(grep, grepInvert)

	for _, f := range files {
		if match(f, grepRE, grepInvertRE) {
			matched = append(matched, f)
			continue
		}

		// When there is a value in grepInvert, if filename matches the pattern, spec file will be skipped.
		if grepInvertRE != nil {
			unmatched = append(unmatched, f)
			continue
		}

		b, err := fs.ReadFile(sys, f)
		if err != nil {
			continue
		}

		testcases := code.Parse(string(b))

		include := false
		for _, tc := range testcases {
			include = include || match(tc.Title, grepRE, grepInvertRE)
			if include {
				// As long as one testcase matched, we know the spec will need to be executed
				matched = append(matched, f)
				break
			}
		}
		if !include {
			unmatched = append(unmatched, f)
		}
	}
	return matched, unmatched
}

// compileRE compiles the regexp contained in grep/grepInvert.
// No pattern specified generates nil value.
func compileRE(grep, grepInvert string) (*regexp.Regexp, *regexp.Regexp) {
	var grepRE *regexp.Regexp
	var grepInvertRE *regexp.Regexp
	if grep != "" {
		grepRE, _ = regexp.Compile(grep)

	}
	if grepInvert != "" {
		grepInvertRE, _ = regexp.Compile(grepInvert)
	}
	return grepRE, grepInvertRE
}

func match(title string, grepRe *regexp.Regexp, grepInvertRe *regexp.Regexp) bool {
	if title == "" {
		return true
	}
	if grepRe != nil && grepInvertRe == nil {
		return grepRe.MatchString(title)
	}
	if grepRe == nil && grepInvertRe != nil {
		return !grepInvertRe.MatchString(title)
	}
	return grepRe.MatchString(title) && !grepInvertRe.MatchString(title)
}
