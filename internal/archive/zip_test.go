package archive

import (
	"archive/zip"
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestZip(t *testing.T) {
	testCases := []struct {
		testName  string
		src       string
		target    string
		expErr    bool
		expResult string
	}{
		{
			testName:  "it can successfully create zip file",
			src:       "../",
			target:    "./test.zip",
			expErr:    false,
			expResult: `.*archive.*`,
		},
		{
			testName: "it failed with invalid target",
			src:      "../archive",
			target:   "",
			expErr:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			err := Zip(tc.src, tc.target)
			if err != nil {
				assert.True(t, tc.expErr)
			} else {
				assert.False(t, tc.expErr)
				r, err := zip.OpenReader(tc.target)
				if err != nil {
					t.Errorf("generated zip file is invalid: %s", err.Error())
				} else {
					success := false
					for _, f := range r.File {
						if matched, _ := regexp.MatchString(tc.expResult, f.Name); matched {
							success = true
						}
					}
					if !success {
						t.Errorf("generated zip file is invalid")
					}
				}
				defer r.Close()
			}
			t.Cleanup(func() {
				os.Remove(tc.target)
			})
		})
	}
}
