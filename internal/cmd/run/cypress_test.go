package run

import (
	"fmt"
	"github.com/saucelabs/saucectl/internal/cypress"
	"os"
	"testing"
)

func Test_expandReporterConfigEnv(t *testing.T) {
	type args struct {
		reporters []cypress.Reporter
	}
	tests := []struct {
		name string
		args args
		env  map[string]string
		want map[string]interface{}
	}{
		{
			name: "Default",
			args: args{
				reporters: []cypress.Reporter{
					{
						Name: "default",
						Options: map[string]interface{}{
							"foo":  "bar",
							"item": "value",
							"number": 0,
							"null": nil,
						},
					},
				},
			},
			want: map[string]interface{}{
				"foo":  "bar",
				"item": "value",
				"number": 0,
				"null": nil,
			},
		},
		{
			name: "Replace root level",
			env: map[string]string{
				"DEMO_ENV": "value",
			},
			args: args{
				reporters: []cypress.Reporter{
					{
						Name: "default",
						Options: map[string]interface{}{
							"foo":  "bar",
							"item": "${DEMO_ENV}",
							"number": 0,
							"null": nil,
						},
					},
				},
			},
			want: map[string]interface{}{
				"foo":  "bar",
				"item": "value",
				"number": 0,
				"null": nil,
			},
		},
		{
			name: "Replace nested level",
			env: map[string]string{
				"DEMO_ENV": "value",
				"DEEPER_ENV": "deep",
			},
			args: args{
				reporters: []cypress.Reporter{
					{
						Name: "default",
						Options: map[string]interface{}{
							"foo":  "bar",
							"item": "${DEMO_ENV}",
							"deeper": map[interface{}]interface{}{
								"item": "${DEEPER_ENV}",
							},
							"number": 0,
							"null": nil,
						},
					},
				},
			},
			want: map[string]interface{}{
				"foo":  "bar",
				"item": "value",
				"deeper": map[string]interface{}{
					"item": "deep",
				},
				"number": 0,
				"null": nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				os.Setenv(k, v)
			}
			expandReporterConfigEnv(tt.args.reporters)
			if fmt.Sprint(tt.args.reporters[0].Options) != fmt.Sprint(tt.want) {
				t.Errorf("expandReporterConfigEnv() = %v, want %v", tt.args.reporters[0].Options, tt.want)
			}
			for k := range tt.env {
				os.Unsetenv(k)
			}
		})
	}
}
