package junit

import (
	"io"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/report"
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
						Status:   job.StatePassed,
						Browser:  "Firefox",
						Platform: "Windows 10",
						URL:      "https://app.saucelabs.com/tests/1234",
					},
					{
						Name:     "Chrome",
						Duration: 5123 * time.Millisecond,
						Status:   job.StatePassed,
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
						Status:   job.StatePassed,
						Browser:  "Firefox",
						Platform: "Windows 10",
					},
					{
						Name:     "Chrome",
						Duration: 171452 * time.Millisecond,
						Status:   job.StateFailed,
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
		{
			name: "with retries",
			fields: fields{
				TestResults: []report.TestResult{
					{
						Name:     "Chrome",
						Duration: 90 * time.Second,
						Status:   job.StatePassed,
						Browser:  "Chrome",
						Platform: "Windows 11",
						URL:      "https://app.saucelabs.com/tests/job-3",
						Attempts: []report.Attempt{
							{ID: "job-1", Status: job.StateFailed, Duration: 30 * time.Second},
							{ID: "job-2", Status: job.StateFailed, Duration: 28 * time.Second},
							{ID: "job-3", Status: job.StatePassed, Duration: 25 * time.Second},
						},
					},
				},
			},
			want: `<testsuites>
  <testsuite name="Chrome" tests="0" time="90">
    <properties>
      <property name="url" value="https://app.saucelabs.com/tests/job-3"></property>
      <property name="browser" value="Chrome"></property>
      <property name="platform" value="Windows 11"></property>
      <property name="retried" value="true"></property>
      <property name="retries" value="3"></property>
      <property name="attempt.0.id" value="job-1"></property>
      <property name="attempt.0.status" value="failed"></property>
      <property name="attempt.0.duration" value="30"></property>
      <property name="attempt.1.id" value="job-2"></property>
      <property name="attempt.1.status" value="failed"></property>
      <property name="attempt.1.duration" value="28"></property>
      <property name="attempt.2.id" value="job-3"></property>
      <property name="attempt.2.status" value="passed"></property>
      <property name="attempt.2.duration" value="25"></property>
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
				Filename:    f.Name(),
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
