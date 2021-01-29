package docker

import (
	"context"
	"github.com/saucelabs/saucectl/cli/command"
)

// ContainerRunner represents the container runner for docker.
type ContainerRunner struct {
	Ctx             context.Context
	Cli             *command.SauceCtlCli
	containerID     string
	docker          *Handler
	containerConfig *containerConfig
}
