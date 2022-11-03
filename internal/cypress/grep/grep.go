// Package grep implements functions to parse and filter spec files by cypress-grep expressions.
//
// See https://www.npmjs.com/package/@cypress/grep for details on the specific syntax
// of cypress-grep expressions.
package grep

import (
	"io/fs"

	"github.com/saucelabs/saucectl/internal/cypress/code"
)

// MatchFiles finds the files whose contents match the grep expression in the title parameter
// and the grep tag expression in the tag parameter.
func MatchFiles(sys fs.FS, files []string, title string, tag string) (matched []string, unmatched []string) {
	for _, f := range files {
		b, err := fs.ReadFile(sys, f)

		if err != nil {
			continue
		}

		testcases := code.Parse(string(b))
		grepExp := ParseGrepTitleExp(title)
		grepTagsExp := ParseGrepTagsExp(tag)

		include := false
		for _, tc := range testcases {
			include = include || match(grepExp, grepTagsExp, tc.Title, tc.Tags)
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

func match(titleExp Expression, tagsExp Expression, title string, tags string) bool {
	// Allow empty title to match. This mimics the behaviour of cypress-grep.
	titleMatch := title == "" || titleExp.Eval(title)
	tagMatch := tagsExp.Eval(tags)

	return titleMatch && tagMatch
}
