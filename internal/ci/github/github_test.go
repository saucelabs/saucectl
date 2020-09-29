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
			want: "a4c9176ecce783ffc964c3cb62248b836380ee36",
			beforeTest: func() {
				os.Setenv("GITHUB_WORKFLOW", "mytest")
				os.Setenv("GITHUB_RUN_ID", "1")
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

func TestPipeline_SetBuildID(t *testing.T) {
	type fields struct {
		GithubWorkflow  string
		GithubRunID     string
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
				GithubWorkflow:  tt.fields.GithubWorkflow,
				GithubRunID:     tt.fields.GithubRunID,
				overrideBuildID: tt.fields.overrideBuildID,
			}
			p.SetBuildID(tt.args.id)
			if p.BuildID() != tt.args.id {
				t.Errorf("BuildID() = %v, want %v", p.overrideBuildID, tt.args.id)
			}
		})
	}
}
