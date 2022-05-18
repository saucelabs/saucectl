package slice

import (
	"fmt"
	"strings"
)

// Join joins a slice of objects into a comma separated string.
// Example:
//   ["value1", "value2"] -> "value1,value2"
func Join(values []any) string {
	var vv []string
	for _, v := range values {
		vv = append(vv, fmt.Sprintf("%v", v))
	}
	return strings.Join(vv, ",")
}
