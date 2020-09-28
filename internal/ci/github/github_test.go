package github

import (
	"os"
	"testing"
)

func TestPipeline_BuildID(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{
			name: "predictable",
			want: "cb40652a8cb08755eab941fa67cf006ed809b649",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("GITHUB_WORKFLOW", "mytest")
			os.Setenv("GITHUB_RUN_ID", "1")
			os.Setenv("INVOCATION_ID", "2")
			p := FromEnv()
			if got := p.BuildID(); got != tt.want {
				t.Errorf("BuildID() = %v, want %v", got, tt.want)
			}
		})
	}
}
