package command

import "github.com/saucelabs/saucectl/cli/docker"

// SauceCtlCli is the cli context
type SauceCtlCli struct {
	Docker *docker.Handler
}

// NewSauceCtlCli creates the context object for the cli
func NewSauceCtlCli() (*SauceCtlCli, error) {
	dockerClient, err := docker.Create()
	if err != nil {
		return nil, err
	}

	err = dockerClient.ValidateDependency()
	if err != nil {
		return nil, err
	}

	cli := &SauceCtlCli{
		Docker: dockerClient,
	}

	return cli, nil
}
