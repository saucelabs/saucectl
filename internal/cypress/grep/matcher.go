package grep

import (
	"io/ioutil"
	"path/filepath"

	"github.com/saucelabs/saucectl/internal/code"
)

func Match(rootDir string, files []string, title string, tag string) []string {
	var matched []string
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
			}
		}
	}
	return matched
}
