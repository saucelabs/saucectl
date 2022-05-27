package configure

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_mask(t *testing.T) {
	testcases := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "it should mask access key",
			input:  "1234567-8912-3456-7891-234567891234",
			expect: "*******************************1234",
		},
		{
			name:   "it should return empty string when input is empty",
			input:  "",
			expect: "",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			result := mask(tc.input)
			assert.Equal(t, tc.expect, result)
		})
	}
}
