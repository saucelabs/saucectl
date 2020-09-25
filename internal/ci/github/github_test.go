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
			want: "4dbcbfc87a92274346776be8e48f477bbc6f15ab",
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
