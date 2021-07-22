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

// redactStringToString redacts a stringToString flag.
func redactStringToString(flag *pflag.Flag) interface{} {
	params, err := stringToStringConv(flag.Value.String())
	if err != nil {
		return map[string]interface{}{}
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

// redactValue redacts potential sensitive values.
func redactValue(flag *pflag.Flag) interface{} {
	if flag.Value.Type() == "stringToString" {
		return redactStringToString(flag)
	}

	if flag.Value.String() == "" {
		return "***EMPTY***"
	}
	return "***REDACTED***"
}

// ExportCommandLineFlagsMap build the map of command line flags of the current execution.
func ExportCommandLineFlagsMap(set *pflag.FlagSet) map[string]interface{} {
	flags := map[string]interface{}{}
	set.Visit(func(flag *pflag.Flag) {
		if sliceContainsString(redactedFlags, flag.Name) {
			flags[flag.Name] = redactValue(flag)
		} else {
			flags[flag.Name] = flag.Value
		}
	})
	return flags
}
