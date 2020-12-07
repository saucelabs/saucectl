package jenkins

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
			want: "4a0e21f5028803b757faf6c14cc6e3ecd4c561bb",
			beforeTest: func() {
				os.Setenv("BUILD_NUMBER", "1")
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
				os.Setenv("BUILD_NUMBER", "123")
			},
		},
		{
			name: "unavailable",
			want: false,
			beforeTest: func() {
				os.Unsetenv("BUILD_NUMBER")
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
		BuildNumber     string
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
				BuildNumber:     tt.fields.BuildNumber,
				overrideBuildID: tt.fields.overrideBuildID,
			}
			p.SetBuildID(tt.args.id)
			if p.BuildID() != tt.args.id {
				t.Errorf("BuildID() = %v, want %v", p.overrideBuildID, tt.args.id)
			}
		})
	}
}