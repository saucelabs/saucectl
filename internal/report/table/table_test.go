package table

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/report"
)

func TestReporter_Render(t *testing.T) {
	type fields struct {
		TestResults []report.TestResult
	}
	startTime := time.Now()
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
						Name:      "Firefox",
						Duration:  34479 * time.Millisecond,
						StartTime: startTime,
						EndTime:   startTime.Add(34479 * time.Millisecond),
						Passed:    true,
						Browser:   "Firefox",
						Platform:  "Windows 10",
						Attempts:  3,
					},
					{
						Name:      "Chrome",
						Duration:  5123 * time.Millisecond,
						StartTime: startTime,
						EndTime:   startTime.Add(5123 * time.Millisecond),
						Passed:    true,
						Browser:   "Chrome",
						Platform:  "Windows 10",
						Attempts:  1,
					},
				},
			},
			want: `
       Name                              Duration    Status    Browser    Platform      Attempts  
──────────────────────────────────────────────────────────────────────────────────────────────────
  ✔    Firefox                                34s    passed    Firefox    Windows 10           3  
  ✔    Chrome                                  5s    passed    Chrome     Windows 10           1  
──────────────────────────────────────────────────────────────────────────────────────────────────
  ✔    All tests have passed                  34s                                                 
`,
		},
		{
			name: "with failure",
			fields: fields{
				TestResults: []report.TestResult{
					{
						Name:      "Firefox",
						Duration:  34479 * time.Millisecond,
						StartTime: startTime,
						EndTime:   startTime.Add(34479 * time.Millisecond),
						Passed:    true,
						Browser:   "Firefox",
						Platform:  "Windows 10",
						Attempts:  1,
					},
					{
						Name:      "Chrome",
						Duration:  171452 * time.Millisecond,
						StartTime: startTime,
						EndTime:   startTime.Add(171452 * time.Millisecond),
						Passed:    false,
						Browser:   "Chrome",
						Platform:  "Windows 10",
						Attempts:  3,
					},
				},
			},
			want: `
       Name                               Duration    Status    Browser    Platform      Attempts  
───────────────────────────────────────────────────────────────────────────────────────────────────
  ✔    Firefox                                 34s    passed    Firefox    Windows 10           1  
  ✖    Chrome                                2m51s    failed    Chrome     Windows 10           3  
───────────────────────────────────────────────────────────────────────────────────────────────────
  ✖    1 of 2 suites have failed (50%)       2m51s                                                 
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buffy bytes.Buffer

			r := &Reporter{
				TestResults: tt.fields.TestResults,
				Dst:         &buffy,
			}
			r.Render()

			out := buffy.String()
			fmt.Println(out)
			if !reflect.DeepEqual(out, tt.want) {
				t.Errorf("Render() got = \n%s, want = \n%s", out, tt.want)
			}
		})
	}
}

func TestReporter_Reset(t *testing.T) {
	type fields struct {
		TestResults []report.TestResult
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name: "expect empty render",
			fields: fields{
				TestResults: []report.TestResult{
					{
						Name:     "Firefox",
						Duration: 34479 * time.Millisecond,
						Passed:   true,
						Browser:  "Firefox",
						Platform: "Windows 10",
					}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Reporter{
				TestResults: tt.fields.TestResults,
			}
			r.Reset()

			if len(r.TestResults) != 0 {
				t.Errorf("len(TestResults) got = %d, want = %d", len(r.TestResults), 0)
			}
		})
	}
}

func TestReporter_Add(t *testing.T) {
	type fields struct {
		TestResults []report.TestResult
	}
	type args struct {
		t report.TestResult
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name:   "just one",
			fields: fields{},
			args: args{
				t: report.TestResult{
					Name:     "Firefox",
					Duration: 34479 * time.Millisecond,
					Passed:   true,
					Browser:  "Firefox",
					Platform: "Windows 10",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Reporter{
				TestResults: tt.fields.TestResults,
			}
			r.Add(tt.args.t)

			if len(r.TestResults) != 1 {
				t.Errorf("len(TestResults) got = %d, want = %d", len(r.TestResults), 1)
			}
			if !reflect.DeepEqual(r.TestResults[0], tt.args.t) {
				t.Errorf(" got = %v, want = %v", r.TestResults[0], tt.args.t)
			}
		})
	}
}
