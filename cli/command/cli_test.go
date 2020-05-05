package command

import (
	"os"
	"testing"
)

func TestNoUpdateInCI(t *testing.T) {
	err := os.Setenv("CI", "1")
	if err != nil {
		t.Fatal(err)
	}

	cli := SauceCtlCli{}
	err = checkUpdates(&cli)
	if err != nil {
		t.Fatal(err)
	}
}
