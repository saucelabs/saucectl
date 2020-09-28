package gitlab

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
			want: "073fcbd01693cc8cb1c00ab959a8e2d8cf96c0de",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("CI_PIPELINE_ID", "1")
			os.Setenv("CI_JOB_STAGE", "test")
			p := FromEnv()
			if got := p.BuildID(); got != tt.want {
				t.Errorf("BuildID() = %v, want %v", got, tt.want)
			}
		})
	}
}
