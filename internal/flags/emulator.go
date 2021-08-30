package flags

import (
	"encoding/csv"
	"errors"
	"fmt"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/msg"
	"strings"
)

// Emulator represents the emulator configuration.
type Emulator struct {
	config.Emulator
	Changed bool
}

// String returns a string represenation of the emulator.
func (e Emulator) String() string {
	if !e.Changed {
		return ""
	}
	return fmt.Sprintf("%+v", e.Emulator)
}

// Set sets the emulator to the values present in s.
// The input has to be a comma separated string in the format of "key=value,key2=value2".
// This method is called by cobra when CLI flags are parsed.
func (e *Emulator) Set(s string) error {
	e.Changed = true

	rec, err := csv.NewReader(strings.NewReader(s)).Read()
	if err != nil {
		return err
	}

	// TODO consider defaulting this in a common place (between config and CLI flags)
	e.PlatformName = "Android"

	for _, v := range rec {
		vs := strings.Split(v, "=")

		if len(vs) < 2 {
			msg.Error("--emulator must be specified using a key-value format, e.g. \"--emulator name=Android Emulator,platformVersion=11.0\"")
			return errors.New("wrong input format; must be of key-value")
		}

		val := vs[1]
		switch vs[0] {
		case "name":
			e.Name = val
		case "orientation":
			e.Orientation = val
		case "platformVersion":
			e.PlatformVersions = []string{val}
		}
	}

	return nil
}

// Type returns the value type.
func (e Emulator) Type() string {
	return "emulator"
}
