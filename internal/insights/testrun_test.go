package insights

import (
	"github.com/saucelabs/saucectl/internal/saucereport"
	"reflect"
	"testing"
	"time"
)

func Test_uniformizeJSONStatus(t *testing.T) {
	type args struct {
		status string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Success",
			args: args{
				status: "success",
			},
			want: StatePassed,
		},
		{
			name: "Failed",
			args: args{
				status: "failed",
			},
			want: StateFailed,
		},
		{
			name: "Errored",
			args: args{
				status: "error",
			},
			want: StateFailed,
		},
		{
			name: "Skipped",
			args: args{
				status: "skipped",
			},
			want: StateSkipped,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := uniformizeJSONStatus(tt.args.status); got != tt.want {
				t.Errorf("uniformizeJSONStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFromSauceReport(t *testing.T) {
	type args struct {
		report saucereport.SauceReport
	}
	tests := []struct {
		name    string
		args    args
		want    []TestRun
		wantErr bool
	}{
		{
			name: "Basic Passing Report",
			args: args{
				report: saucereport.SauceReport{
					Status: StatePassed,
					Suites: []saucereport.Suite{
						{
							Name: "Suite #1",
							Tests: []saucereport.Test{
								{
									Name:      "Test #1.1",
									Status:    StatePassed,
									StartTime: time.Date(2022, 12, 13, 14, 15, 16, 17, time.UTC),
									Duration:  20,
								},
								{
									Name:      "Test #1.2",
									Status:    StateFailed,
									StartTime: time.Date(2022, 12, 13, 14, 15, 16, 17, time.UTC),
									Duration:  20,
									Output:    "my-dummy-failure",
								},
							},
						},
					},
				},
			},
			want: []TestRun{
				{
					Name:         "Test #1.1",
					CreationTime: time.Date(2022, 12, 13, 14, 15, 16, 17, time.UTC),
					StartTime:    time.Date(2022, 12, 13, 14, 15, 16, 17, time.UTC),
					EndTime:      time.Date(2022, 12, 13, 14, 15, 36, 17, time.UTC),
					Duration:     20,
					Status:       StatePassed,
				},
				{
					Name:         "Test #1.2",
					CreationTime: time.Date(2022, 12, 13, 14, 15, 16, 17, time.UTC),
					StartTime:    time.Date(2022, 12, 13, 14, 15, 16, 17, time.UTC),
					EndTime:      time.Date(2022, 12, 13, 14, 15, 36, 17, time.UTC),
					Duration:     20,
					Status:       StateFailed,
					Errors: []TestRunError{
						{
							Message: "my-dummy-failure",
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromSauceReport(tt.args.report)

			// Replicate IDs as they are random
			for idx := range got {
				tt.want[idx].ID = got[idx].ID
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("FromSauceReport() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FromSauceReport() got = %v, want %v", got, tt.want)
			}
		})
	}
}
