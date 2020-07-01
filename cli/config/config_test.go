package config

import (
	"os"
	"reflect"
	"testing"

	"github.com/docker/docker/pkg/testutil/assert"
	"gotest.tools/v3/fs"
)

func TestJobConfiguration(t *testing.T) {
	dir := fs.NewDir(t, "fixtures",
		fs.WithFile("invalid_config.yaml", "foo", fs.WithMode(0755)),
		fs.WithFile("valid_config.yaml", "apiVersion: 1.2", fs.WithMode(0755)))
	defer dir.Remove()

	cases := []struct {
		Name       string
		Input      string
		ShouldPass bool
	}{
		{"With nil filename", "", false},
		{"With non existing config", "/dont/exist", false},
		{"With non invalid config", dir.Path() + "/invalid_config.yaml", false},
		{"With valid config", dir.Path() + "/valid_config.yaml", true},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			configObject, err := NewJobConfiguration(tc.Input)
			if !tc.ShouldPass {
				if err == nil {
					t.Error("No error was returned for failing test case")
				}

				return
			}

			if err != nil {
				t.Error("Error was returned for passing test case")
			}

			assert.Equal(t, configObject.APIVersion, "1.2")
		})
	}
}

func TestRunnerConfiguration(t *testing.T) {
	dir := fs.NewDir(t, "fixtures",
		fs.WithFile("valid_config.yaml", "rootDir: /foo/bar", fs.WithMode(0755)))
	defer dir.Remove()

	configObject, err := NewRunnerConfiguration(dir.Path() + "/valid_config.yaml")
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, configObject.RootDir, "/foo/bar")
}

func TestMetadata_ExpandEnv(t *testing.T) {
	tests := []struct {
		name       string
		fields     Metadata
		beforeTest func()
		want       Metadata
	}{
		{
			name: "var replacement",
			fields: Metadata{
				Name:  "Test $tname",
				Tags:  []string{"$ttag"},
				Build: "Build $tbuild",
			},
			beforeTest: func() {
				os.Setenv("tname", "Envy")
				os.Setenv("ttag", "xp1")
				os.Setenv("tbuild", "Bob")
			},
			want: Metadata{
				Name:  "Test Envy",
				Tags:  []string{"xp1"},
				Build: "Build Bob",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.beforeTest()

			m := Metadata{
				Name:  tt.fields.Name,
				Tags:  tt.fields.Tags,
				Build: tt.fields.Build,
			}

			m.ExpandEnv()

			if !reflect.DeepEqual(m, tt.want) {
				t.Errorf("ExpandEnv() = %v, want %v", m, tt.want)
			}
		})
	}
}
