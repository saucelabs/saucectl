package flags

import (
	"encoding/csv"
	"fmt"
	"github.com/spf13/pflag"
	"strings"
)

// redactedFlags contains the list of flags that needs to be redacted before upload.
var redactedFlags = []string{"cypress.key", "env"}

// stringToStringConv converts the stringToString value to a map.
// This function has been copied from pflag library as stringToString is private.
func stringToStringConv(val string) (map[string]string, error) {
	val = strings.Trim(val, "[]")
	// An empty string would cause an empty map
	if len(val) == 0 {
		return map[string]string{}, nil
	}
	r := csv.NewReader(strings.NewReader(val))
	ss, err := r.Read()
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(ss))
	for _, pair := range ss {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("%s must be formatted as key=value", pair)
		}
		out[kv[0]] = kv[1]
	}
	return out, nil
}

func sliceContainsString(slice []string, val string) bool {
	for _, value := range slice {
		if value == val {
			return true
		}
	}
	return false
}

// redactStringToString redacts the values of a stringToString flag.
func redactStringToString(flag *pflag.Flag) map[string]string {
	params, err := stringToStringConv(flag.Value.String())
	if err != nil {
		return map[string]string{}
	}

	for key, val := range params {
		if val == "" {
			params[key] = "***EMPTY***"
		} else {
			params[key] = "***REDACTED***"
		}
	}
	return params
}

// redactStringValue redacts the value of a string flag.
func redactStringValue(flag *pflag.Flag) string {
	if flag.Value.String() == "" {
		return "***EMPTY***"
	}
	return "***REDACTED***"
}

// cleanString redacts value if the flag is marked as sensitive.
func cleanString(fl *pflag.Flag) string {
	if sliceContainsString(redactedFlags, fl.Name) {
		return redactStringValue(fl)
	}
	return fl.Value.String()
}

// cleanStringToString redacts entry values if the flag is marked as sensitive.
func cleanStringToString(fl *pflag.Flag) map[string]string {
	if sliceContainsString(redactedFlags, fl.Name) {
		return redactStringToString(fl)
	}
	values, err := stringToStringConv(fl.Value.String())
	if err != nil {
		return map[string]string{}
	}
	return values
}

// CaptureCommandLineFlags build the map of command line flags of the current execution.
func CaptureCommandLineFlags(set *pflag.FlagSet) map[string]interface{} {
	flags := map[string]interface{}{}
	set.Visit(func(flag *pflag.Flag) {
		if flag.Value.Type() == "stringToString" {
			flags[flag.Name] = cleanStringToString(flag)
		} else {
			flags[flag.Name] = cleanString(flag)
		}
	})
	return flags
}
