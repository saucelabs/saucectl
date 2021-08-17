package table

import (
	"bytes"
	"github.com/saucelabs/saucectl/internal/report"
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
			want:
			`
       Name                              Duration    Status    Browser    Platform    
──────────────────────────────────────────────────────────────────────────────────────
  ✔    Firefox                                34s    passed    Firefox    Windows 10  
  ✔    Chrome                                  5s    passed    Chrome     Windows 10  
──────────────────────────────────────────────────────────────────────────────────────
  ✔    All tests have passed                  39s                                     
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
			want:
			`
       Name                               Duration    Status    Browser    Platform    
───────────────────────────────────────────────────────────────────────────────────────
  ✔    Firefox                                 34s    passed    Firefox    Windows 10  
  ✖    Chrome                                2m51s    failed    Chrome     Windows 10  
───────────────────────────────────────────────────────────────────────────────────────
  ✖    1 of 2 suites have failed (50%)       3m25s                                     
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
