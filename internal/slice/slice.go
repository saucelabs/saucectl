package slice

import (
	"fmt"
	"reflect"
	"strings"
)

// Join concatenates the elements of its first argument to create a single string. The separator string sep is placed
// between elements in the resulting string.
//
// Disclaimer: Use this function judiciously.
func Join(value any, sep string) string {
	if reflect.TypeOf(value).Kind() != reflect.Slice {
		return fmt.Sprintf("%v", value)
	}

	switch reflect.TypeOf(value).Elem().Kind() {
	case reflect.String:
		return strings.Join(value.([]string), sep)
	case reflect.Interface:
		elems := value.([]interface{})
		var vv []string
		for _, v := range elems {
			vv = append(vv, fmt.Sprintf("%v", v))
		}
		return strings.Join(vv, sep)
	case reflect.Int:
		elems := value.([]int)
		var vv []string
		for _, v := range elems {
			vv = append(vv, fmt.Sprintf("%d", v))
		}
		return strings.Join(vv, sep)
	default:
		return fmt.Sprintf("%v", value)
	}
}
