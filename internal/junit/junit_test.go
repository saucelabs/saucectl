package junit

import (
	"encoding/xml"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want TestSuites
	}{
		{
			name: "should parse without root testsuites element",
			data: []byte(`
			<testsuite errors="3" package="com.example.android.testing.androidjunitrunnersample" tests="66" time="42.962">
			</testsuite>
			`),
			want: TestSuites{
				TestSuites: []TestSuite{
					{
						Errors:  3,
						Package: "com.example.android.testing.androidjunitrunnersample",
						Tests:   66,
						Time:    "42.962",
					},
				},
			},
		},
		{
			name: "should parse with root testsuites element",
			data: []byte(`
			<testsuites>
				<testsuite errors="3" package="com.example.android.testing.androidjunitrunnersample" tests="66" time="42.962">
				</testsuite>
			</testsuites>
			`),
			want: TestSuites{
				XMLName: xml.Name{Space: "", Local: "testsuites"},
				TestSuites: []TestSuite{
					{
						Errors:  3,
						Package: "com.example.android.testing.androidjunitrunnersample",
						Tests:   66,
						Time:    "42.962",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := Parse(tt.data)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, 0, got.Failures)
		})
	}
}
