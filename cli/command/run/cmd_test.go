package run

import (
	"github.com/saucelabs/saucectl/internal/region"
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

func Test_apiBaseURL(t *testing.T) {
	type args struct {
		r region.Region
	}
	tests := []struct {
		name     string
		args     args
		sauceAPI string
		want     string
	}{
		{
			name:     "region based",
			args:     args{r: region.EUCentral1},
			sauceAPI: "",
			want:     region.EUCentral1.APIBaseURL(),
		},
		{
			name:     "override",
			args:     args{r: region.USWest1},
			sauceAPI: "https://nowhere.saucelabs.com",
			want:     "https://nowhere.saucelabs.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sauceAPI = tt.sauceAPI
			if got := apiBaseURL(tt.args.r); got != tt.want {
				t.Errorf("apiBaseURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
