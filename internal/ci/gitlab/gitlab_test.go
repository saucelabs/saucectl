package gitlab

import (
	"os"
	"testing"
)

func TestPipeline_BuildID(t *testing.T) {
	tests := []struct {
		name       string
		want       string
		beforeTest func()
	}{
		{
			name: "predictable",
			want: "073fcbd01693cc8cb1c00ab959a8e2d8cf96c0de",
			beforeTest: func() {
				os.Setenv("CI_PIPELINE_ID", "1")
				os.Setenv("CI_JOB_STAGE", "test")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.beforeTest()
			p := FromEnv()
			if got := p.BuildID(); got != tt.want {
				t.Errorf("BuildID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAvailable(t *testing.T) {
	tests := []struct {
		name       string
		want       bool
		beforeTest func()
	}{
		{
			name: "available",
			want: true,
			beforeTest: func() {
				os.Setenv("GITLAB_CI", "true")
			},
		},
		{
			name: "unavailable",
			want: false,
			beforeTest: func() {
				os.Unsetenv("GITLAB_CI")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.beforeTest()
			if got := Available(); got != tt.want {
				t.Errorf("Available() = %v, want %v", got, tt.want)
			}
		})
	}
}
