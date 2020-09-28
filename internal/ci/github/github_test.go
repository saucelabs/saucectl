package github

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
			want: "cb40652a8cb08755eab941fa67cf006ed809b649",
			beforeTest: func() {
				os.Setenv("GITHUB_WORKFLOW", "mytest")
				os.Setenv("GITHUB_RUN_ID", "1")
				os.Setenv("INVOCATION_ID", "2")
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
				os.Setenv("GITHUB_RUN_ID", "1")
			},
		},
		{
			name: "unavailable",
			want: false,
			beforeTest: func() {
				os.Unsetenv("GITHUB_RUN_ID")
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
