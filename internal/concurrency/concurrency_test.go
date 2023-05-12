package concurrency

import (
	"testing"

	"gotest.tools/v3/assert"
)

func Test_SplitTests(t *testing.T) {
	var testCases = []struct {
		name      string
		files     []string
		count     int
		expResult [][]string
	}{
		{
			name:      "concuurrency is 1",
			files:     []string{"1", "2", "3"},
			count:     1,
			expResult: [][]string{[]string{"1", "2", "3"}},
		},
		{
			name:      "concuurrency is less than file count",
			files:     []string{"1", "2", "3", "4", "5"},
			count:     3,
			expResult: [][]string{[]string{"1", "4"}, []string{"2", "5"}, []string{"3"}},
		},
		{
			name:      "file count can be devidec by concurrency",
			files:     []string{"1", "2", "3"},
			count:     3,
			expResult: [][]string{[]string{"1"}, []string{"2"}, []string{"3"}},
		},
		{
			name:      "concuurrency is greater than file count",
			files:     []string{"1", "2", "3"},
			count:     5,
			expResult: [][]string{[]string{"1"}, []string{"2"}, []string{"3"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := SplitTests(tc.files, tc.count)
			assert.Equal(t, len(tc.expResult), len(result))
			for i := 0; i < len(result); i++ {
				for j := 0; j < len(result[i]); j++ {
					assert.Equal(t, tc.expResult[i][j], result[i][j])
				}
			}
		})
	}
}
