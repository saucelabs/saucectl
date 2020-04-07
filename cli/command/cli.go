package command

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/saucelabs/saucectl/cli/docker"
)

// SauceCtlCli is the cli context
type SauceCtlCli struct {
	Docker *docker.Handler
	Logger zerolog.Logger
}

// NewSauceCtlCli creates the context object for the cli
func NewSauceCtlCli() (*SauceCtlCli, error) {
	// UNIX Time is faster and smaller than most timestamps
	// If you set zerolog.TimeFieldFormat to an empty string,
	// logs will write with UNIX time
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	logger.Info().Msg("Start Program")

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
		Logger: logger,
	}

	return cli, nil
}
