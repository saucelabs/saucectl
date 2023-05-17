package imagerunner

import (
	"testing"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/region"
)

func TestValidate(t *testing.T) {
	type args struct {
		p Project
	}
	tests := []struct {
		name    string
		args    args
		wantErr string
	}{
		{
			name: "Passing",
			args: args{
				p: Project{
					Sauce: config.SauceConfig{
						Region: region.USWest1.String(),
					},
					Suites: []Suite{
						{
							Name:     "Main Suite",
							Workload: "other",
							Image:    "dummy/image",
						},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "No Image",
			args: args{
				p: Project{
					Sauce: config.SauceConfig{
						Region: region.USWest1.String(),
					},
					Suites: []Suite{
						{
							Name:     "Main Suite",
							Workload: "other",
						},
					},
				},
			},
			wantErr: `missing "image" for suite: Main Suite`,
		},
		{
			name: "No Workload Type",
			args: args{
				p: Project{
					Sauce: config.SauceConfig{
						Region: region.USWest1.String(),
					},
					Suites: []Suite{
						{
							Name:  "Main Suite",
							Image: "dummy/image",
						},
					},
				},
			},
			wantErr: `missing "workload" value for suite: Main Suite`,
		},
		{
			name: "Invalid Workload Type",
			args: args{
				p: Project{
					Sauce: config.SauceConfig{
						Region: region.USWest1.String(),
					},
					Suites: []Suite{
						{
							Name:     "Main Suite",
							Image:    "dummy/image",
							Workload: "invalid-workload-type",
						},
					},
				},
			},
			wantErr: `"invalid-workload-type" is an invalid "workload" value for suite: Main Suite`,
		},
		{
			name: "Invalid resourceProfile",
			args: args{
				p: Project{
					Sauce: config.SauceConfig{
						Region: region.USWest1.String(),
					},
					Suites: []Suite{
						{
							Name:            "Main Suite",
							Image:           "dummy/image",
							Workload:        "other",
							ResourceProfile: "test",
						},
					},
				},
			},
			wantErr: `invalid resourceProfile for suite: Main Suite, resourceProfile should be one of [c1m1 c2m2 c3m3]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.args.p)
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			if errStr != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", errStr, tt.wantErr)
			}
		})
	}
}
