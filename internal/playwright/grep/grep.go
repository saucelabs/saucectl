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
func MatchFiles(sys fs.FS, files []string, pattern string) (matched []string, unmatched []string) {
	grepRe, err := regexp.Compile(pattern)
	if err != nil {
		// In case of non-parsable token by use, match all files
		return files, []string{}
	}

	for _, f := range files {
		if grepRe.MatchString(f) {
			matched = append(matched, f)
			continue
		}

		b, err := fs.ReadFile(sys, f)
		if err != nil {
			continue
		}

		testcases := code.Parse(string(b))

		include := false
		for _, tc := range testcases {
			include = include || match(grepRe, tc.Title)
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

func match(titleExp *regexp.Regexp, title string) bool {
	return title == "" || titleExp.MatchString(title)
}
