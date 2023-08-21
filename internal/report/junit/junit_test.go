package junit

import (
	"io"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/junit"
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

func TestReduceJunitFiles(t *testing.T) {
	input := []junit.TestSuites{
		{
			TestSuites: []junit.TestSuite{
				{
					Tests:   24,
					Time:    "47.917",
					Package: "com.example.android.testing.espresso.BasicSample",
					TestCases: []junit.TestCase{
						{
							Name:      "changeText_newActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test10Test",
							Status:    "success",
						},
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test10Test",
							Status:    "success",
						},
						{
							Name:      "changeText_newActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test11Test",
							Status:    "success",
						},
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test11Test",
							Status:    "success",
						},
						{
							Name:      "changeText_newActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test12Test",
							Status:    "success",
						},
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test12Test",
							Status:    "error",
							Error: &junit.Error{
								Message: "Whoops!",
								Type:    "androidx.test.espresso.base.WTFException",
								Text:    "A deeply philosophical error message.",
							},
						},
						{
							Name:      "changeText_newActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test1Test",
							Status:    "success",
						},
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test1Test",
							Status:    "success",
						},
						{
							Name:      "changeText_newActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test2Test",
							Status:    "success",
						},
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test2Test",
							Status:    "error",
							Error: &junit.Error{
								Message: "Whoops!",
								Type:    "androidx.test.espresso.base.WTFException",
								Text:    "A deeply philosophical error message.",
							},
						},
						{
							Name:      "changeText_newActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test3Test",
							Status:    "success",
						},
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test3Test",
							Status:    "success",
						},
						{
							Name:      "changeText_newActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test4Test",
							Status:    "success",
						},
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test4Test",
							Status:    "success",
						},
						{
							Name:      "changeText_newActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test5Test",
							Status:    "success",
						},
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test5Test",
							Status:    "success",
						},
						{
							Name:      "changeText_newActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test6Test",
							Status:    "success",
						},
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test6Test",
							Status:    "success",
						},
						{
							Name:      "changeText_newActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test7Test",
							Status:    "success",
						},
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test7Test",
							Status:    "success",
						},
						{
							Name:      "changeText_newActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test8Test",
							Status:    "success",
						},
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test8Test",
							Status:    "success",
						},
						{
							Name:      "changeText_newActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test9Test",
							Status:    "success",
						},
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test9Test",
							Status:    "success",
						},
					},
					SystemOut: "<Base System Output>",
				},
			},
		},
		{
			TestSuites: []junit.TestSuite{
				{
					Tests:   2,
					Time:    "11.007",
					Package: "com.example.android.testing.espresso.BasicSample",
					TestCases: []junit.TestCase{
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test2Test",
							Status:    "success",
						},
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test12Test",
							Status:    "error",
							Error: &junit.Error{
								Message: "Whoops!",
								Type:    "androidx.test.espresso.base.WTFException",
								Text:    "A deeply philosophical error message.",
							},
						},
					},
					SystemOut: "<Base System Output>",
				},
			},
		},
		{
			TestSuites: []junit.TestSuite{
				{
					Tests:   1,
					Time:    "6.004",
					Package: "com.example.android.testing.espresso.BasicSample",
					TestCases: []junit.TestCase{
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test12Test",
							Status:    "error",
							Error: &junit.Error{
								Message: "Whoops!",
								Type:    "androidx.test.espresso.base.WTFException",
								Text:    "A deeply philosophical error message.",
							},
						},
					},
					SystemOut: "<Base System Output>",
				},
			},
		},
		{
			TestSuites: []junit.TestSuite{
				{
					Tests:   1,
					Time:    "5.535",
					Package: "com.example.android.testing.espresso.BasicSample",
					TestCases: []junit.TestCase{
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test12Test",
							Status:    "success",
						},
					},
					SystemOut: "<Base System Output>",
				},
			},
		},
	}
	want := junit.TestSuites{
		TestSuites: []junit.TestSuite{
			{
				Tests:   24,
				Time:    "47.917",
				Package: "com.example.android.testing.espresso.BasicSample",
				TestCases: []junit.TestCase{
					{
						Name:      "changeText_newActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test10Test",
						Status:    "success",
					},
					{
						Name:      "changeText_sameActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test10Test",
						Status:    "success",
					},
					{
						Name:      "changeText_newActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test11Test",
						Status:    "success",
					},
					{
						Name:      "changeText_sameActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test11Test",
						Status:    "success",
					},
					{
						Name:      "changeText_newActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test12Test",
						Status:    "success",
					},
					{
						Name:      "changeText_sameActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test12Test",
						Status:    "success",
					},
					{
						Name:      "changeText_newActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test1Test",
						Status:    "success",
					},
					{
						Name:      "changeText_sameActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test1Test",
						Status:    "success",
					},
					{
						Name:      "changeText_newActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test2Test",
						Status:    "success",
					},
					{
						Name:      "changeText_sameActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test2Test",
						Status:    "success",
					},
					{
						Name:      "changeText_newActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test3Test",
						Status:    "success",
					},
					{
						Name:      "changeText_sameActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test3Test",
						Status:    "success",
					},
					{
						Name:      "changeText_newActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test4Test",
						Status:    "success",
					},
					{
						Name:      "changeText_sameActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test4Test",
						Status:    "success",
					},
					{
						Name:      "changeText_newActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test5Test",
						Status:    "success",
					},
					{
						Name:      "changeText_sameActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test5Test",
						Status:    "success",
					},
					{
						Name:      "changeText_newActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test6Test",
						Status:    "success",
					},
					{
						Name:      "changeText_sameActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test6Test",
						Status:    "success",
					},
					{
						Name:      "changeText_newActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test7Test",
						Status:    "success",
					},
					{
						Name:      "changeText_sameActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test7Test",
						Status:    "success",
					},
					{
						Name:      "changeText_newActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test8Test",
						Status:    "success",
					},
					{
						Name:      "changeText_sameActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test8Test",
						Status:    "success",
					},
					{
						Name:      "changeText_newActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test9Test",
						Status:    "success",
					},
					{
						Name:      "changeText_sameActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test9Test",
						Status:    "success",
					},
				},
				SystemOut: "<Base System Output>",
			},
		},
	}

	got := reduceTestSuites(input)
	if !cmp.Equal(want, got) {
		t.Error(cmp.Diff(want, got))
	}
}
