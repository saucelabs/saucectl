package config

import (
	"fmt"
	"os"
)

func ExampleMetadata_ExpandEnv() {
	os.Setenv("tname", "Envy")
	os.Setenv("ttag", "xp1")
	os.Setenv("tbuild", "Bob")

	m := Metadata{
		Name:  "Test $tname",
		Tags:  []string{"$ttag"},
		Build: "Build $tbuild",
	}

	m.ExpandEnv()

	fmt.Println(m)
	// Output: {Test Envy [xp1] Build Bob}
}
