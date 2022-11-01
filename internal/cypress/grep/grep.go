// Package grep implements functions to parse and filter spec files by cypress-grep expressions.
//
// See https://www.npmjs.com/package/@cypress/grep for details on the specific syntax
// of cypress-grep expressions.
package grep

import (
	"io/ioutil"
	"path/filepath"

	"github.com/saucelabs/saucectl/internal/code"
)

// Match finds the files whose contents match the grep expression in the title parameter
// and the grep tag expression in the tag parameter.
func Match(rootDir string, files []string, title string, tag string) (matched []string, unmatched []string) {
	for _, f := range files {
		b, err := ioutil.ReadFile(filepath.Join(rootDir, f))	
		if err != nil {
			continue
		}

		testcases := code.Parse(string(b))
		grepExp := ParseGrepExp(title)
		grepTagsExp := ParseGrepTagsExp(tag)

		for _, tc := range testcases {
			include := true
			if title != "" {
				include = include && grepExp.Eval(tc.Title)
			}
			if tag != "" {
				include = include && grepTagsExp.Eval(tc.Tags)
			}
			if include {
				matched = append(matched, f)
			} else {
				unmatched = append(unmatched, f)
			}
		}
	}

	return matched, unmatched
}
