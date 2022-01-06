package flags

import (
	"encoding/csv"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/saucelabs/saucectl/internal/msg"
)

// QuarantineMode represents the testcafe quarantineMode configuration.
type QuarantineMode struct {
	Values  map[string]interface{}
	Changed bool
}

// String returns a string represenation of the quarantineMode.
func (q QuarantineMode) String() string {
	if !q.Changed {
		return ""
	}
	return fmt.Sprintf("%+v", q.Values)
}

// Set sets the quarantineMode to the values present in s.
// The input has to be a comma separated string in the format of "key=value,key2=value2".
// This method is called by cobra when CLI flags are parsed.
func (q *QuarantineMode) Set(s string) error {
	q.Changed = true
	q.Values = make(map[string]interface{})

	rec, err := csv.NewReader(strings.NewReader(s)).Read()
	if err != nil {
		return err
	}

	for _, v := range rec {
		kvPair := strings.Split(v, "=")

		if len(kvPair) < 2 {
			msg.Error("--quarantineMode must be specified using a key-value format, e.g. \"--quarantineMode attemptLimit=3,successThreshold=2\"")
			return errors.New(msg.InvalidKeyValueInputFormat)
		}

		val := kvPair[1]
		switch kvPair[0] {
		case "attemptLimit":
			if err := q.setInt("attemptLimit", val); err != nil {
				return err
			}
		case "successThreshold":
			if err := q.setInt("successThreshold", val); err != nil {
				return err
			}
		}
	}

	return nil
}

func (q *QuarantineMode) setInt(key string, val string) error {
	vint, err := strconv.Atoi(val)
	if err != nil {
		return fmt.Errorf("%s must be an integer: %s", key, err)
	}

	q.Values[key] = vint

	return nil
}

// Type returns the value type.
func (q QuarantineMode) Type() string {
	return "quarantineMode"
}
