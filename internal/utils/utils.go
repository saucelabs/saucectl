package utils

import (
	"bufio"
	"strings"
)

// Split string into Splice on line-breaks
func SplitLines(s string) ([]string) {
	var l []string
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		l = append(l, sc.Text())
	}
	return l
}
