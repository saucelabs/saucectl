package fpath

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestMatchFiles(t *testing.T) {
	testCases := []struct {
		name   string
		files  []string
		match  []string
		expLen int
		exp    []string
	}{
		{
			name:   "should match files by match patterns",
			files:  []string{"1.json", "2.log", "3.yml"},
			match:  []string{"*json", "*log", "*yml"},
			expLen: 3,
			exp:    []string{"1.json", "2.log", "3.yml"},
		},
		{
			name:   "should return empty list if not match any files",
			files:  []string{"1.json", "2.log", "3.yml"},
			match:  []string{"*yaml"},
			expLen: 0,
			exp:    []string{},
		},
		{
			name:   "should return empty list if match pattern is empty",
			files:  []string{"1.json", "2.log", "3.yml"},
			match:  []string{},
			expLen: 0,
			exp:    []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := MatchFiles(tc.files, tc.match)
			assert.Equal(t, tc.expLen, len(result))
			for i := 0; i < tc.expLen; i++ {
				assert.Equal(t, tc.exp[i], result[i])
			}
		})
	}
}
