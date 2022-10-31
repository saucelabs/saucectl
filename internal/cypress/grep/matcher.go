package grep

import (
	"io/ioutil"
	"path/filepath"

	"github.com/saucelabs/saucectl/internal/code"
)

func Match(rootDir string, files []string, title string) []string {
	var matched []string
	for _, f := range files {
		b, err := ioutil.ReadFile(filepath.Join(rootDir, f))	
		if err != nil {
		}

		testcases := code.Parse(string(b))
		expr := ParseGrepExp(title)

		for _, tc := range testcases {
			if expr.Eval(tc.Title) {
				matched = append(matched, f)
			}
		}
	}
	return matched
}
