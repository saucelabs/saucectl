package junit

import (
	"github.com/saucelabs/saucectl/internal/report"
	"io"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestReporter_Render(t *testing.T) {
	type fields struct {
		TestResults []report.TestResult
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "all pass",
			fields: fields{
				TestResults: []report.TestResult{
					{
						Name:     "Firefox",
						Duration: 34479 * time.Millisecond,
						Passed:   true,
						Browser:  "Firefox",
						Platform: "Windows 10",
						URL:      "https://app.saucelabs.com/tests/1234",
					},
					{
						Name:     "Chrome",
						Duration: 5123 * time.Millisecond,
						Passed:   true,
						Browser:  "Chrome",
						Platform: "Windows 10",
					},
				},
			},
			want: `<testsuites>
  <testsuite name="Firefox" tests="0" time="34">
    <properties>
      <property name="url" value="https://app.saucelabs.com/tests/1234"></property>
      <property name="browser" value="Firefox"></property>
      <property name="platform" value="Windows 10"></property>
    </properties>
  </testsuite>
  <testsuite name="Chrome" tests="0" time="5">
    <properties>
      <property name="browser" value="Chrome"></property>
      <property name="platform" value="Windows 10"></property>
    </properties>
  </testsuite>
</testsuites>
`,
		},
		{
			name: "with failure",
			fields: fields{
				TestResults: []report.TestResult{
					{
						Name:     "Firefox",
						Duration: 34479 * time.Millisecond,
						Passed:   true,
						Browser:  "Firefox",
						Platform: "Windows 10",
					},
					{
						Name:     "Chrome",
						Duration: 171452 * time.Millisecond,
						Passed:   false,
						Browser:  "Chrome",
						Platform: "Windows 10",
					},
				},
			},
			want: `<testsuites>
  <testsuite name="Firefox" tests="0" time="34">
    <properties>
      <property name="browser" value="Firefox"></property>
      <property name="platform" value="Windows 10"></property>
    </properties>
  </testsuite>
  <testsuite name="Chrome" tests="0" time="171">
    <properties>
      <property name="browser" value="Chrome"></property>
      <property name="platform" value="Windows 10"></property>
    </properties>
  </testsuite>
</testsuites>
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.CreateTemp("", "saucectl-report.xml")
			if err != nil {
				t.Fatalf("Failed to create temp file %s", err)
			}
			//f.Close()
			defer os.Remove(f.Name())

			r := &Reporter{
				TestResults: tt.fields.TestResults,
				Path:        f.Name(),
			}
			r.Render()

			b, err := io.ReadAll(f)
			if err != nil {
				t.Fatalf("Failed to read and verify output file %s due to %s", f.Name(), err)
			}

			bstr := string(b)
			if !reflect.DeepEqual(bstr, tt.want) {
				t.Errorf("Render() got = \n%s, want = \n%s", bstr, tt.want)
			}
		})
	}
}
