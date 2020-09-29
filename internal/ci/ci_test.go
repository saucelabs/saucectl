package ci

import (
	"os"
	"testing"

	"github.com/saucelabs/saucectl/cli/config"
	"github.com/saucelabs/saucectl/cli/command"

	"github.com/saucelabs/saucectl/internal/fleet"

	"github.com/stretchr/testify/assert"
)

func TestAvailable(t *testing.T) {
	tests := []struct {
		name     string
		envSetup func()
		want     bool
	}{
		{
			name: "detect CI",
			envSetup: func() {
				os.Setenv("CI", "1")
			},
			want: true,
		},
		{
			name: "detect build identifier",
			envSetup: func() {
				os.Setenv("BUILD_NUMBER", "123")
			},
			want: true,
		},
		{
			name: "detect nothing",
			envSetup: func() {
				os.Unsetenv("CI")
				os.Unsetenv("BUILD_NUMBER")
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.envSetup()
			if got := IsAvailable(); got != tt.want {
				t.Errorf("Available() = %v, want %v", got, tt.want)
			}
		})
	}
}

type FakeSequencer struct {
	fleet.Sequencer
}

func TestRunBeforeExec(t *testing.T) {
        jobConfig := config.Project{}
	cli := &command.SauceCtlCli{}
	seq := FakeSequencer{}
	oldMethod := newRunnerConfiguration
	newRunnerConfiguration = func(path string) (config.RunnerConfiguration, error){
		return config.RunnerConfiguration{}, nil
	}
	r, err := NewRunner(jobConfig, cli, seq)
	assert.Equal(t, err, nil)
	err = r.beforeExec(jobConfig.BeforeExec)
	assert.Equal(t, err, nil)
	newRunnerConfiguration = oldMethod
}

