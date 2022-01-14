package doctor

import (
	"context"
	"errors"
	"fmt"
	"github.com/fatih/color"
	"github.com/saucelabs/saucectl/internal/docker"
)

// Verify verifies that all essential dependencies are met.
func Verify() error {
	var err error
	if err = VerifyDocker(); err != nil {
		return err
	}

	return err
}

// VerifyDocker verifies that docker dependencies are met.
func VerifyDocker() error {
	handler, err := docker.Create()
	if err != nil {
		return err
	}

	fmt.Println("[•] Docker")
	if !handler.IsInstalled() {
		fmt.Printf("    %s Connection: unable to connect to docker. Is it installed & running?\n", color.RedString("[✖]"))
		return errors.New("unable to communicate with docker")
	}
	fmt.Printf("    %s Connection\n", color.GreenString("[✔]"))

	if err := handler.PullImage(context.Background(), "hello-world"); err != nil {
		fmt.Printf("    %s Pulling Images: %s\n", color.RedString("[✖]"), err)
		return err
	}
	fmt.Printf("    %s Pull Images\n", color.GreenString("[✔]"))

	if err := handler.IsLaunchable("hello-world"); err != nil {
		fmt.Printf("    %s Launch Containers: %s\n", err, color.RedString("[✖]"))
		return err
	}
	fmt.Printf("    %s Launch Containers\n", color.GreenString("[✔]"))

	return nil
}
