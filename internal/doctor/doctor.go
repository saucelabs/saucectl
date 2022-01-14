package doctor

import (
	"context"
	"errors"
	"fmt"
	"github.com/fatih/color"
	"github.com/saucelabs/saucectl/internal/docker"
	"strings"
)

var indent = "    "

var (
	point = "[•] "
	fail  = color.RedString("[✖] ")
	pass  = color.GreenString("[✔] ")
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

	printPoint(0, "Docker")
	if !handler.IsInstalled() {
		printFail(1, "Connection: unable to connect to docker. Is it installed & running?")
		return errors.New("unable to communicate with docker")
	}
	printPass(1, "Connection")

	if err := handler.PullImage(context.Background(), "hello-world"); err != nil {
		printFail(1, "Pull Images: %s", err)
		return err
	}
	printPass(1, "Pull Images")

	if err := handler.IsLaunchable("hello-world"); err != nil {
		printFail(1, "Launch Containers: %s", err)
		return err
	}
	printPass(1, "Launch Containers")

	return nil
}

func printPoint(lvl int, msg string, a ...interface{}) {
	printBullet(lvl, point, msg, a...)
}

func printFail(lvl int, msg string, a ...interface{}) {
	printBullet(lvl, fail, msg, a...)
}

func printPass(lvl int, msg string, a ...interface{}) {
	printBullet(lvl, pass, msg, a...)
}

func printBullet(lvl int, bullet, msg string, a ...interface{}) {
	var b strings.Builder

	for i := 0; i < lvl; i++ {
		b.WriteString(indent)
	}

	b.WriteString(bullet)

	_, _ = fmt.Fprintf(&b, msg, a...)

	fmt.Println(b.String())
}
