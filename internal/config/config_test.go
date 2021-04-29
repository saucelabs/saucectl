package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStandardizeVersionFormat(t *testing.T) {
	assert.Equal(t, "5.6.0", StandardizeVersionFormat("v5.6.0"))
	assert.Equal(t, "5.6.0", StandardizeVersionFormat("5.6.0"))
}

func TestConfig_Validate(t *testing.T) {
	testCases := []struct {
		name      string
		npm       Npm
		expResult string
		expErr    bool
	}{
		{
			name:      "Should not return error when StrictSSL is empty",
			npm:       Npm{StrictSSL: ""},
			expResult: "",
			expErr:    false,
		},
		{
			name:      "Should not return error when StrictSSL is true",
			npm:       Npm{StrictSSL: "true"},
			expResult: "",
			expErr:    false,
		},
		{
			name:      "Should not return error when StrictSSL is false",
			npm:       Npm{StrictSSL: "false"},
			expResult: "",
			expErr:    false,
		},
		{
			name:      "Should not return error when StrictSSL is invalid",
			npm:       Npm{StrictSSL: "test"},
			expResult: "invalid npm strictSSL setting: 'test'",
			expErr:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.npm.Validate()
			if err != nil {
				assert.True(t, tc.expErr)
				assert.Equal(t, tc.expResult, err.Error())
			} else {
				assert.False(t, tc.expErr)
			}
		})
	}
}
