package cypress

import (
	"github.com/saucelabs/saucectl/internal/config"
	"testing"
	"errors"
	"github.com/stretchr/testify/assert"
)

func TestValidateThrowsErrors(t *testing.T) {
	testCases := []struct {
		name        string
		p           *Project
		expectedErr error
	}{
		{
			name:        "validating throws error on empty suites",
			p:           &Project{},
			expectedErr: errors.New("no suites defined"),
		},
		{
			name:        "validating throws error on wrong docker file transfer mode",
			p:           &Project{
				Suites: []Suite {
					{
						Name: "some suite",
					},
				},
				Docker: config.Docker{
					FileTransfer: "fruta",
				},
			},
			expectedErr: errors.New("illegal file transfer type 'fruta', must be one of 'mount|copy'"),
		},
		{
			name:        "validating throws error on missing browser name",
			p:           &Project{
				Suites: []Suite {
					{
						Name: "some suite",
					},
				},
				Docker: config.Docker{
					FileTransfer: "copy",
				},
			},
			expectedErr: errors.New("no browser specified in suite 'some suite'"),
		},
		{
			name:        "validating throws error on missing config",
			p:           &Project{
				Suites: []Suite {
					{
						Name: "some suite",
						Browser: "chrome",
					},
				},
				Docker: config.Docker{
					FileTransfer: "copy",
				},
			},
			expectedErr: errors.New("no config.testFiles specified in suite 'some suite'"),
		},
		{
			name:        "validating throws error on non-unique suite name",
			p:           &Project{
				Suites: []Suite {
					{
						Name: "some suite",
						Browser: "chrome",
						Config: SuiteConfig{
							TestFiles: []string{"spec.js"},
						},
					},
					{
						Name: "some suite",
						Browser: "chrome",
						Config: SuiteConfig{
							TestFiles: []string{"spec.js"},
						},
					},
				},
				Docker: config.Docker{
					FileTransfer: "copy",
				},
			},
			expectedErr: errors.New("suite names must be unique, but found duplicate for 'some suite'"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(*tc.p)
			assert.NotNil(t, err)
			assert.Equal(t, err.Error(), tc.expectedErr.Error())
		})
	}
}
