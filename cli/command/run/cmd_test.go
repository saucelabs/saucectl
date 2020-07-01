package run

import (
	"github.com/saucelabs/saucectl/cli/config"
	"os"
	"reflect"
	"testing"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/stretchr/testify/assert"
	"gotest.tools/v3/fs"
)

func TestNewRunCommand(t *testing.T) {
	dir := fs.NewDir(t, "fixtures",
		fs.WithFile("config.yaml", "apiVersion: 1.2\nimage:\n  base: test", fs.WithMode(0755)))
	cli := command.SauceCtlCli{}
	cmd := Command(&cli)
	assert.Equal(t, cmd.Use, runUse)

	if err := cmd.Flags().Set("config", dir.Path()+"/config.yaml"); err != nil {
		t.Fatal(err)
	}

	var args []string
	exitCode, err := Run(cmd, &cli, args)

	assert.Equal(t, err, nil)
	assert.Equal(t, exitCode, 123)
}

func Test_expandEnvMetadata(t *testing.T) {
	type args struct {
		cfg *config.JobConfiguration
	}
	tests := []struct {
		name      string
		args      args
		envarFunc func()
		want      config.Metadata
	}{
		{
			name: "env var replacement",
			args: args{cfg: &config.JobConfiguration{
				Metadata: config.Metadata{
					Name:  "Test $tname",
					Tags:  []string{"$ttag"},
					Build: "Build $tbuild",
				},
			}},
			envarFunc: func() {
				os.Setenv("tname", "Envy")
				os.Setenv("ttag", "xp1")
				os.Setenv("tbuild", "Bob")
			},
			want: config.Metadata{
				Name:  "Test Envy",
				Tags:  []string{"xp1"},
				Build: "Build Bob",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.envarFunc()
			expandEnvMetadata(tt.args.cfg)
			if !reflect.DeepEqual(tt.args.cfg.Metadata, tt.want) {
				t.Errorf("Metadata = %v, want %v", tt.args.cfg.Metadata, tt.want)
			}
		})
	}
}
