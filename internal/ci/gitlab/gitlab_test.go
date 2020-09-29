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
			want: "cc54702bac9b00dc6ddc091f6c8c0c2d1a3c03a9",
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

func TestPipeline_SetBuildID(t *testing.T) {
	type fields struct {
		CIPipelineID    string
		CIJobStage      string
		overrideBuildID string
	}
	type args struct {
		id string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name:   "override",
			fields: fields{},
			args:   args{"123"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pipeline{
				CIPipelineID:    tt.fields.CIPipelineID,
				CIJobStage:      tt.fields.CIJobStage,
				overrideBuildID: tt.fields.overrideBuildID,
			}
			p.SetBuildID(tt.args.id)
			if p.BuildID() != tt.args.id {
				t.Errorf("BuildID() = %v, want %v", p.overrideBuildID, tt.args.id)
			}
		})
	}
}
